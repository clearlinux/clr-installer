#!/bin/bash

# Root login must bedisabled aws images

usage() {
   echo "usage: $0 [chrootpath]"
   echo "Provide path to existing chroot"
   exit 1
}


main() {
    local CHROOTPATH=$1
    sudo mkdir -p ${CHROOTPATH}/etc/ssh/
    sudo echo "PermitRootLogin no" >> ${CHROOTPATH}/etc/ssh/sshd_config
    
}

if [ $# -eq 0 ]; then
    usage
fi

if [ ! -d "$1" ]; then
    usage
fi

main $@
