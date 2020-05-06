**Why did we do this?**

Supporting Legacy systems is (or was) a requirement of the installation media only. This means that Clear Linux itself does (or did) not support booting on non-EFI systems at all. ISO support was added mainly to support legacy boot and also because it is industry standard.

There was already an example of how this is done in the release tools (create-iso.sh), much of how ISO generation works was built on the process in this script.

**How is the ISO boot process different?**

The normal boot process uses a 2-stage bootloader with a shim (bootloaderx64.efi) and systemd-boot (loaderx64.efi) as the second stage.

A very broad overview looks like this:

(system boot) -> (EFI firmware) -> (bootloaderx64.efi) -> (loaderx64.efi) -> Kernel

The ISO has more steps, and two distinct boot flows

On EFI:

(system boot) -> (EFI firmware) -> (bootloaderx64.efi) -> (loaderx64.efi) -> (initrd) -> (initrd init, syscheck and fs overlay) -> (switch_root and launch init)

On Legacy:

(system boot) -> (MBR on disk and isolinux) -> (initrd) -> (initrd init, syscheck and fs overlay) -> (switch_root and launch init)

For much more detail, check the clear-boot-manager source:

[https://github.com/clearlinux/clr-boot-manager/blob/master/src/bootloaders/shim-systemd.c](https://github.com/clearlinux/clr-boot-manager/blob/master/src/bootloaders/shim-systemd.c)

Also, the Arch Linux wiki has a great rundown of systemd-boot:

[https://wiki.archlinux.org/index.php/Systemd-boot](https://wiki.archlinux.org/index.php/Systemd-boot)

**What does initrd’s init do?**

This init prepares the system to run.

It:

* Mounts temporary filesystems (/sys, /dev, /etc, etc)

* Runs a series of system compatibility tests

* Loads a minimum set of required modules

* Finds the filesystem containing rootfs.img, mounts it, then mounts rootfs.img

* Sets up a RAMFS of 512MB, overlays the new rootfs on top of the RAMFS

* Finally executes switch_root to the overlaid filesystem.

The section that most often fails is finding the installer media - this is done by executing `blkid -L CLR_ISO`, which converts a filesystem LABEL to the associated device name.

CLR_ISO is a special label, which is ignored by clr-installer via a blacklist (so the boot media isn’t installable). One cannot install to a device which has a filesystem labeled CLR_ISO.

When understanding init, execution starts at the bottom of the shell file in the main() function.

**Detailed description of ISO creation:**

ISO generation is a multi-step serial process that’s executed *after* all other image-creation tasks are completed. This module makes extensive reuse of pieces that have been completed by previous image-creation tasks, especially leveraging the rootfs and EFI partitions being accessible.

ISO creation is a serial process, consisting of 8 steps:

* mkTempDirs()

    * Creates temporary directories for ISO creation:

        * EFI

        * Initrd

        * cdroot

* mkRootfs()

    * Mksquashfs from the already-created image’s root filesystem

        * Creates rootfs.img in the cd root’s ‘images’ directory

        * Gzip and a specific block size, as used by Ubuntu. They tested many different block sizes and compression algorithms. This specific combo was the best compromise between size, speed, and support.

* mkInitrd()

    * Creates an initrd by installing os-core bundle to a new image

        * Uses the same *version* as the already-created image

* mkInitrdInitScript()

    * Discovers kernel, determines the type and kernel version

    * Creates module directory structure in initrd, copies required modules

    * Finds initrd’s *init* template (initrd_init_template)

        * Fills in insmod commands during init for the above kernel modules

* buildInitrdImage()

    * Builds the initrd images by CPIO’ing the directory result from mkInitrd() AND mkInitrdInitScript()

        * Has to change directory to do this, changes back to previous after CPIO command is complete

    * Creates /EFI/BOOT/initrd.gz directory structure and file on the new cd root

* mkEfiBoot()

    * Creates EFI image in cd root/EFI/efiboot.img

        * Required by xorriso

    * Copies all files from the already-created image’s EFI partition

    * Modifies kernel command line parameters to support initrd

        * Adds a kernel command line option to enable initrd (initrd /EFI/BOOT/initrd.gz)

        * Removes PARTUUID

    * Copies all files from already-created image’s EFI partition to the cd root

        * Specifically to support Rufus

    * Initrd is copied to efiboot.img

        * EFI booting appears to treat the root of efiboot.img as ‘/’ - legacy booting treats the cd root as ‘/’

    * efiboot.img is unmounted. This step is done here because xorriso requires this image be unmounted, and defer() is done too late.

* mkLegacyBoot()

    * Installs isolinux

        * Copies isolinux required files from the *local machine*, this is why clr-installer has a dependency on syslinux.

    *  Creates boot.txt, loader.cfg, entry configuration files, and isolinux.cfg

* PackageIso()

    * Executes xorriso to create the ISO

**ISO root list of files:**

 tree

.<br>
├── EFI<br>
│   ├── BOOT<br>
│   │   ├── BOOTX64.EFI<br>
│   │   └── initrd.gz<br>
│   ├── efiboot.img<br>
│   └── org.clearlinux<br>
│   	├── bootloaderx64.efi<br>
│   	├── freestanding-00-intel-ucode.cpio<br>
│   	├── freestanding-i915-firmware.cpio.xz<br>
│   	├── kernel-org.clearlinux.native.5.1.15-791<br>
│   	└── loaderx64.efi<br>
├── images<br>
│   └── rootfs.img<br>
├── isolinux<br>
│   ├── boot.cat<br>
│   ├── boot.txt<br>
│   ├── isohdpfx.bin<br>
│   ├── isolinux.bin<br>
│   ├── isolinux.cfg<br>
│   ├── ldlinux.c32<br>
│   ├── libutil.c32<br>
│   └── menu.c32<br>
├── kernel<br>
│   └── kernel.xz<br>
└── loader<br>
	├── entries<br>
	│   └── Clear-linux-native-5.1.15-791.conf<br>
	└── loader.conf<br>

**Debugging initrd’s _init_**

Sometimes, the live ISO fails to boot. A recent example is this bug: [https://github.com/clearlinux/clr-installer/issues/465](https://github.com/clearlinux/clr-installer/issues/465)

The init script is verbose enough to be able to tell where and why a failure happened in many cases. In this case, the message "Searching for installer media, retrying…" is right in the find_and_mount_installer() function in the initrd_init_template file (located in the iso_templates folder)

We can see that the init script is searching for the installer media by its label (which is CLR_ISO). In this case, the script cannot find the installer media (the cd root).

What do we do now? How do we get more information or work with this script interactively?

The easiest way to resolve this kind of issue is to add "exec /usr/bin/sh" somewhere it’ll be executed.

Insert this line, ensure the new template will be used in the new image by copying over /usr/share/clr-installer/iso_templates/initrd_init_template or using the environment variable CLR_INSTALLER_ISO_TEMPLATE_DIR, or ensure it’s in .gopath/iso_templates, and build a new image.

Note: if you’re feeling hacky, and your media is r/w, you can also just mount the filesystem, then extract initrd.gz with CPIO, modify the script, and re-create initrd.gz with CPIO. See [https://access.redhat.com/solutions/24029](https://access.redhat.com/solutions/24029) for commands to extract/compress.

On the next boot, **remove** the command line parameter console=ttyS0,112500n8 (unless you’re using serial), otherwise sh will only accept i/o on the serial port.

Now we can execute the commands that init would execute ourselves, and see where the problems may lie. For this bug, I suspect there’s something off about how hyperv treats filesystem labels.

Check the bug for the solution, it wound up taking much longer than I thought.

**Useful links:**

[https://wiki.syslinux.org/wiki/index.php?title=Isohybrid](https://wiki.syslinux.org/wiki/index.php?title=Isohybrid)

[https://www.rodsbooks.com/gdisk/hybrid.html#bootloaders](https://www.rodsbooks.com/gdisk/hybrid.html#bootloaders)

