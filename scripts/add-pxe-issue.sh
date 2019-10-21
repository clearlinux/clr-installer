#!/bin/bash
echo "Creating custom issue file for $1"

echo "
PXE installation of Clear Linux in progress and will reboot
automatically when completed. Please do not interrupt.
" >> $1/etc/issue

exit 0
