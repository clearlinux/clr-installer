#!/bin/bash

DESTDIR=$1
SAVE_DIR=$(pwd)
TEMP_INST=$(mktemp -d)
git clone . ${TEMP_INST}
cd ${TEMP_INST}
make install DESTDIR=${DESTDIR}

scripts/installer-image-post.sh ${DESTDIR}

cd ${SAVE_DIR}
/bin/rm -rf ${TEMP_INST}

exit 0
