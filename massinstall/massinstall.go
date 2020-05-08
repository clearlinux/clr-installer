// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package massinstall

import (
	"fmt"
	"time"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// rebootDelay is the number of second before automatic reboot
	rebootDelay = 5
)

// MassInstall is the frontend implementation for the "mass installer" it also
// implements the progress interface: progress.Client
type MassInstall struct {
	prgDesc  string
	prgIndex int
	step     int
}

// New creates a new instance of MassInstall frontend implementation
func New() *MassInstall {
	return &MassInstall{}
}

func printPipedStatus(mi *MassInstall) bool {
	isStdoutTTY := utils.IsStdoutTTY()
	mi.step++

	if !isStdoutTTY && mi.step == 1 {
		fmt.Println(mi.prgDesc)
		return true
	} else if !isStdoutTTY {
		return true
	}

	return false
}

// Step is the progress step implementation for progress.Client interface
func (mi *MassInstall) Step() {
	if printPipedStatus(mi) {
		return
	}

	elms := []string{"|", "-", "\\", "|", "/", "-", "\\"}

	fmt.Printf("%s [%s]\r", mi.prgDesc, elms[mi.prgIndex])

	if mi.prgIndex+1 == len(elms) {
		mi.prgIndex = 0
	} else {
		mi.prgIndex = mi.prgIndex + 1
	}
}

// LoopWaitDuration is part of the progress.Client implementation and returns the
// duration each loop progress step should wait
func (mi *MassInstall) LoopWaitDuration() time.Duration {
	return 50 * time.Millisecond
}

// Desc is part of the implementation for ProgresIface and is used to adjust the progress bar
// label content
func (mi *MassInstall) Desc(desc string) {
	mi.prgDesc = desc
}

// Partial is part of the progress.Client implementation and sets the progress bar based
// on actual progression
func (mi *MassInstall) Partial(total int, step int) {
	if printPipedStatus(mi) {
		return
	}

	line := fmt.Sprintf("%s %.0f%%\r", mi.prgDesc, (float64(step)/float64(total))*100)
	fmt.Printf("%s", line)
}

// Success is part of the progress.Client implementation and represents the
// successful progress completion of a task
func (mi *MassInstall) Success() {
	if !utils.IsStdoutTTY() {
		mi.step = 0
		return
	}

	mi.prgIndex = 0
	fmt.Printf("%s [success]\n", mi.prgDesc)
}

// Failure is part of the progress.Client implementation and represents the
// unsuccessful progress completion of a task
func (mi *MassInstall) Failure() {
	if !utils.IsStdoutTTY() {
		mi.step = 0
		return
	}

	mi.prgIndex = 0
	fmt.Printf("%s [*failed*]\n", mi.prgDesc)
}

// MustRun is part of the Frontend implementation and tells the core implementation that this
// frontend wants or should be executed
func (mi *MassInstall) MustRun(args *args.Args) bool {
	return args.ConfigFile != "" && (!args.ForceTUI && !args.ForceGUI)
}

// Run is part of the Frontend implementation and is the actual entry point for the
// "mass installer" frontend
func (mi *MassInstall) Run(md *model.SystemInstall, rootDir string, options args.Args) (bool, error) {
	var instError error
	var devs []*storage.BlockDevice
	var results []string

	// If there are no media defined, then we should look for
	// Advanced Configuration labels
	if len(md.TargetMedias) > 0 {
		// If the partitions are defined from the configuration file,
		// assume the user knows what they are doing and ignore validation checks
		if !options.SkipValidationSizeSet && !options.SkipValidationAllSet {
			md.MediaOpts.SkipValidationSize = true
			md.MediaOpts.SkipValidationAll = true
		} else {
			if !options.SkipValidationSizeSet {
				md.MediaOpts.SkipValidationSize = true
			} else {
				if !options.SkipValidationAllSet {
					md.MediaOpts.SkipValidationAll = false
				}
			}
		}

		// Need to ensure the partitioner knows we are running from
		// the command line and will be using the whole disk
		for _, curr := range md.TargetMedias {
			md.InstallSelected[curr.Name] = storage.InstallTarget{Name: curr.Name, WholeDisk: true}
			log.Debug("Mass installer using defined media in YAML")
		}

		if md.IsTargetDesktopInstall() {
			results = storage.DesktopValidatePartitions(md.TargetMedias, md.MediaOpts)
		} else {
			results = storage.ServerValidatePartitions(md.TargetMedias, md.MediaOpts)
		}

		if len(results) > 0 {
			for _, errStr := range results {
				log.Error("Disk Partition: Validation Error: %q", errStr)
				fmt.Printf("Disk Partition: Validation Error: %q\n", errStr)
			}

			return false, errors.Errorf("Disk partitions failed validation")
		}
	} else {
		// Check for Advance Partitioning labels
		log.Debug("Mass installer found no media in YAML; checking for Advanced Disk Partition Labels.")
		isAdvancedSelected := false
		var err error
		devs, err = storage.ListAvailableBlockDevices(md.TargetMedias)
		log.Debug("massinstall: results of ListAvailableBlockDevices: %+v", devs)

		if err != nil {
			log.Error("Error detecting advanced partitions: %q", err)
			fmt.Printf("Error detecting advanced partitions: %q\n", err)
			return false, err
		}

		devs = storage.FindAdvancedInstallTargets(devs)
		for _, curr := range devs {
			md.AddTargetMedia(curr)
			log.Debug("massinstall: AddTargetMedia %+v", curr)
			md.InstallSelected[curr.Name] = storage.InstallTarget{Name: curr.Name, Friendly: curr.Model,
				Removable: curr.RemovableDevice}
			isAdvancedSelected = true
		}

		if isAdvancedSelected {
			log.Debug("Mass installer operating in Advanced Disk Partition Mode.")
			var results []string
			if md.IsTargetDesktopInstall() {
				results = storage.DesktopValidateAdvancedPartitions(devs, md.MediaOpts)
			} else {
				results = storage.ServerValidateAdvancedPartitions(devs, md.MediaOpts)
			}
			if len(results) > 0 {
				for _, errStr := range results {
					log.Error("Advanced Disk Partition: Validation Error: %q", errStr)
					fmt.Printf("Advanced Disk Partition: Validation Error: %q\n", errStr)
				}

				return false, errors.Errorf("Disk partitions failed validation")
			}
		} else {
			log.Error("Failed to detected advanced partition labels!")
			fmt.Println("Failed to detected advanced partition labels!")
			return false, errors.Errorf("Failed to detected advanced partition labels!")
		}
	}

	progress.Set(mi)

	log.Debug("Starting install")

	if !md.AutoUpdate.Value() {
		fmt.Println("Swupd auto-update set to off!")
	}

	instError = controller.Install(rootDir, md, options)
	if instError != nil {
		if !errors.IsValidationError(instError) {
			fmt.Printf("ERROR: Installation has failed!\n")
		}
		return false, instError
	}

	if instError != nil {
		return false, instError
	} else if md.PostReboot {
		fmt.Printf("\nSystem will restart -- Control-C to abort!\n\n")
		fmt.Printf("Rebooting in ...")
		for i := rebootDelay; i > 0; i-- {
			fmt.Printf("%d...", i)
			time.Sleep(time.Second * 1)
		}
		fmt.Printf("0\n\n")
	}

	return md.PostReboot, nil
}
