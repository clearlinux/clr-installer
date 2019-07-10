#!/bin/bash

# Create a custom telemetry configuration to only log locally
echo "Creating custom telemetry configuration for $1"
mkdir -p $1/etc/telemetrics/

# Create configuration to keep data private
if [[ ! -f "$1/etc/telemetrics/telemetrics.conf" ]];then
        cat <<EOF > $1/etc/telemetrics/telemetrics.conf
server=http://localhost/v2/collector
record_server_delivery_enabled=false
record_retention_enabled=true
EOF
fi

exit 0
