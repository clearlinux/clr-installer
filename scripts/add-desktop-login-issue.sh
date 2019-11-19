#!/bin/bash
echo "Creating custom issue file for $1"

echo "Welcome to the Clear Linux* OS live image!

 * Documentation:     https://clearlinux.org/documentation
 * Community Support: https://community.clearlinux.org

To install Clear Linux* OS onto this system please login as 'clrlinux',
which has no password, and run:
  sudo -E clr-installer

" >> $1/etc/issue

exit 0
