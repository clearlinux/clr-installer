#!/bin/bash
echo "Creating custom issue file for $1"

echo "Welcome to the Clear Linux* OS live image!

 * Documentation:     https://clearlinux.org/documentation
 * Community Support: https://clearlinux.org/community

To install Clear Linux* OS onto this sytem please login as root,
enter a new temporary password, and run:
clr-installer
" >> $1/etc/issue

exit 0
