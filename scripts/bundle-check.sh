#!/bin/bash

# c-basic-offset: 4; tab-width: 4; indent-tabs-mode: t
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:

# Pre-Install Hook
# Check that all bundles in the optional bundle install
# file are valid bundles for this release of Clear Linux OS

# Good for debug
#set -ex

CHROOTPATH=$1
HOOKDIR=$(dirname $0)
ETCDIR=$(cd ${HOOKDIR}/../etc ; pwd)
BUNDLES_DIR="/usr/share/clr-bundles"


CHECK_BUNDLES=$(jq -r '.[] | .[].name' ${ETCDIR}/bundles.json)
ALL_OKAY=1

for b in ${CHECK_BUNDLES}
do
	if [ ! -f "${BUNDLES_DIR}/${b}" ]
	then
		echo "${b} is NOT a valid bundle!"
		ALL_OKAY=0
	fi
done

if [ ${ALL_OKAY} -ne 1 ]
then
	echo "Update the ${ETCDIR}/bundles.json file."
	exit 1
fi

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
