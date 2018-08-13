# Clear Linux Installer

## Dependencies
The following bundles are required in order to run clr-installer:

+ sysadmin-basic (for kbd)
+ storage-utils
+ network-basic

## How to test?
Make sure you have any extra storage device, an USB memory stick should work fine, the installer will detect and use it if you choose.

## Clone this repository

```
git clone https://github.com/clearnlinux/clr-installer.git
```

## Build the installer

```
cd clr-installer && make
```

## Install (installing the installer)

To create a bootable image which will launch the installer, use the `mk-installer-image.sh` in the scripts directory.

Create a bootable installer on USB media:
```
scripts/mk-installer-image.sh <usb device>
```

To create a raw image file and perform a Clear Linux install to it.
```
scripts/mk-installer-image.sh
```

## Testing [Run as root]

In order to execute an install the user must run clr-installer as root. It's always possible to tweak configurations and only __save__ the configuration for future use, in that case it's not required to run as root.

Having said that, to run a install do:

```
sudo .gopath/bin/clr-installer
```

# Multiple Installer Modes
Currently the installer supports 2 modes (a third one is on the way):
1. Mass Installer - using an install descriptor file
2. TUI - a text based user interface
3. GUI - a graphical user interface (yet to come)

## Using Mass Installer
In order to use the Mass Installer provide a ```--config```, such as:

```
sudo .gopath/bin/clr-installer --config ~/my-install.yaml
```

## Using TUI
Call the clr-installer executable without any additional flags, such as:

```
sudo .gopath/bin/clr-installer
```

## Reboot
If you're running the installer on a development machine you may not want to reboot the system after the install completion, for that use the ```--reboot=false``` flag, such as:

```
sudo .gopath/bin/clr-installer --reboot=false
```

or if using the Mass Installer mode:

```
sudo .gopath/bin/clr-installer --config=~/my-install.yaml --reboot=false
```

