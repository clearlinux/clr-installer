#!/bin/bash

# c-basic-offset: 4; tab-width: 4; indent-tabs-mode: t
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:

# Developer Image Post Install steps

set -ex

CHROOTPATH=$1
export HOOKDIR=$(dirname $0)

DESTDIR=$1
SAVE_DIR=$(pwd)
TEMP_INST=$(mktemp -d)
export HOME=$(getent passwd $(id -un) |& awk -F: '{print $(NF-1)}')
git clone https://github.com/clearlinux/swupd-client.git ${TEMP_INST}
cd ${TEMP_INST}

# git checkout master
#pr="1142"
#git fetch origin refs/pull/${pr}/head && git checkout -B pr-${pr} FETCH_HEAD
git checkout master

./autogen.sh
./configure --with-contenturl=https://cdn.download.clearlinux.org/update/ \
	--with-versionurl=https://cdn.download.clearlinux.org/update/ --with-formatid=29

make
make install DESTDIR=${DESTDIR}

cd ${SAVE_DIR}
/bin/rm -rf ${TEMP_INST}

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
