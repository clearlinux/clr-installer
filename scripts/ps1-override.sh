#!/bin/bash
echo "Setting custom PS1 terminal prompt $1"

echo "PS1='\[\e[38;5;39m\]\u\[\e[0m\]@\[\e[38;5;208m\]clr-live \[\e[38;5;39m\]\w \[\e[38;5;39m\]# \[\e[0;0m\]'" >> $1/etc/profile

exit 0
