#!/bin/bash

# c-basic-offset: 4; tab-width: 4; indent-tabs-mode: t
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:

# Server Post Install steps

set -ex

CHROOTPATH=$1
export HOOKDIR=$(dirname $0)

# Force Telemetry to use local host server
${HOOKDIR}/local-telemetry-post.sh ${CHROOTPATH}

# Delay booting to give user a change to change boot params
${HOOKDIR}/wait-to-boot-post.sh ${CHROOTPATH}

# Add issue (pre-login message) to inform user of how to run the installer
${HOOKDIR}/add-server-login-issue.sh ${CHROOTPATH}

# Add changes to PS1 to indicate live image by setting the hostname
echo "clr-live" > ${CHROOTPATH}/etc/hostname

chroot $CHROOTPATH systemd-machine-id-setup

exit 0

# Editor modelines  -  https://www.wireshark.org/tools/modelines.html
#
# Local variables:
# c-basic-offset: 4
# tab-width: 4
# indent-tabs-mode: t
# End:
#
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:
#
