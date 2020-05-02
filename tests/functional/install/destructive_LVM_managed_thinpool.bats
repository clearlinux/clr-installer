#!/usr/bin/env bats

# Author: Karthik Prabhu Vinod
# Email: karthik.prabhu.vinod@intel.com

load "../testlib"

test_setup() {

   # skip test if dm-thin-pool module not present
   if [[ ! $(modinfo dm-thin-pool) ]]; then
      skip "This test does not run well in travis. We can test this locally"
   fi

    create_testworking_dir
    #test setup
    # Disk 1
    qemu-img create -f raw "$TESTWORKINGDIR"/loopdevice.img 15G
    loop_device_disk=$(losetup --partscan --find --show "$TESTWORKINGDIR"/loopdevice.img)

    # Creating a part of LVM2_member type on Disk1
    parted "${loop_device_disk}" mklabel gpt

    fdisk -u -w always -W always ${loop_device_disk} <<EOF
n



w
EOF
    pvcreate -ff "$loop_device_disk"p1 3>&-
    vgcreate -y docker "$loop_device_disk"p1 3>&-
    lvcreate --wipesignatures y -n thinpool docker -l 95%VG 3>&-
    lvcreate --wipesignatures y -n thinpoolmeta docker -l 1%VG 3>&-
    lvconvert -y --zero n -c 512K --thinpool docker/thinpool --poolmetadata docker/thinpoolmeta 3>&-

}

test_teardown() {

    #test cleanup
    lvremove -y /dev/mapper/docker-thinpool 3>&- || true
    vgremove -y docker 3>&- || true
    pvremove -y "$loop_device_disk"p1 || true
    losetup -d "$loop_device_disk" || true
    rm -r "$TESTWORKINGDIR"/loopdevice.img  || true
    clean_testworking_dir

}

#------------------------------------
# Managed LVM thinpool 
#------------------------------------
#NAME                      MAJ:MIN RM   SIZE RO TYPE MOUNTPOINT
#loop0                       7:0    0    30G  0 loop 
#└─loop0p1                 259:5    0    30G  0 part 
#  ├─docker-thinpool_tmeta 252:0    0   304M  0 lvm  
#  │ └─docker-thinpool     252:2    0  28.5G  0 lvm  
#  └─docker-thinpool_tdata 252:1    0  28.5G  0 lvm  
#    └─docker-thinpool     252:2    0  28.5G  0 lvm
# Issue 633 
@test "INSTALL003: Destructive Install Managed LVM thinpool" {

    # test
    # Succeed with force-destructive
    run sh -c "$CLR_INSTALLER_EXE -c $TESTSCRIPTS/basic.yaml -b installer:${loop_device_disk}"
    assert_status_is "0"

}
