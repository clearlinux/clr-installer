# Clear Linux Installer

## Clear Linux OS Security
As the installer is a part of the Clear Linux OS distribution, this program follows the [Clear Linux OS Security processes](https://clearlinux.org/documentation/clear-linux/concepts/security).

## Dependencies
In order to build and run clr-installer, install the latest clr-installer bundle:

Text-based only
```
swupd bundle-add clr-installer
```

Graphical installer
```
swupd bundle-add clr-installer-gui
```

## How to test?
Make sure there is free storage space, such as a USB memory stick, unallocated disk, or unallocated (free) partition on a disk and choose it while running the installer.

## Clone this repository

```
git clone https://github.com/clearlinux/clr-installer.git
```

## Build the installer

```
cd clr-installer && make
```

## Install (installing the installer)

To create a bootable image which will launch the installer, use the [installer.yaml](../master/scripts/installer.yaml) as the config file.
```
sudo .gopath/bin/clr-installer --config scripts/installer.yaml
```
Refer to [InstallerYAMLSyntax](../master/scripts/InstallerYAMLSyntax.md) for syntax of the config file.

Create a bootable installer on USB media:
```
sudo .gopath/bin/clr-installer --config scripts/installer.yaml -b installer:<usb device> --iso
```

> Note: Replace ```<usb device>``` with the usb's device file as follows:
>
> sudo .gopath/bin/clr-installer --config scripts/installer.yaml -b installer:/dev/sdb --iso
>

## Testing [Run as root]

In order to execute an install the user must run clr-installer as root. It's always possible to tweak configurations and only __save__ the configuration for future use, in that case it's not required to run as root.

Having said that, to run a install do:

```
sudo .gopath/bin/clr-installer
```

# Multiple Installer Modes
Currently the installer supports 3 modes
1. Mass Installer - using an install descriptor file
2. TUI - a text based user interface
3. GUI - a graphical user interface

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
or
```
sudo .gopath/bin/clr-installer-tui
```


## Using GUI
Call the clr-installer executable without any additional flags, such as:

```
sudo .gopath/bin/clr-installer-gui
```

## Reboot
For scenarios where a reboot may not be desired, such as when running the installer on a development machine, use the ```--reboot=false``` flag as follows:

```
sudo .gopath/bin/clr-installer --reboot=false
```

or if using the Mass Installer mode:

```
sudo .gopath/bin/clr-installer --config=~/my-install.yaml --reboot=false
```

