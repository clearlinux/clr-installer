#!/bin/bash

CHROOTPATH=$1

# Force Telemetry to use local host server
scripts/local-telemetry-post.sh ${CHROOTPATH}

# Delay booting to give user a change to change boot params
scripts/wait-to-boot-post.sh ${CHROOTPATH}

# Add issue (pre-login message) to inform user of how to run the installer
scripts/add-login-issue.sh ${CHROOTPATH}

# Add changes to PS1 to indicate live image by setting the hostname
echo "clr-live" > ${CHROOTPATH}/etc/hostname

# Allow proxy environment variables to be used by root for the install
mkdir -p ${CHROOTPATH}/etc/sudoers.d/
echo 'Defaults env_keep += "http_proxy https_proxy ftp_proxy no_proxy socks_proxy"'  >> ${CHROOTPATH}/etc/sudoers.d/env_keep
echo 'Defaults env_keep += "HTTP_PROXY HTTPS_PROXY FTP_PROXY NO_PROXY SOCKS_PROXY"'  >> ${CHROOTPATH}/etc/sudoers.d/env_keep

exit 0
