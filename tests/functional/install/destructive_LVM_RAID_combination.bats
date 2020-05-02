#!/usr/bin/env bats

# Author: Karthik Prabhu Vinod
# Email: karthik.prabhu.vinod@intel.com

load "../testlib"

test_setup() {

    create_testworking_dir
    qemu-img create -f raw "$TESTWORKINGDIR"/loop_disk1 1G
    qemu-img create -f raw "$TESTWORKINGDIR"/loop_disk2 6G
    qemu-img create -f raw "$TESTWORKINGDIR"/loop_disk3 9G

    # loopback disk1
    loop_device_disk1=$(losetup --partscan --find --show "$TESTWORKINGDIR"/loop_disk1)
    # loopback disk2
    loop_device_disk2=$(losetup --partscan --find --show "$TESTWORKINGDIR"/loop_disk2)
    # loopback disk3 
    loop_device_disk3=$(losetup --partscan --find --show "$TESTWORKINGDIR"/loop_disk3)

    # create part from disk1
    parted "${loop_device_disk1}" mklabel gpt
    fdisk -u -w always -W always "$loop_device_disk1" <<EOF
n



w
EOF

    # create part from disk2
    parted "${loop_device_disk2}" mklabel gpt
    fdisk -u -w always -W always "$loop_device_disk2" <<EOF
n



w
EOF

    # create part from disk3
    parted "${loop_device_disk3}" mklabel gpt
    fdisk -u -w always -W always "$loop_device_disk3" <<EOF
n


+1G
n


+6G
w
EOF

    # Create raid md3 based on disk1 disk3
    echo y | mdadm --create /dev/md3 --level=1 --raid-devices=2 "$loop_device_disk1" "${loop_device_disk3}"p1 --force
    # Create raid md4 based on disk2 disk3
    echo y | mdadm --create /dev/md4 --level=1 --raid-devices=2 "$loop_device_disk2" "${loop_device_disk3}"p2 --force
    
    # Required for raid devices to be ready
    wait_for_raid_to_be_active "/dev/md3"
    wait_for_raid_to_be_active "/dev/md4"

    # create three LVMS on md3 which get replicated on disk1 and disk3
    parted "/dev/md3" mklabel gpt
    fdisk -u -w always -W always /dev/md3 <<EOF
n



w
EOF

    pvcreate -ff /dev/md3p1 3>&-
    vgcreate -y TESTVG /dev/md3p1 3>&-
    lvcreate --wipesignatures y -n testraid3lv1 TESTVG -l 30%VG 3>&-
    lvcreate --wipesignatures y -n testraid3lv2 TESTVG -l 30%VG 3>&-
    lvcreate --wipesignatures y -n testraid3lv3 TESTVG -l 30%VG 3>&-

    # create three LVMS on md4 which get replicated on disk2 and disk3
    parted "/dev/md4" mklabel gpt
    fdisk -u -w always -W always /dev/md4 <<EOF
n



w
EOF

    pvcreate -ff /dev/md4p1 3>&-
    vgextend -y TESTVG /dev/md4p1 3>&-
    lvcreate --wipesignatures y -n testraid4lv1 TESTVG -l 30%VG 3>&-
    lvcreate --wipesignatures y -n testraid4lv2 TESTVG -l 30%VG 3>&-
    lvcreate --wipesignatures y -n testraid4lv3 TESTVG -l 30%VG 3>&-

    # Required for raid devices to be ready
    wait_for_raid_to_be_active /dev/md3
    wait_for_raid_to_be_active /dev/md4

}

test_teardown() {

    #Cleanup
    lvremove -y /dev/mapper/TESTVG-testlv1 /dev/mapper/TESTVG-testlv2 /dev/mapper/TESTVG-testlv3 3>&- || true
    vgremove -y TESTVG 3>&- || true
    pvremove -y /dev/md4p1 3>&- || true

    mdadm --stop /dev/md3 || true
    mdadm --stop /dev/md4 || true

    losetup -d "$loop_device_disk1" 3>&- || true
    losetup -d "$loop_device_disk2" 3>&- || true
    losetup -d "$loop_device_disk3" 3>&- || true

    rm -r "$TESTWORKINGDIR"/{loop_disk1,loop_disk2,loop_disk3} || true

    clean_testworking_dir

}


#------------------------------------
# LVM and RAID combination
#------------------------------------
#NAME                        MAJ:MIN RM   SIZE RO TYPE  MOUNTPOINT
#loop0                         7:0    0     1G  0 loop  
#└─md3                         9:3    0  1022M  0 raid1 
#  └─md3p1                   259:7    0  1021M  0 part  
#    ├─TESTVG-testraid3lv1   252:0    0   304M  0 lvm   
#    ├─TESTVG-testraid3lv2   252:1    0   304M  0 lvm   
#    └─TESTVG-testraid3lv3   252:2    0   304M  0 lvm   
#loop1                         7:1    0    10G  0 loop  
#└─md4                         9:4    0    10G  0 raid1 
#  └─md4p1                   259:8    0    10G  0 part  
#    ├─TESTVG-testraid4lv1   252:3    0   3.3G  0 lvm   
#    ├─TESTVG-testraid4lv2   252:4    0   3.3G  0 lvm   
#    └─TESTVG-testraid4lv3   252:5    0   3.3G  0 lvm   
#loop2                         7:2    0    30G  0 loop  
#├─loop2p1                   259:5    0     1G  0 part  
#│ └─md3                       9:3    0  1022M  0 raid1 
#│   └─md3p1                 259:7    0  1021M  0 part  
#│     ├─TESTVG-testraid3lv1 252:0    0   304M  0 lvm   
#│     ├─TESTVG-testraid3lv2 252:1    0   304M  0 lvm   
#│     └─TESTVG-testraid3lv3 252:2    0   304M  0 lvm   
#└─loop2p2                   259:6    0    10G  0 part  
#  └─md4                       9:4    0    10G  0 raid1 
#    └─md4p1                 259:8    0    10G  0 part  
#      ├─TESTVG-testraid4lv1 252:3    0   3.3G  0 lvm   
#      ├─TESTVG-testraid4lv2 252:4    0   3.3G  0 lvm   
#      └─TESTVG-testraid4lv3 252:5    0   3.3G  0 lvm
@test "INSTALL004: Destructive Install LVM and RAID combination" {

    run sh -c "$CLR_INSTALLER_EXE -c $TESTSCRIPTS/basic.yaml -b installer:${loop_device_disk3} --force-destructive"
    assert_status_is "0"

}
