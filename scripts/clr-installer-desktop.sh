#!/bin/bash

PALETTE="['rgb(0,0,0)', 'rgb(205,0,0)', 'rgb(0,205,0)', 'rgb(205,205,0)', \
 'rgb(0,0,205)', 'rgb(205,0,205)', 'rgb(0,205,205)', 'rgb(250,235,215)', \
'rgb(64,64,64)', 'rgb(255,0,0)', 'rgb(0,255,0)', 'rgb(255,255,0)', \
'rgb(0,0,255)', 'rgb(255,0,255)', 'rgb(0,255,255)', 'rgb(255,255,255)']"

GTERM_ROOT="/org/gnome/terminal/legacy/profiles:"
PROF_LIST="$GTERM_ROOT/list"

list=$(dconf read $PROF_LIST)
IFS=", " read -a ids <<< "${list:1:-1}"

nids=""
configured=0

if [ ${#ids[@]} -gt 0 ]; then
    for ((i=0; i<${#ids[@]}; ++i)); do
        pid=${ids[i]}

        vname=$(dconf read $GTERM_ROOT/:${pid:1:-1}/visible-name)
        if [ "$vname" == "'clr-installer'" ]; then
            # terminal already configured just use it
            configured=1
        fi

        nids="$nids,$pid"
    done
fi

if [ $configured -eq 0 ]; then
    # default profile
    if [ ${#ids[@]} -eq 0 ]; then
        UUID=$(uuidgen)
        GTERM_PROF="$GTERM_ROOT/:$UUID"
        nids="'${UUID}'${nids}"

        dconf write $GTERM_ROOT/list "[$nids]"
        dconf write $GTERM_ROOT/default "'$UUID'"
        dconf write $GTERM_PROF/use-theme-colors "true"
    fi

    # custom profile
    UUID=$(uuidgen)
    GTERM_PROF="$GTERM_ROOT/:$UUID"
    nids="'${UUID}',${nids}"
    echo "nids: $nids"

    dconf write $GTERM_ROOT/list "[$nids]"
    dconf write $GTERM_PROF/use-theme-colors "false"
    dconf write $GTERM_PROF/use-system-font "false"
    dconf write $GTERM_PROF/visible-name "'clr-installer'"
    dconf write $GTERM_PROF/foreground-color "'rgb(255,255,255)'"
    dconf write $GTERM_PROF/background-color "'rgb(0,0,0)'"
    dconf write $GTERM_PROF/background-color "'rgb(0,0,0)'"
    dconf write $GTERM_PROF/palette "$PALETTE"
    dconf write $GTERM_PROF/font "'Monospace 18'"
    dconf write /org/gnome/shell/favorite-apps "$FAVORITE_APPS"
fi

gnome-terminal --profile=clr-installer -x sh -c "sudo clr-installer"
