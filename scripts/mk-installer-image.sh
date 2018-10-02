#!/bin/bash

IMG=${IMG:-"clr-installer.img"}
CONF="clr-installer.yaml"

export EXTRA_BUNDLES=${EXTRA_BUNDLES:-""}

if [ -z "$CLR_INSTALLER_ROOT_DIR" ]; then
    SRCDIR=$(dirname $0)
    if [ -z "$SRCDIR" ]; then
        SRCDIR="."
    fi
    CLR_INSTALLER_ROOT_DIR=$(cd ${SRCDIR}/.. ; echo $PWD)
fi
echo "Using CLR_INSTALLER_ROOT_DIR=${CLR_INSTALLER_ROOT_DIR}"

INST_MAIN_DIR=$CLR_INSTALLER_ROOT_DIR/clr-installer
INST_TOML_FILE=$CLR_INSTALLER_ROOT_DIR/Gopkg.toml
INST_MAKEFILE=$CLR_INSTALLER_ROOT_DIR/Makefile

if [ ! -d $INST_MAIN_DIR ] || [ ! -f $INST_TOML_FILE ] || [ ! -f $INST_MAKEFILE ]; then
    echo "CLR_INSTALLER_ROOT_DIR doesn't point to the Clear Linux" \
        "OS Installer source dir"
    exit 1
fi

WORK_DIR=$PWD
TEMP=$(mktemp -d)

if [ -z "$1" ]; then
    IMG_SIZE=${IMG_SIZE:-"4G"}

    echo "Creating empty image file '{$IMG}' of size ${IMG_SIZE}..."
    rm -f {$WORK_DIR}/${IMG}
    /usr/bin/qemu-img create -f raw ${WORK_DIR}/${IMG} ${IMG_SIZE}

    echo "Enabling ${IMG} file for loopback..."
    INSTDEV=$(sudo losetup --find --show ${WORK_DIR}/${IMG})
    MEDIA=$(basename ${INSTDEV})
    PART="${MEDIA}p"
    ROOTDEV="${INSTDEV}p3"
    DEVMAJMIN=$(losetup -O "MAJ:MIN" -n ${INSTDEV} | awk '{$1=$1};1')

    DEVSIZE=$(echo ${IMG_SIZE} | sed -e 's/b//gi' -e 's/k/*1024/gi' -e 's/m/*1024*1024/gi' -e 's/g/*1024*1024*1024/gi' -e 's/t/*1024*1024*1024*1024/gi')
    DEVSIZE=$((${DEVSIZE}))
    echo "Using Loopback device ${INSTDEV} [${DEVMAJMIN}] ..."
    TARGET="loopback device ${INSTDEV}"

    sudo partprobe ${INSTDEV}
    sleep 2
    CLEANUPCMD="sudo losetup -d ${INSTDEV}"
else
    if [ ! -z "${IMG_SIZE}" ]; then
        echo "Warning: Ignoring IMG_SIZE=${IMG_SIZE} when using block device"
    fi
    INSTDEV="$1"
    shift

    if [ ! -b ${INSTDEV} ]; then
        echo "${INSTDEV}: is not a block device!"
        exit 2
    fi
    $(mount -l  | grep ${INSTDEV} 1>/dev/null 2>/dev/null)
    if [ $? -eq 0 ]; then
        echo "ERROR: The device ${INSTDEV} is already mounted; abort..."
        exit 2
    fi

    MEDIA=$(basename ${INSTDEV})
    PART="${MEDIA}"
    ROOTDEV="${INSTDEV}3"
    DEVMAJMIN=$(file ${INSTDEV} | awk '{print $NF}' | sed -e 's/[()]//g' -e 's/\//:/')

    DEVSIZE=$(sudo blockdev --getsize64 ${INSTDEV})

    echo "Using block device ${INSTDEV} [${DEVMAJMIN}] ..."
    TARGET="block device ${INSTDEV}"
    CLEANUPCMD="/usr/bin/true"

    # Sanity check
    USB=$(lsblk -dno tran ${INSTDEV})
    if [ "${USB}" != "usb" ]; then
        read -p "Warning: Device ${INSTDEV} is not type USB; are you sure? [n]" answer
        shopt -s nocasematch
        case "${answer}" in
            y | yes )
                echo "Proceeding with install in 5 seconds"
                sleep 5
                ;;
            *)
                echo "Aborting install..."
                exit 3
                ;;
        esac
    fi

fi

# Standard Partitions
# /boot   150MB     [157286400]
# swap      2GB     [2147483648]
# /         remainder
ROOTSIZE=$((${DEVSIZE}-157286400-2147483648))
echo "Device size: ${DEVSIZE}"
echo "Root   size: ${ROOTSIZE}"

echo "Creating installation configuration file ..."
sed -e "s/1990197248/${ROOTSIZE}/g" -e "s/loop0p/${PART}/g" -e "s/loop0/${MEDIA}/g" -e "s/7:0/${DEVMAJMIN}/g" <<EOF >${WORK_DIR}/${CONF}
#clear-linux-config
targetMedia:
- name: loop0
  majMin: "7:0"
  size: "4294967296"
  ro: "false"
  rm: "false"
  type: loop
  children:
  - name: loop0p1
    fstype: vfat
    mountpoint: /boot
    size: "157286400"
    ro: "false"
    rm: "false"
    type: part
  - name: loop0p2
    fstype: swap
    size: "2147483648"
    ro: "false"
    rm: "false"
    type: part
  - name: loop0p3
    fstype: ext4
    mountpoint: /
    size: "1990197248"
    ro: "false"
    rm: "false"
    type: part
networkInterfaces: []
keyboard: us
language: en_US.UTF-8
bundles: [os-core, os-core-update, os-installer, telemetrics, ${EXTRA_BUNDLES}]
telemetry: false
timezone: America/Los_Angeles
kernel: kernel-native
postReboot: false
postArchive: false
autoUpdate: false
EOF

echo ""
echo "Installing Clear Linux OS to ${TARGET}..."
echo ""
pushd $CLR_INSTALLER_ROOT_DIR
sudo make
sudo -E $CLR_INSTALLER_ROOT_DIR/.gopath/bin/clr-installer --config ${WORK_DIR}/${CONF} --reboot=false
if [ $? -ne 0 ]; then
    echo "********************"
    echo "Install failed; Stopped image build process..."
    echo "********************"
    exit $?
fi

echo "Installing clr-installer into $TEMP"
sudo mount ${ROOTDEV} $TEMP

sudo make install DESTDIR=$TEMP
echo "Enabling clr-installer on boot for $TEMP"
sudo systemctl --root=$TEMP enable clr-installer

# Create a custom telemetry configuration to only log locally
echo "Creating custom telemetry configuration for $TEMP"
sudo /usr/bin/mkdir -p ${TEMP}/etc/telemetrics/
sudo /usr/bin/cp \
    ${TEMP}/usr/share/defaults/telemetrics/telemetrics.conf \
    ${TEMP}/etc/telemetrics/telemetrics.conf
sudo sed -i -e '/server=/s/clr.telemetry.intel.com/localhost/' \
    -e '/spool_process_time/s/=900/=3600/' \
    -e '/record_retention_enabled/s/=false/=true/' \
    ${TEMP}/etc/telemetrics/telemetrics.conf
popd

sudo umount ${TEMP}
$(${CLEANUPCMD})
sudo /bin/rm -rf ${TEMP}

exit 0
