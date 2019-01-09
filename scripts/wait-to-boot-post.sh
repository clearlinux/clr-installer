#!/bin/bash

# Have the installer image wait 5 seconds before launch
# Useful for users to change the boot command for debug
echo "timeout 5" >> $1/boot/loader/loader.conf

exit 0
