#!/bin/bash

CHROOTPATH=$1

# Enable the installer on boot
scripts/enable-installer-post.sh ${CHROOTPATH}

# Force Telemetry to use local host server
scripts/local-telemetry-post.sh ${CHROOTPATH}

# Delay booting to give user a change to change boot params
scripts/wait-to-boot-post.sh ${CHROOTPATH}

exit 0
