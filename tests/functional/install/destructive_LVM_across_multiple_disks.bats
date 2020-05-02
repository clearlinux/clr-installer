#!/usr/bin/env bats

# Author: Karthik Prabhu Vinod
# Email: karthik.prabhu.vinod@intel.com

load "../testlib"

test_setup() {
    if [ -n "$TRAVIS" ]; then
        skip "This test does not run well in travis. We can test this locally"
    fi

    create_testworking_dir
    #test setup

    # Disk 1
    qemu-img create -f raw "$TESTWORKINGDIR"/loopdevice.img 5G
    loop_device_disk1=$(losetup --partscan --find --show "$TESTWORKINGDIR"/loopdevice.img)

    #Disk 2
    qemu-img create -f raw "$TESTWORKINGDIR"/loopdevice2.img 5G
    loop_device_disk2=$(losetup --partscan --find --show "$TESTWORKINGDIR"/loopdevice2.img)

    # Creating a part of LVM2_member type on Disk1
    fdisk -u -w always -W always ${loop_device_disk1} <<EOF
n
p



t
8e
w
EOF

    # Creating a part of LVM2_member type on Disk2
    fdisk -u -w always -W always ${loop_device_disk2} <<EOF
n
p



t
8e
w
EOF

    pvcreate -ff "$loop_device_disk1"p1 3>&-
    pvcreate -ff "$loop_device_disk2"p1 3>&-
    vgcreate -y dockershared "$loop_device_disk1"p1 "$loop_device_disk2"p1 3>&-

    lvcreate --wipesignatures y -n lv1 dockershared -l 30%VG 3>&-
    lvcreate --wipesignatures y -n lv2 dockershared -l 30%VG 3>&-
    lvcreate --wipesignatures y -n lv3 dockershared -l 40%VG 3>&-

}

test_teardown() {

    #test cleanup
    lvremove -y /dev/mapper/dockershared-lv1 3>&- || true
    lvremove -y /dev/mapper/dockershared-lv2 3>&- || true
    lvremove -y /dev/mapper/dockershared-lv3 3>&- || true
    vgremove -y dockershared 3>&- || true
    pvremove -y "$loop_device_disk1"p1 || true
    pvremove -y "$loop_device_disk2"p1 || true

    losetup -d $loop_device_disk1 || true
    losetup -d $loop_device_disk2 || true

    rm -r "$TESTWORKINGDIR"/loopdevice.img || true
    rm -r "$TESTWORKINGDIR"/loopdevice2.img || true

    clean_testworking_dir

}

#------------------------------------
# LVM across multiple disks
#------------------------------------
#NAME                 MAJ:MIN RM   SIZE RO TYPE MOUNTPOINT
#loop0                  7:0    0    15G  0 loop 
#└─loop0p1            259:5    0    15G  0 part 
#  ├─dockershared-lv1 252:0    0     9G  0 lvm  
#  └─dockershared-lv3 252:2    0    12G  0 lvm  
#loop1                  7:1    0    15G  0 loop 
#└─loop1p1            259:6    0    15G  0 part 
#  ├─dockershared-lv2 252:1    0     9G  0 lvm  
#  └─dockershared-lv3 252:2    0    12G  0 lvm  
@test "INSTALL002: Destructive Install LVM across two disks" {

    # test
    # Fail without force-destructive
    run sh -c "$CLR_INSTALLER_EXE -c $TESTSCRIPTS/basic.yaml -b installer:${loop_device_disk2}"
    assert_status_is_not "0"

    # Succeed with force-destructive
    run sh -c "$CLR_INSTALLER_EXE -c $TESTSCRIPTS/basic.yaml -b installer:${loop_device_disk2} --force-destructive"
    assert_status_is "0"

}
