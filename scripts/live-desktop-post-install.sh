#!/bin/bash

set -ex

CHROOTPATH=$1

GDM_DIR=$CHROOTPATH/etc/gdm/
THEMES_DIR=$CHROOTPATH/usr/share/clr-installer/themes
DESKTOP_DIR=$CHROOTPATH/usr/share/applications/

usermod --root $CHROOTPATH -a -G wheelnopw clrlinux
usermod --root $CHROOTPATH -u 1001 clrlinux
passwd --root $CHROOTPATH -d clrlinux

systemctl --root=$CHROOTPATH disable clr-installer
systemd-machine-id-setup --root=$CHROOTPATH

mkdir -p $GDM_DIR

cp themes/clr.png $THEMES_DIR
cp etc/clr-installer.desktop $DESKTOP_DIR
cp etc/custom.conf $GDM_DIR
cp scripts/clr-installer-desktop.sh $CHROOTPATH/usr/bin/
cp etc/clr-desktop.yaml $CHROOTPATH/var/lib/clr-installer/clr-installer.yaml

FAVORITE_APPS="['clr-installer.desktop', 'org.gnome.Terminal.desktop', \
       'org.gnome.Nautilus.desktop', 'firefox.desktop', \
       'org.gnome.Evolution.desktop']"

chroot $CHROOTPATH su - clrlinux -c \
       "dbus-run-session \
        dconf write /org/gnome/shell/favorite-apps \"$FAVORITE_APPS\""
