#!/usr/bin/env bats

# Author: Karthik Prabhu Vinod
# Email: karthik.prabhu.vinod@intel.com

load "../testlib"

test_setup() {

    create_testworking_dir
    qemu-img create -f raw "$TESTWORKINGDIR"/testimgfile.img 5G
    loopbackdevice=$(losetup --partscan --find --show "$TESTWORKINGDIR"/testimgfile.img)

}

test_teardown() {

    losetup -d "$loopbackdevice" || true
    clean_testworking_dir
    
}

@test "INSTALL001: Basic Install" {
    
    run sh -c "$CLR_INSTALLER_EXE -c $TESTSCRIPTS/basic.yaml -b installer:${loopbackdevice}"
    assert_status_is "0"

}
