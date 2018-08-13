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
git clone https://github.intel.com/iclr/clr-installer.git
```

## Build the installer

```
cd clr-installer && make
```

## Install (installing the installer)

If you want to actually install the clr-installer to your system (it's not necessary but possible), run:

```
make install
```

It's possible to specify a ```DESTDIR``` environment variable and change the root directory for your install, i.e:

```
make DESTDIR=/opt/my-root/ install
```

## Run as root

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

## Installing to an image file
Follow the steps below to create a raw image file and perform a Clear Linux install to it.

Create a raw image file with qemu-img:

```
qemu-img create -f raw clr-linux.img 4G
```

Setup a loop device:

```
sudo losetup --find --show clr-linux.img
```

> This command will display the just created loop device file i.e /dev/loop0

Now you can launch the installer and use this loop device to perform an install to it.

After finishing the install you may want to detach the loop device, i.e:

```
sudo losetup -d /dev/loop0
```

> Adapt this line to reflect the loop device file created in the second step