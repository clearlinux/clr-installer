#!/usr/bin/env bats

# Author: Karthik Prabhu Vinod
# Email: karthik.prabhu.vinod@intel.com

load "../testlib"

global_setup() {
    create_testworking_dir
}

global_teardown() {
    clean_testworking_dir
}

test_teardown() {
    losetup -d "$loopbackdevice"
}

@test "INSTALL001: Basic Install" {
    
    qemu-img create -f raw "$TESTWORKINGDIRECTORY"/testimgfile.img 5G
    
    loopbackdevice=$(losetup --partscan --find --show "$TESTWORKINGDIRECTORY"/testimgfile.img)
    export loopbackdevice

    run sh -c "$CLR_INSTALLER_EXE -c $TESTSCRIPTS/basic.yaml -b installer:${loopbackdevice}"

    assert_status_is "0"

}
