#!/bin/bash

echo "Enabling clr-installer on boot for $1"
systemctl --root=$1 enable clr-installer

# Create a custom telemetry configuration to only log locally
echo "Creating custom telemetry configuration for $1"
mkdir -p $1/etc/telemetrics/

cp $1/usr/share/defaults/telemetrics/telemetrics.conf \
   $1/etc/telemetrics/telemetrics.conf

sed -i -e '/server=/s/clr.telemetry.intel.com/localhost/' \
    -e '/spool_process_time/s/=900/=3600/' \
    -e '/record_retention_enabled/s/=false/=true/' \
    $1/etc/telemetrics/telemetrics.conf

# Ensure telemetry is not enabled
touch $1/etc/telemetrics/opt-out

# Have the installer image wait 5 seconds before launch
# Useful for users to change the boot command for debug
echo "timeout 5" >> $1/boot/loader/loader.conf

exit 0
