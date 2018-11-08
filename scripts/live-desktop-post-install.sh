#!/bin/bash

set -ex

CHROOTPATH=$1

GDM_DIR=$CHROOTPATH/etc/gdm/
THEMES_DIR=$CHROOTPATH/usr/share/clr-installer/themes
DESKTOP_DIR=$CHROOTPATH/usr/share/applications/

usermod --root $CHROOTPATH -a -G wheelnopw clrlinux
systemctl --root=$CHROOTPATH disable clr-installer
systemd-machine-id-setup --root=$CHROOTPATH

mkdir -p $GDM_DIR

cp themes/clr.png $THEMES_DIR
cp etc/clr-installer.desktop $DESKTOP_DIR
cp etc/custom.conf $GDM_DIR
cp scripts/clr-installer-desktop.sh $CHROOTPATH/usr/bin/

FAVORITE_APPS="['clr-installer.desktop', 'org.gnome.Terminal.desktop', \
       'org.gnome.Nautilus.desktop', 'firefox.desktop', \
       'org.gnome.Evolution.desktop']"

chroot $CHROOTPATH su - clrlinux -c \
       "dbus-launch \
        dconf write /org/gnome/shell/favorite-apps \"$FAVORITE_APPS\""

chroot $CHROOTPATH su - clrlinux -c \
       "dbus-launch \
        dconf write /org/gnome/desktop/session/idle-delay 0"
