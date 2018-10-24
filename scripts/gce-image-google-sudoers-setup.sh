#!/bin/bash

# This script creates the sudoers elements that
# GCE expects in the image to add user to google-sudoers
# whe a user key is injected

usage() {
   echo "Provide path to existing chroot"
   exit 1
}


main() {
    local CHROOTPATH=$1
    sudo touch ${CHROOTPATH}/etc/sudoers
    sudo chmod 440 ${CHROOTPATH}/etc/sudoers
    sudo mkdir ${CHROOTPATH}/etc/sudoers.d
}

if [ $# -eq 0 ]; then
    usage
fi

if [ ! -d "$1" ]; then
    usage
fi

main $@
