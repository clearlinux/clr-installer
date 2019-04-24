#!/bin/bash

CHROOTPATH=$1

# Force Telemetry to use local host server
scripts/local-telemetry-post.sh ${CHROOTPATH}

# Delay booting to give user a change to change boot params
scripts/wait-to-boot-post.sh ${CHROOTPATH}

# Add issue (pre-login message) to inform user of how to run the installer
scripts/add-login-issue.sh ${CHROOTPATH}

# Add changes to PS1 to indicate live image
scripts/ps1-override.sh ${CHROOTPATH}

exit 0
