# Installer YAML Syntax

This document describes the syntax for constructing a clr-installer configuration file.

## Production Configuration YAML Files
These can be found on the publisher site.
https://download.clearlinux.org/current/config/image/

## Environment Variables
Environment variables can be defined which will be used when installation commands are executed. These are most commonly used for `pre-install`, `post-install`, or `post-image` hooks.
```yaml
env:
  <variable>: <value>
```

## Device Aliases
To avoid changing a device name in multiple locations in the `targetMedia`, device aliases can be used to simply change between image files and physical devices.
```yaml
# switch between aliases in order to install to an actual block device
# i.e /dev/sda
block-devices: [
   {name: "bdevice", file: "os-image.img"}
]
```
or 
```yaml
block-devices: [
   {name: "bdevice", file: "/dev/sda"}
]
```

## Target Media
The `targetMedia` is the media where the Clear Linux OS will be installed. This can be either an image filename, or a physical device name. When using image filenames, first define a device alias for the image file.

Item | Description | Required?
------------ | ------------- | ------------- 
`name:` | Block-device alias or the physical device name| Yes
`type:` | Type of the target media should always be `disk` | Yes
`children:` | List of partition for the image | Yes
`size:` | Size of the media to be used, or the image file size to be generated. This will be calculated as the sum of the partition sizes if not present. | No

### Children
Item | Description | Required?
------------ | ------------- | ------------- 
`name:` | Block-device alias and partition number or the physical partition name| Yes
`type:` | Partition type should be `part` for a standard partition or `crypt` for encrypted partitions | Yes
`fstype:` | Type of the partition can be one of: `swap`, or `ext2`, `ext3`, `ext4`, `xfs`, `f2fs`, `btrfs`, or `vfat` | Yes
`size:` | Size of the partition. Set to `0` to use the remaining free space for this partition; there can only be one partition of size `0`. The suffixes `B` for bytes, `K` or `KB` for kilobytes, `M` or `MB` for megabytes, `G` or `GB` for gigabytes, `T` or `TB` for terabytes, `P` or `PB` for petabytes, `KiB` for kibibyte, `MiB` for mebibyte, `GiB` for gibibyte, `TiB` for tebibyte, `PiB` for pebibyte can be used.  | Yes
`mountpoint:` | The file system path where the partition should be mounted. | No
`options:` | Additional file system options to be used when creating the fs | No
`label:` | Short string labeling the partition | No

```yaml
block-devices: [
   {name: "installer", file: "installer.img"}
]

targetMedia:
- name: ${installer}
  type: disk
  children:
  - name: ${installer}1
    fstype: vfat
    mountpoint: /boot
    size: "150MiB"
    type: part
  - name: ${installer}2
    fstype: ext4
    mountpoint: /
    size: "2.6GiB"
    type: part
```

### Swap
The default, as of release `2.5.0`, is to create a swapfile `/var/swapfile` during an interactive installation or if no swap partition is defined when Advanced Installation Media Targets are defined. The default swapfile size can be overridden by setting it in the YAML configuration file, which in turn can be overridden by using the `--swap-file-size=<size>` on the command line.

```yaml
swapFileSize: "64MiB"
```

If a swap partition is defined and the swapFileSize or `--swap-file-size=<size>` are set, both types of swap will be configured in the target system.

### Advanced Installation Media Targets

To use Advance Partition Labels for a command line installation, `targetMedia`
should  be left out of the YAML configuration file. Instead, Partition Labels
are used in to tag and convey which partitions should be used for an advanced
installation.

Partition Label | Description | Required?
------------ | ------------- | ------------- 
`CLR_BOOT` | The /boot partition; must be vfat | Yes
`CLR_ROOT` | The / root partition; must be ext[234], xfs, or f2fs due to clr-boot-manager requirement | Yes
`CLR_SWAP` | A optional swap partition; can be more than one or used in place of a swapfile | No
`CLR_MNT_<mount_point>` | Any additional partitions that should be included in the install like /srv, /home, ... | No

#### NOTE:
You may also add `_F` to the partition label to force the formatting.
#### EXAMPLES:
Partition Label | Description
------------ | -------------
`CLR_F_SWAP` | Label a partition to be used as swap, and have the installer run mkswap on the partition.
`CLR_MNT_/home` | Label a partition to be mounted as `/home`.
`CLR_F_MNT_/data` | Label a partition to be mounted as `/data`, and have the installer run mkfs on the partition.

## Clear Linux Bundles
This is a list of the Clear Linux OS Bundles that should be installed during the installation of the OS on the target media.

```yaml
bundles: [os-core, os-core-update, clr-installer]
```

This is a list of Clear Linux OS Bundles that will be used to populate the `bundles` field in the target media's custom config file.

```yaml
targetBundles: [desktop-autostart, vim]
```

For a current list of available bundles, refer to:
https://github.com/clearlinux/clr-bundles


## Users
A set of user accounts can be created at the time of installation.

Item | Description | Required?
------------ | ------------- | ------------- 
`login:` | Name of the user's login | Yes
`username:` | The full name of the user. | No
`password:` | The encrypted password suitable for the /etc/passwd file. This string can be generated using `clr-installer --genpass <passwd>` | No
`ssh-keys:` | A list of SSH keys add to the `.ssh/authorized_keys` file for the account | No
`admin` | Boolean value if this account is an administrative and should be included in the `wheel` group | No


```yaml
users:
- login: clrlinux
  username: Clear Linux OS
  admin: true
```

For a current list of available bundles, refer to:
https://github.com/clearlinux/clr-bundles


## Installation Options
Item | Description | Default
------------ | ------------- | ------------- 
`keyboard:` | Name of the keyboard type. Valid value can be found using `localectl list-keymaps`; may require installing the `kbd` bundle first. | us
`language:` | Name of the system language. Valid values can be found using `locale -a`; may require installing the `glibc-locale` bundle first. | en_US.UTF-8
`timezone:` | Name of the system timezone. Valid values can be found using `timedatectl list-timezones`; may require installing the `tzdata` bundle first. | UTC
`swapFileSize:` | Size of the swapfile. If set to `0` no swapfile will be created. The suffixes `B` for bytes, `K` or `KB` for kilobytes, `M` or `MB` for megabytes, `G` or `GB` for gigabytes, `KiB` for kibibyte, `MiB` for mebibyte, `GiB` for gibibyte. | `-UNDEFINED-`
`kernel` | Kernel bundle to be used | kernel-native
`httpsProxy` | HTTPS Proxy as a string | `-UNDEFINED-`
`allowInsecureHTTP` | Allow installation and downloads over insecure connections | false
`hostname` | Name of the host system | `-UNIQUE RANDOM-`
`version` | Version of Clear Linux OS to install | `-LATEST_VERSION-`
`copySwupd` | Copy /etc/swupd configuration files to target | false (true for user-interface installs)
`swupdFormat` | swupd format to use for the installation. | `-FORMART_ON_BUILD_SYSTEM-`
`swupdMirror` | URL of the swupd stream to use. Useful for installing from a local mirror or from a locally published mix. | `-UNDEFINED-`
`swupdSkipOptional` | Don't install optionally included bundles; true or false | false
`autoUpdate` | Should the system automatically update to the latest release of Clear Linux OS as part of the installation?; true or false | true
`offline` | Install update content for minimal offline installation | false
`postReboot` | Should the system reboot after the installation completes?; true or false | true
`postArchive` | Should the system archive the log and configuration file on the target media?; true or false | true
`legacyBios` | Is the install using the Legacy boot from BIOS?; true or false | false
`copyNetwork` | Copy the locally configured network interfaces to target; `/etc/systemd/network` | false
`iso` | Generate a bootable ISO image file?; true or false | false
`isoPublisher` | Publisher string added to ISO metadata; 128 char max | `-UNDEFINED-`
`isoApplicationId` | Publisher string added to ISO metadata; 128 char max | server|desktop determined by bundle list
`keepImage` | Retain the raw image file?; true or false | true (false when iso is true)
`skipValidationSize` | Skip the size requirement checks during partition validation; may be set/overridden with the --skip-validation-size command line option | false
`telemetry` | Should telemetry be enabled by default; true or false | false
`telemetryURL` | URL of where the telemetry records should publish | `-UNDEFINED-`
`telemetryPolicy` | Policy string displayed to users during interactive installs | `-UNDEFINED-`

```yaml

keyboard: us
language: en_US.UTF-8
timezone: UTC
kernel: kernel-native
autoUpdate: false
postArchive: false
postReboot: false
telemetry: false
```


## Kernel Arguments
Supports adding or removing kernel arguments. There is NO support for directly defining the entire kernel command line in order to avoid non-bootable configurations.

Item | Description | Required?
------------ | ------------- | ------------- 
`add:` | A YAML list of strings with additional kernel parameters. These are always appending to the pre-defined kernel parameters.| No
`remove:` | A YAML list of strings to attempt to remove from the pre-defined kernel parameters. Only exact matches are removed. | No


```yaml
kernel-arguments: {
  add: ["nomodeset", "i915.modeset=0"],
  remove: ["console=ttyS0,115200n8"]
}
```

## Installation Hooks
Clear Linux OS Installer supports `pre-install`, `post-install`, and `post-image` hooks which are executed either before (pre) the start of the installation, after (post) the installation steps are completed, or after (post) the image file is created.

Item | Description | Required?
------------ | ------------- | ------------- 
`cmd:` | The command to run plus any arguments; usually passing `chrootDir`| Yes
`chroot:` | Boolean indicating if this command should be run chrooted | No


### Environment Variables
In addition to the environment variables defined in the `env` section of the YAML file, two internal variables are also predefined for use with hooks:

Environment Variable | Description
------------ | ------------- 
`yamlDir` | The directory where the configuration YAML file resides. This is useful as most installation hooks are stored in (or relative to) the same directory as the YAML file.
`chrootDir` | The directory where the installation is being placed (chrooted). This should be passed as an argument to the installation hook to ensure modifications are made to the correct location of the install.
`imageFile` | The file name of the image file generated by the installer. Only useful for `post-image` hooks.

```yaml
post-install: [
   {cmd: "${yamlDir}/installer-post.sh ${chrootDir}"}
]

post-image: [
   {cmd: "xz -q -T0 --stdout ${imageFile} > ${imageFile}.xz"}
]
```

