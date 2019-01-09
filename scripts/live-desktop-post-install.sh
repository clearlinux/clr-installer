#!/bin/bash

set -ex

CHROOTPATH=$1

# Force Telemetry to use local host server
scripts/local-telemetry-post.sh ${CHROOTPATH}

# Delay booting to give user a change to change boot params
scripts/wait-to-boot-post.sh ${CHROOTPATH}

GDM_DIR=$CHROOTPATH/etc/gdm/
THEMES_DIR=$CHROOTPATH/usr/share/clr-installer/themes
DESKTOP_DIR=$CHROOTPATH/usr/share/applications/
VAR_DIR=$CHROOTPATH/var/lib/clr-installer

usermod --root $CHROOTPATH -a -G wheelnopw clrlinux
usermod --root $CHROOTPATH -u 1001 clrlinux
passwd --root $CHROOTPATH -d clrlinux

systemctl --root=$CHROOTPATH disable clr-installer
systemd-machine-id-setup --root=$CHROOTPATH

install -D -m 644 themes/clr.png $THEMES_DIR/clr.png
install -D -m 644 etc/custom.conf $GDM_DIR/custom.conf
install -D -m 644 etc/clr-desktop.yaml $VAR_DIR/clr-installer.yaml

FAVORITE_APPS="['clr-installer.desktop', 'org.gnome.Terminal.desktop', \
       'org.gnome.Nautilus.desktop', 'firefox.desktop', \
       'org.gnome.Evolution.desktop']"

chroot $CHROOTPATH su - clrlinux -c \
       "dbus-run-session \
        dconf write /org/gnome/shell/favorite-apps \"$FAVORITE_APPS\""
