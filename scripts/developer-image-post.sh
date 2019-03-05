#!/bin/bash

DESTDIR=$1
SAVE_DIR=$(pwd)
TEMP_INST=$(mktemp -d)
export HOME=$(getent passwd $(id -un) |& awk -F: '{print $(NF-1)}')
git clone . ${TEMP_INST}
cd ${TEMP_INST}
make install DESTDIR=${DESTDIR}

scripts/installer-post.sh ${DESTDIR}

cd ${SAVE_DIR}
/bin/rm -rf ${TEMP_INST}

exit 0
