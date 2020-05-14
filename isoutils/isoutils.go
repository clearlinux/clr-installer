// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package isoutils

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/utils"
)

type paths int

const (
	clrEfi    paths = iota + 0
	clrImgEfi       // Location to mount the EFI partition in the passed-in img file
	clrInitrd
	clrRootfs
	clrCdroot
)

var (
	tmpPaths = make([]string, 5)
)

func mkTmpDirs() error {
	msg := "Making temp directories for ISO creation"
	prg := progress.NewLoop(msg)
	log.Info(msg)
	var err error

	tmpPaths[clrEfi], err = ioutil.TempDir("", "clrEfi-")
	if err != nil {
		prg.Failure()
		return err
	}
	tmpPaths[clrInitrd], err = ioutil.TempDir("", "clrInitrd-")
	if err != nil {
		prg.Failure()
		return err
	}
	tmpPaths[clrCdroot], err = ioutil.TempDir("", "clrCdroot-")
	if err != nil {
		prg.Failure()
		return err
	}

	/* Create specific directories for the new cd's root */
	for _, d := range []string{
		tmpPaths[clrCdroot] + "/isolinux",
		tmpPaths[clrCdroot] + "/EFI",
		tmpPaths[clrCdroot] + "/images",
		tmpPaths[clrCdroot] + "/kernel",
	} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			err = os.Mkdir(d, os.ModePerm)
			if err != nil {
				prg.Failure()
				return err
			}
		}
	}

	prg.Success()
	return err
}

func mkRootfs() error {
	msg := "Making squashfs of rootfs"
	prg := progress.NewLoop(msg)
	log.Info(msg)

	/* TODO: This takes a long time to run, it'd be nice to see it's output as it's running */
	args := []string{
		"mksquashfs",
		tmpPaths[clrRootfs],
		tmpPaths[clrCdroot] + "/images/rootfs.img",
		"-b",
		"131072",
		"-comp",
		"gzip",
		"-e",
		"boot/",
		"-e",
		"proc/",
		"-e",
		"sys/",
		"-e",
		"dev/",
		"-e",
		"run/",
	}
	err := cmd.RunAndLog(args...)
	if err != nil {
		prg.Failure()
		return err
	}
	prg.Success()
	return err
}

// GetIsoLoopDevice returns the name of the ISO loop device
func GetIsoLoopDevice() string {
	w := bytes.NewBuffer(nil)
	args := []string{
		"losetup",
		"-j",
		"/mnt/media/images/rootfs.img",
		"-O",
		"NAME",
		"-n",
	}
	err := cmd.Run(w, args...)
	if err != nil {
		return ""
	}
	if strings.Compare(w.String(), "") == 0 {
		return ""
	}

	return strings.TrimSuffix(w.String(), "\n")
}

// ExtractSquashfs extracts the src directory from a squashfs to the dest directory
func ExtractSquashfs(src, dest, loopDev string) error {
	args := []string{
		"unsquashfs",
		"-d",
		dest,
		"-f",
		loopDev,
		"-e",
		src,
	}
	err := cmd.RunAndLog(args...)
	if err != nil {
		return err
	}
	return err
}

func mkInitrd(version string, model *model.SystemInstall, options args.Args) error {
	msg := "Installing the base system for initrd"
	var prg progress.Progress

	log.Info(msg)

	var err error
	options.SwupdStateDir = tmpPaths[clrInitrd] + "/var/lib/swupd/"
	options.SwupdFormat = "staging"
	sw := swupd.New(tmpPaths[clrInitrd], options, model)

	/* Install os-core and os-core-plus (we only need kmod-bin) as initrd */
	if err := sw.OSInstall(version, swupd.IsoPrefix, []string{"clr-installer-iso-init"}); err != nil {
		prg = progress.NewLoop(msg)
		prg.Failure()
		return err
	}
	prg = progress.NewLoop(msg)
	prg.Success()
	return err
}

func mkInitrdInitScript(templatePath string) error {
	msg := "Creating and installing init script to initrd"
	prg := progress.NewLoop(msg)
	log.Info(msg)

	type Modules struct {
		Modules            []string
		IsoMediaBootOption string
	}
	mods := Modules{IsoMediaBootOption: args.KernelMediaCheck}

	//Modules to insmod during init, paths relative to the kernel folder
	modules := []string{
		"/kernel/fs/isofs/isofs.ko",
		"/kernel/drivers/cdrom/cdrom.ko",
		"/kernel/drivers/scsi/sr_mod.ko",
		"/kernel/fs/overlayfs/overlay.ko",
	}

	/* Find kernel, then break the name into kernelVersion */
	kernelGlob, err := filepath.Glob(tmpPaths[clrRootfs] + "/lib/kernel/org.clearlinux.*")
	if err != nil || len(kernelGlob) != 1 {
		prg.Failure()
		log.Error("Failed to determine kernel revision or > 1 kernel found")
		return err
	}
	kernelTypeVersion := strings.SplitAfter(filepath.Base((kernelGlob[0])), "org.clearlinux.")[1]
	kernelType := strings.Split(kernelTypeVersion, ".")[0] //kernelType examples: native,kvm,lts2018,hyperv
	kernelVersion := strings.SplitAfter(kernelTypeVersion, kernelType+".")[1]

	/* Copy files to initrd, and add to mods so they're added to the init template */
	for _, i := range modules {
		rootfsModPath := tmpPaths[clrRootfs] + "/usr/lib/modules/" + kernelVersion + "." + kernelType + i

		/* copy kernel module to initramfs */
		initrdModPath := filepath.Dir(tmpPaths[clrInitrd] + "/usr/lib/modules/" + kernelVersion + "." + kernelType + i)

		if _, err := os.Stat(initrdModPath); os.IsNotExist(err) {
			err = os.MkdirAll(initrdModPath, os.ModePerm)
			if err != nil {
				prg.Failure()
				return err
			}
		}

		err = utils.CopyFile(rootfsModPath, initrdModPath+"/"+filepath.Base(i))
		if err != nil {
			prg.Failure()
			return err
		}
		mods.Modules = append(mods.Modules, "/usr/lib/modules/"+kernelVersion+"."+kernelType+i)
	}

	tmpl, err := ioutil.ReadFile(templatePath + "/initrd_init_template")
	if err != nil {
		prg.Failure()
		log.Error("Failed to open: initrd template at %s\n", templatePath+"initrd_init_template")
		return err
	}

	t := template.New("Modules template")
	t, err = t.Parse(string(tmpl))
	if err != nil {
		prg.Failure()
		log.Error("Failed to parse init's template")
		return err
	}

	f, err := os.Create(tmpPaths[clrInitrd] + "/init")
	if err != nil {
		prg.Failure()
		log.Error("Failed to create init file for initrd!")
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	err = t.Execute(f, mods)
	if err != nil {
		prg.Failure()
		log.Error("Failed to execute template filling")
		return err
	}

	log.Debug("Init script contents after template expansion: ")
	// soft error just needed for logging
	_ = cmd.RunAndLog("cat", tmpPaths[clrInitrd]+"/init")

	/* Set correct owner and permissions on initrd's init */
	if err = os.Chown(tmpPaths[clrInitrd]+"/init", 0, 0); err != nil {
		prg.Failure()
		return err
	}
	if err = os.Chmod(tmpPaths[clrInitrd]+"/init", 0700); err != nil {
		prg.Failure()
		return err
	}

	prg.Success()
	return err
}

/* Build initrd image, and copy to the correct location */
func buildInitrdImage() error {
	msg := "Building initrd image"
	prg := progress.NewLoop(msg)
	log.Info(msg)

	// Determine current user's path so we can revert to it when this function ends
	currPath, err := os.Getwd()
	if err != nil {
		prg.Failure()
		return err
	}

	/* find all files in the initrd path, create the initrd */
	/* The find command must return filenames without a path (eg, must be run in the current dir) */
	err = os.Chdir(tmpPaths[clrInitrd])
	if err != nil {
		prg.Failure()
		return err
	}

	initrdPath := tmpPaths[clrCdroot] + "/EFI/BOOT/"
	if _, err := os.Stat(initrdPath); os.IsNotExist(err) {
		err = os.MkdirAll(initrdPath, os.ModePerm)
		if err != nil {
			prg.Failure()
			return err
		}
	}

	args := "sudo find .| cpio -o -H newc | gzip >" + initrdPath + "initrd.gz"
	_, err = exec.Command("bash", "-c", args).CombinedOutput()
	if err != nil {
		prg.Failure()
		return err
	}

	err = os.Chdir(currPath)
	if err != nil {
		prg.Failure()
		return err
	}

	prg.Success()
	return err
}

// We take a copy of options from normal EFI entry and edit it for new EFI entry: /loader/entries/iso-checksum.conf
func prepareISOCheckSumBootEntry(inputOptions []string) []string {
	outputOptions := append([]string(nil), inputOptions...)
	for i, option := range outputOptions {
		if strings.Contains(option, "title") {
			outputOptions[i] = "title Verify ISO Integrity"
		}

		if i == len(outputOptions)-1 {
			outputOptions[i] = strings.TrimSpace(option) + " " + args.KernelMediaCheck
		}
	}
	return outputOptions
}

func mkEfiBoot() error {
	msg := "Building efiboot image"
	prg := progress.NewLoop(msg)
	log.Info(msg)

	cmds := [][]string{
		{"fallocate", "-l", "100M", tmpPaths[clrCdroot] + "/EFI/efiboot.img"},
		{"mkfs.fat", "-n", "\"CLEAR_EFI\"", tmpPaths[clrCdroot] + "/EFI/efiboot.img"},
		{"mount", "-t", "vfat", "-o", "loop", tmpPaths[clrCdroot] + "/EFI/efiboot.img", tmpPaths[clrEfi]},
		{"cp", "-pr", tmpPaths[clrImgEfi] + "/.", tmpPaths[clrEfi]},
	}

	for _, i := range cmds {
		err := cmd.RunAndLog(i...)
		if err != nil {
			prg.Failure()
			return err
		}
	}

	/* Modify loader/entries/Clear-linux-*, add initrd= line and remove ROOT= from kernel command line options */
	entriesGlob, err := filepath.Glob(tmpPaths[clrEfi] + "/loader/entries/Clear-linux-*")
	entriesIsoChecksum := filepath.Join(tmpPaths[clrEfi] + "/loader/entries/iso-checksum.conf")
	if err != nil || len(entriesGlob) != 1 {
		prg.Failure()
		log.Error("Failed to modify efi entries file")
		return err
	}

	input, err := ioutil.ReadFile(entriesGlob[0])
	if err != nil {
		prg.Failure()
		log.Error("Failed to read EFI entries file")
		return err
	}

	/* Replace current options line with initrd information, extract options line for modification */
	lines := strings.Split(string(input), "\n")
	var optionsLine string
	for i, line := range lines {
		if strings.Contains(line, "options") {
			optionsLine = line
			lines[i] = "initrd /EFI/BOOT/initrd.gz"
		}
	}

	options := strings.Split(optionsLine, " ")
	for i, option := range options {
		if strings.Contains(option, "PARTUUID") {
			options = append(options[:i], options[i+1:]...) //remove slice from options
			break                                           //no other ops
		}
	}
	lines = append(lines, strings.Join(options, " "))

	err = ioutil.WriteFile(entriesGlob[0], []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		prg.Failure()
		log.Error("Failed to write kernel boot parameters file")
		return err
	}

	isoBootMenuLines := prepareISOCheckSumBootEntry(lines)
	err = ioutil.WriteFile(entriesIsoChecksum, []byte(strings.Join(isoBootMenuLines, "\n")), 0644)
	if err != nil {
		prg.Failure()
		log.Error("Failed to write kernel boot parameters file for isomd5sum check")
		return err
	}

	/* Copy EFI files to the cdroot for Rufus support */
	cpCmd := []string{"cp", "-pr", tmpPaths[clrEfi] + "/.", tmpPaths[clrCdroot]}
	err = cmd.RunAndLog(cpCmd...)
	if err != nil {
		prg.Failure()
		return err
	}

	/* Copy initrd to efiboot.img and finally unmount efiboot.img */
	initrdPaths := []string{tmpPaths[clrCdroot] + "/EFI/BOOT/initrd.gz", tmpPaths[clrEfi] + "/EFI/BOOT/initrd.gz"}
	err = utils.CopyFile(initrdPaths[0], initrdPaths[1])
	if err != nil {
		prg.Failure()
		return err
	}

	/*
		Copy initrd to iso-checksum.conf.
		If we dont do this, mkLegacyBoot will fail to find it while creating correspoding isolinux entry
	*/
	err = utils.CopyFile(tmpPaths[clrEfi]+"/loader/entries/iso-checksum.conf",
		tmpPaths[clrImgEfi]+"/loader/entries/iso-checksum.conf")
	if err != nil {
		prg.Failure()
		return err
	}

	/* Unmount EFI partition here, because this must be unmounted when calling xorriso! */
	if err := syscall.Unmount(tmpPaths[clrEfi], syscall.MNT_FORCE|syscall.MNT_DETACH); err != nil {
		prg.Failure()
		return err
	}

	prg.Success()
	return err
}

func mkLegacyBoot(templatePath string) error {
	msg := "Setting up BIOS boot with isolinux"
	prg := progress.NewLoop(msg)
	log.Info(msg)

	type BootConf struct {
		Options           string
		OptionsMediaCheck string
	}
	bc := BootConf{}

	/* Find kernel path so we can copy the kernel later */
	kernelGlob, err := filepath.Glob(tmpPaths[clrRootfs] + "/lib/kernel/org.clearlinux.*")
	if err != nil || len(kernelGlob) != 1 {
		prg.Failure()
		return err
	}
	kernelPath := kernelGlob[0]

	paths := [][]string{
		{"/usr/share/syslinux/isohdpfx.bin", tmpPaths[clrCdroot] + "/isolinux/isohdpfx.bin"},
		{"/usr/share/syslinux/isolinux.bin", tmpPaths[clrCdroot] + "/isolinux/isolinux.bin"},
		{"/usr/share/syslinux/ldlinux.c32", tmpPaths[clrCdroot] + "/isolinux/ldlinux.c32"},
		{"/usr/share/syslinux/menu.c32", tmpPaths[clrCdroot] + "/isolinux/menu.c32"},
		{"/usr/share/syslinux/libutil.c32", tmpPaths[clrCdroot] + "/isolinux/libutil.c32"},
		{kernelPath, tmpPaths[clrCdroot] + "/kernel/kernel.xz"},
	}

	for _, i := range paths {
		err = utils.CopyFile(i[0], i[1])
		if err != nil {
			prg.Failure()
			return err
		}
	}

	/* Create the 'boot.txt' file for isolinux */
	bootFile, err := os.Create(tmpPaths[clrCdroot] + "/isolinux/boot.txt")
	if err != nil {
		prg.Failure()
		return err
	}
	defer func() {
		_ = bootFile.Close()
	}()

	_, err = bootFile.WriteString("\n\nClear Linux OS for Intel Architecture\n")
	if err != nil {
		prg.Failure()
		return err
	}

	// inner Function to remove filter unwanted parts like PARTUUID, word: `option` for different boot menus
	filterOptionsLineFunc := func(optionLine []byte) string {
		/* Read options from the EFI partition, remove the string 'options' and root=PARTUUID from the options line */
		lines := strings.Split(string(optionLine), "\n")
		var optionsLineTemp string
		for _, line := range lines {
			if strings.Contains(line, "options") {
				optionsLineTemp = line
			}
		}

		options := strings.Split(optionsLineTemp, " ")
		for i, option := range options {
			if strings.Contains(option, "options") || strings.Contains(option, "PARTUUID") {
				options[i] = ""
			}
		}

		return strings.Join(options, " ")
	}

	/* Find the (kernel boot) options file, load it into bc.Options */
	optionsGlob, err := filepath.Glob(tmpPaths[clrImgEfi] + "/loader/entries/Clear-linux-*")
	if err != nil || len(optionsGlob) > 1 { // Fail if there's >1 match
		prg.Failure()
		log.Error("Failed to determine boot options for kernel")
		return err
	}
	optionsFile, err := ioutil.ReadFile(optionsGlob[0])
	if err != nil {
		prg.Failure()
		log.Error("Failed to read options file from rootfs")
		return err
	}

	bc.Options = filterOptionsLineFunc(optionsFile)

	// ISO Integrity boot option
	entriesIsoChecksum, err := filepath.Glob(tmpPaths[clrImgEfi] + "/loader/entries/iso-checksum*")
	if err != nil || len(entriesIsoChecksum) > 1 { // Fail if there's >1 match
		prg.Failure()
		log.Error("Failed to determine boot options for ISO Integrity check")
		return err
	}

	optionsFileISO, err := ioutil.ReadFile(entriesIsoChecksum[0])
	if err != nil {
		prg.Failure()
		log.Error("Failed to read options file from iso boot menu")
		return err
	}

	bc.OptionsMediaCheck = filterOptionsLineFunc(optionsFileISO)

	/* Fill boot options in isolinux.cfg */
	tmpl, err := ioutil.ReadFile(templatePath + "/isolinux.cfg.template")
	if err != nil {
		prg.Failure()
		log.Error("Failed to find template")
		return err
	}

	t := template.New("Modules template")
	t, err = t.Parse(string(tmpl))
	if err != nil {
		prg.Failure()
		log.Error("Failed to parse template.")
		return err
	}

	f, err := os.Create(tmpPaths[clrCdroot] + "/isolinux/isolinux.cfg")
	if err != nil {
		prg.Failure()
		log.Error("Failed to create isolinux.cfg on cd root!")
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	err = t.Execute(f, bc)
	if err != nil {
		prg.Failure()
		log.Error("Failed to execute template filling")
		return err
	}

	prg.Success()
	return err
}

func implantIsoChecksum(imgName string) error {
	msg := "Adding Checksums for ISO Integrity"
	prg := progress.NewLoop(msg)
	log.Info(msg)

	args := []string{
		"implantisomd5",
	}

	if len(imgName) > 0 {
		isoName := imgName + ".iso"
		args = append(args, isoName)
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		prg.Failure()
		return err
	}

	prg.Success()
	return err
}

func packageIso(imgName, appID, publisher string) error {
	msg := "Building ISO"
	prg := progress.NewLoop(msg)
	log.Info(msg)

	args := []string{
		"xorriso", "-as", "mkisofs",
		"-o", imgName + ".iso",
		"-V", "CLR_ISO",
	}

	if len(appID) > 0 {
		args = append(args, "-appid", appID)
	}
	if len(publisher) > 0 {
		args = append(args, "-publisher", publisher)
	}

	args = append(args,
		"-isohybrid-mbr", tmpPaths[clrCdroot]+"/isolinux/isohdpfx.bin",
		"-c", "isolinux/boot.cat", "-b", "isolinux/isolinux.bin",
		"-no-emul-boot", "-boot-load-size", "4", "-boot-info-table",
		"-eltorito-alt-boot", "-e", "EFI/efiboot.img", "-no-emul-boot",
		"-isohybrid-gpt-basdat", tmpPaths[clrCdroot],
	)

	err := cmd.RunAndLog(args...)
	if err != nil {
		prg.Failure()
		return err
	}

	prg.Success()
	return err
}

func cleanup() {
	msg := "Cleaning up from ISO creation"
	prg := progress.NewLoop(msg)
	log.Info(msg)
	var err error

	/* In case something fails during mkEfiBoot, check and umount clrImgEfi */
	if err = syscall.Unmount(tmpPaths[clrEfi], syscall.MNT_FORCE|syscall.MNT_DETACH); err != nil {
		// Failed to unmount, usually the normal case, but could be umount actually failed.
	}

	/* Remove all directories in /tmp/clr_* */
	for _, d := range tmpPaths {
		if d == tmpPaths[clrRootfs] || d == tmpPaths[clrImgEfi] { //both these paths are handled by clr-installer
			continue
		}
		err = os.RemoveAll(d)
		if err != nil {
			log.Warning("Failed to remove dir: %s", d)
		}
	}
	prg.Success()
}

/*MakeIso creates an ISO image from a built image in the current directory*/
func MakeIso(rootDir string, imgName string, model *model.SystemInstall, options args.Args) error {
	tmpPaths[clrRootfs] = rootDir
	tmpPaths[clrImgEfi] = rootDir + "/boot"
	var err error

	templateDir, err := utils.LookupISOTemplateDir()
	if err != nil {
		return err
	}
	// Determine version from the root filesystem
	version, err := ioutil.ReadFile(rootDir + "/usr/share/clear/version")
	if err != nil {
		return err
	}

	if err = mkTmpDirs(); err != nil {
		return err
	}
	defer cleanup()

	if err = mkRootfs(); err != nil {
		return err
	}

	if err = mkInitrd(string(version), model, options); err != nil {
		return err
	}

	if err = mkInitrdInitScript(templateDir); err != nil {
		return err
	}

	if err = buildInitrdImage(); err != nil {
		return err
	}

	if err = mkEfiBoot(); err != nil {
		return err
	}

	err = mkLegacyBoot(templateDir)
	if err != nil {
		return err
	}

	appID := model.ISOApplicationID
	if len(appID) == 0 {
		appID = "server"
		if model.IsTargetDesktopInstall() {
			appID = "desktop"
		}
	}

	if err = packageIso(imgName, appID, model.ISOPublisher); err != nil {
		return err
	}

	if err = implantIsoChecksum(imgName); err != nil {
		return err
	}

	return err
}
