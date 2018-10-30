#!/bin/bash

main() {

	local CHROOT=$1

	# Create workspace
	mkdir -p ${CHROOT}/tmp/workspace
	local WS=/tmp/workspace

	# Setting local variables for ingredients
	local ACRN_BIN=/usr/lib/acrn/acrn.sbl
	local SOS_BOOTARGS=/usr/share/acrn/samples/apl-mrb/sos_bootargs_debug.txt
	local IASTOOL=/usr/bin/iasimage

	# Checking file existence
	if [ ! -f ${CHROOT}${ACRN_BIN} ]
	then
		echo "ACRN binary is not found."
		exit 1
	fi

	if [ ! -f ${CHROOT}${SOS_BOOTARGS} ]
	then
		echo "SOS Bootargs is not found"
		exit 1
	fi

	KERNEL_PATH=$(find ${CHROOT}/usr/lib/kernel -maxdepth 1 -name 'org.clearlinux.pk414-sos*')
	if [ $(echo "$KERNEL_PATH" | wc -l) -ne 1 ]
	then
		echo "Only one kernel should exist"
		exit 1
	else
		KERNEL_PATH=${KERNEL_PATH#${CHROOT}}
	fi

	if [ ! -x ${CHROOT}${IASTOOL} ]
	then
		echo "iasimage tool is not found"
		exit 1
	fi

	# Download the debug key for signing
	curl -o ${CHROOT}${WS}/bxt_dbg_priv_key.pem -k ${DEBUG_KEY_PATH}

	if [[ $? -ne 0 ]]
	then
		echo "Failed to retrieve debug key"
		exit 1;
	fi

	# Create stitched image
	touch ${CHROOT}${WS}/hv_cmdline
	chroot ${CHROOT} /usr/bin/iasimage create -o ${WS}/iasImage -i 0x40300 -d ${WS}/bxt_dbg_priv_key.pem ${WS}/hv_cmdline ${ACRN_BIN} ${SOS_BOOTARGS} ${KERNEL_PATH}

	if [ ! -f ${CHROOT}${WS}/iasImage ]
	then
		echo "Failed to sign IAS image. Stopped."
		exit 1
	fi

	# Install to the SOS boot partition
	cp ${CHROOT}${WS}/iasImage ${CHROOT}/mnt

	# Clean up
	rm -rf ${CHROOT}${WS}

}

main $@
