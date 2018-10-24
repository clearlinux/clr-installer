#!/bin/bash

# NOTE: Changes to /usr in this file is due to /etc and /var being tmpfs
# so the only way to permanently alter system default behavior is to
# modify /usr. This makes the system not exactly Clear Linux like.

# Cleanup optimized firstboot data in var
rm -fr "$1/var/"*

# Create directories used in later setup for embedded service
mkdir "$1/root/bundle"
mkdir "$1/root/integration"

# Stop systemd-journal from running
rm "$1/usr/lib/systemd/system/sysinit.target.wants/systemd-journal"*
rm "$1/usr/lib/systemd/system/sockets.target.wants/systemd-journald"*

# Create specialized mount units for /etc and /var to be tmpfs
cat <<EOF > "$1/usr/lib/systemd/system/etc.mount"
[Unit]
Description=Make /etc temporary
DefaultDependencies=no
Conflicts=umount.target
Before=local-fs.target umount.target
After=swap.target

[Mount]
What=tmpfs
Where=/etc
Type=tmpfs
Options=mode=0755
EOF

cat <<EOF > "$1/usr/lib/systemd/system/var.mount"
[Unit]
Description=Make /var temporary
DefaultDependencies=no
Conflicts=umount.target
Before=local-fs.target umount.target
After=swap.target

[Mount]
What=tmpfs
Where=/var
Type=tmpfs
Options=mode=0755
EOF

cat <<EOF > "$1/usr/lib/systemd/system/machine-id.service"
[Unit]
Description=Create /etc/machine-id after /etc is mounted as tmpfs
DefaultDependencies=no
Conflicts=shutdown.target
Before=sysinit.target shutdown.target
After=etc.mount

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/systemd-machine-id-setup
EOF

ln -s ../etc.mount "$1/usr/lib/systemd/system/local-fs.target.wants/"
ln -s ../var.mount "$1/usr/lib/systemd/system/local-fs.target.wants/"
ln -s ../machine-id.service "$1/usr/lib/systemd/system/local-fs.target.wants/"
