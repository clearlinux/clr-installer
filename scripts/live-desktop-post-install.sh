#!/bin/bash

# c-basic-offset: 4; tab-width: 4; indent-tabs-mode: t
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:

# Desktop Post Install steps

set -ex

CHROOTPATH=$1
export HOOKDIR=$(dirname $0)

# Force Telemetry to use local host server
${HOOKDIR}/local-telemetry-post.sh ${CHROOTPATH}

# Delay booting to give user a change to change boot params
${HOOKDIR}/wait-to-boot-post.sh ${CHROOTPATH}

GDM_DIR=$CHROOTPATH/etc/gdm/
THEMES_DIR=$CHROOTPATH/usr/share/clr-installer/themes
DESKTOP_DIR=$CHROOTPATH/usr/share/applications/
VAR_DIR=$CHROOTPATH/var/lib/clr-installer

# Add the user account for auto-login
echo "Creating user account clrlinux"
chroot $CHROOTPATH usermod -a -G wheelnopw clrlinux
chroot $CHROOTPATH usermod -u 1001 clrlinux
chroot $CHROOTPATH passwd -d clrlinux

chroot $CHROOTPATH systemd-machine-id-setup

mkdir -p $GDM_DIR/
cat > $GDM_DIR/custom.conf <<CUSTOM_CONF
[daemon]
AutomaticLoginEnable=True
AutomaticLogin=clrlinux
CUSTOM_CONF
chmod 644 $GDM_DIR/custom.conf

mkdir -p $VAR_DIR/
cat > $VAR_DIR/clr-installer.yaml <<DESKTOP_YAML_DEFAULT
---
bundles: [
    c-basic,
    desktop-autostart,
    git,
    openssh-server,
    os-core,
    os-core-update,
    vim,
  ]

keyboard: us
language: en_US.UTF-8
timezone: UTC
kernel: kernel-native
DESKTOP_YAML_DEFAULT
chmod 644 $VAR_DIR/clr-installer.yaml

FAVORITE_APPS="['clr-installer-gui.desktop', 'org.gnome.Terminal.desktop', \
       'org.gnome.Nautilus.desktop', 'firefox.desktop', \
       'org.gnome.Evolution.desktop']"

chroot $CHROOTPATH su - clrlinux -c \
       "dbus-run-session \
        dconf write /org/gnome/shell/favorite-apps \"$FAVORITE_APPS\""

# Disable auto-mount of media as it will be excluded from install targets
chroot $CHROOTPATH su - clrlinux -c \
       "dbus-run-session \
        dconf write /org/gnome/desktop/media-handling/automount false"
chroot $CHROOTPATH su - clrlinux -c \
       "dbus-run-session \
        dconf write /org/gnome/desktop/media-handling/automount-open false"

exit 0

# Editor modelines  -  https://www.wireshark.org/tools/modelines.html
#
# Local variables:
# c-basic-offset: 4
# tab-width: 4
# indent-tabs-mode: t
# End:
#
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:
#
