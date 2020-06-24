#!/usr/bin/env bats

# Author: Mark Horn
# Email: mark.d.horn@intel.com

load "../testlib"
LOOP=""

test_setup() {

    create_testworking_dir
    qemu-img create -f raw "$TESTWORKINGDIR"/loop_disk 12G

    # loopback disk
    LOOP=$(losetup --partscan --find --show "$TESTWORKINGDIR"/loop_disk)

    # Partition disk
    parted ${LOOP} mklabel gpt
    parted ${LOOP} mkpart CLR_BOOT_F fat32 0% 300MB
    parted ${LOOP} mkpart pv 300MB 100%
    parted ${LOOP} print
    udevadm settle --timeout 10
    # Create the physical volume
    pvcreate -ff ${LOOP}p2
    # Create the volume group
    vgcreate VgFuncTestClrRoot ${LOOP}p2
    # Create the logical volumes with Advanced naming
    lvcreate VgFuncTestClrRoot -n CLR_ROOT_F -l 90%FREE
    lvcreate VgFuncTestClrRoot -n CLR_SWAP -l 20%FREE
    lvcreate VgFuncTestClrRoot -n CLR_MNT_+home -l 70%FREE
    lvcreate VgFuncTestClrRoot -n CLR_MNT_+data -l 100%FREE

    udevadm settle --timeout 10
}

test_teardown() {

    #Cleanup
    lvremove -y /dev/mapper/VgFuncTestClrRoot-CLR_ROOT_F /dev/mapper/VgFuncTestClrRoot-CLR_SWAP \
        /dev/mapper/VgFuncTestClrRoot-CLR_MNT_+home /dev/mapper/VgFuncTestClrRoot-CLR_MNT_+data 3>&- || true
    vgremove -y VgFuncTestClrRoot 3>&- || true
    pvremove -y ${LOOP}p2 3>&- || true

    losetup -d "${LOOP}" 3>&- || true

    rm -r "$TESTWORKINGDIR"/loop_disk || true

    clean_testworking_dir

}


#------------------------------------
# Advanced LVM
#------------------------------------
#NAME                                MAJ:MIN RM  SIZE RO TYPE MOUNTPOINT
#loop0                               253:16   0   12G  0 disk
#├─loop0                             253:17   0  285M  0 part
#└─loop0                             253:18   0 11.7G  0 part
#  ├─VgFuncTestClrRoot-CLR_ROOT_F    252:0    0 10.6G  0 lvm
#  ├─VgFuncTestClrRoot-CLR_SWAP      252:1    0  240M  0 lvm
#  ├─VgFuncTestClrRoot-CLR_MNT_+home 252:2    0  672M  0 lvm
#  └─VgFuncTestClrRoot-CLR_MNT_+data 252:3    0  288M  0 lvm
#
@test "INSTALL005: Advanced Install using Logical Volumes" {

    run sh -c "$CLR_INSTALLER_EXE -c $TESTSCRIPTS/advanced.yaml --log-file $TESTWORKINGDIR/advanced.log"
    assert_status_is "0"

}
