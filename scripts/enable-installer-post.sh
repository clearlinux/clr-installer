#!/bin/bash

echo "Enabling clr-installer on boot for $1"
systemctl --root=$1 enable clr-installer

exit 0
