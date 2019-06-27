// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
	"gopkg.in/yaml.v2"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	diskUtilTitle = `Launch Disk Utility`
	diskUtil      = `cfdisk`
)

// DiskUtilPage is the Page implementation for the disk partitioning menu page
type DiskUtilPage struct {
	BasePage

	descrptionLabel *clui.Label

	chooserList *clui.ListBox

	diskUtilTargets []storage.InstallTarget

	labelWarning *clui.Label

	devs         []*storage.BlockDevice
	activeDisk   *storage.BlockDevice
	activeSerial string
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *DiskUtilPage) GetConfiguredValue() string {
	return ""
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *DiskUtilPage) GetConfigDefinition() int {
	return ConfigNotDefined
}

// SetDone sets the configured disk into the model and sets the page as done
func (page *DiskUtilPage) SetDone(done bool) bool {
	log.Debug("%s diskUtil called for Target %v", diskUtil,
		page.diskUtilTargets[page.chooserList.SelectedItem()])
	page.rundiskUtil(page.diskUtilTargets[page.chooserList.SelectedItem()].Name)

	// disk pages
	page.tui.gotoPage(TuiPageMediaConfig, page)

	return false
}

// Activate updates the UI elements with the most current list of block devices
func (page *DiskUtilPage) Activate() {

	page.confirmBtn.SetEnabled(false)

	if err := page.buildMediaLists(); err != nil {
		page.Panic(err)
	}

	page.activated = page.cancelBtn

	clui.RefreshScreen()
}

func (page *DiskUtilPage) setConfirmButton() {
	if page.labelWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

// The disk page gives the user the option so select how to set the storage device,
// if to manually configure it or a guided standard partition schema
func newDiskUtilPage(tui *Tui) (Page, error) {
	page := &DiskUtilPage{}
	page.setup(tui, TuiPageDiskUtil, CancelButton|ConfirmButton, TuiPageMediaConfig)

	// Top label for the page
	lbl := clui.CreateLabel(page.content, 2, 2, diskUtilTitle, clui.Fixed)
	lbl.SetPaddings(0, 1)

	contentFrame := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	contentFrame.SetPack(clui.Vertical)

	page.descrptionLabel = clui.CreateLabel(contentFrame, AutoSize, 7, "", Fixed)
	page.descrptionLabel.SetMultiline(true)
	page.descrptionLabel.SetPaddings(0, 2)
	descriptiveMessage := "Launch an external program (" + diskUtil +
		") to modify the selected\n" +
		"disk in an attempt to allocate enough free disk space to allow\n" +
		"the installation to succeed.\n\n" +
		"Only disk with a 'gpt' partition can be modified.\n\n" +
		"NOTE: The tool can only resize the partition, NOT the filesystem!"
	page.descrptionLabel.SetTitle(descriptiveMessage)

	// Put a blank line between the text label and the listbox
	clui.CreateLabel(contentFrame, 1, 1, "", Fixed)

	listFrame := clui.CreateFrame(contentFrame, 60, 3, BorderNone, Fixed)
	listFrame.SetPack(clui.Vertical)

	page.chooserList = clui.CreateListBox(listFrame, 60, 3, Fixed)
	page.chooserList.SetAlign(AlignLeft)
	page.chooserList.SetStyle("List")

	page.chooserList.OnActive(func(active bool) {
		if active {
			page.chooserList.SetStyle("ListActive")
			page.setConfirmButton()
		} else {
			page.chooserList.SetStyle("List")
		}
	})

	page.chooserList.OnKeyPress(func(k term.Key) bool {
		if k == term.KeyEnter {
			if page.confirmBtn != nil {
				page.confirmBtn.ProcessEvent(clui.Event{Type: clui.EventKey, Key: k})
			}
			return true
		}

		return false
	})

	// Warning label
	page.labelWarning = clui.CreateLabel(contentFrame, 1, 2, "", Fixed)
	page.labelWarning.SetMultiline(true)
	page.labelWarning.SetBackColor(errorLabelBg)
	page.labelWarning.SetTextColor(errorLabelFg)

	if len(page.devs) < 1 {
		warning := "No media found for modification"
		log.Warning(warning)
		warning = fmt.Sprintf("Warning: %s", warning)
		page.labelWarning.SetTitle(warning)
	} else {
		page.labelWarning.SetTitle("")
	}

	// Add a Rescan media button
	rescanBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Rescan Media", Fixed)
	rescanBtn.OnClick(func(ev clui.Event) {
		var err error
		page.devs, err = storage.RescanBlockDevices(page.getModel().TargetMedias)
		if err != nil {
			page.Panic(err)
		}

		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}

		// Check if the active device is still present
		var found bool
		for _, bd := range page.devs {
			if bd.Serial == page.activeSerial {
				found = true
				page.activeDisk = bd
			}
		}
		if !found {
			page.activeSerial = ""
			page.activeDisk = nil
			page.getModel().TargetMedias = nil
		}

		page.GotoPage(TuiPageDiskUtil)
	})

	page.activated = page.backBtn

	page.setConfirmButton()

	return page, nil
}

func (page *DiskUtilPage) buildChooserList() {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	page.chooserList.Clear()

	found := false

	for _, target := range page.diskUtilTargets {
		page.chooserList.AddItem(fmtdiskUtilTarget(target))
		found = true
	}

	if found {
		page.chooserList.SelectItem(0)
	}
}

func (page *DiskUtilPage) buildMediaLists() error {
	page.labelWarning.SetTitle("")

	var err error
	page.devs, err = storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		page.Panic(err)
	}

	page.diskUtilTargets = storage.FindModifyInstallTargets(page.devs)

	page.buildChooserList()

	clui.RefreshScreen()

	return nil
}

func (page *DiskUtilPage) rundiskUtil(disk string) {

	stdMsg := "Could not launch " + diskUtil + ". Check A " + log.GetLogFileName()
	msg := ""

	// We need to save the current state for the relaunch of clr-installer
	tmpYaml, err := ioutil.TempFile("", "clr-installer-diskUtil-*.yaml")
	if err != nil {
		log.Warning("Could not make YAML tempfile: %v", err)
		msg = stdMsg
	}

	scrubbed := scrubModel(page.getModel())

	if saveErr := scrubbed.WriteFile(tmpYaml.Name()); saveErr != nil {
		log.Warning("Could not save config to %s %s", tmpYaml.Name(), msg)
		msg = stdMsg
	}

	tmpBash, err := ioutil.TempFile("", "clr-installer-diskUtil-*.sh")
	if err != nil {
		log.Warning("Could not make BASH tempfile: %v", err)
		msg = stdMsg
	}

	drive := filepath.Join("/dev", disk)
	exists, err := utils.FileExists(drive)
	if err != nil {
		log.Warning("Failed to check drive %s: %v", drive, err)
		msg = stdMsg
	} else if !exists {
		log.Warning("Request drive %s does not exist", drive)
		msg = stdMsg
	}

	var content bytes.Buffer
	_, _ = fmt.Fprintf(&content, "#!/bin/bash\n")
	_, _ = fmt.Fprintf(&content, "echo Switching to %s %s\n", diskUtil, drive)
	_, _ = fmt.Fprintf(&content, "sleep 2\n")
	_, _ = fmt.Fprintf(&content, "/usr/bin/%s %s\n", diskUtil, drive)
	_, _ = fmt.Fprintf(&content, "sleep 1\n")
	_, _ = fmt.Fprintf(&content, "echo Checking partitions with partprobe %s\n", drive)
	_, _ = fmt.Fprintf(&content, "/usr/bin/partprobe %s\n", drive)
	_, _ = fmt.Fprintf(&content, "sleep 1\n")
	_, _ = fmt.Fprintf(&content, "/bin/rm %s\n", tmpBash.Name())
	_, _ = fmt.Fprintf(&content, "echo Restarting Clear Linux OS Installer ...\n")
	_, _ = fmt.Fprintf(&content, "sleep 2\n")
	args := append(os.Args, "--config", tmpYaml.Name(), "--tui")
	allArgs := strings.Join(args, " ")
	_, err = fmt.Fprintf(&content, "exec %s", allArgs)
	if err != nil {
		log.Warning("Could not write BASH buffer: %v", err)
		msg = stdMsg
	}
	if _, err := tmpBash.Write(content.Bytes()); err != nil {
		log.Warning("Could not write BASH tempfile: %v", err)
		msg = stdMsg
	}
	_ = tmpBash.Close()
	_ = os.Chmod(tmpBash.Name(), 0700)

	if msg != "" {
		if _, err := CreateWarningDialogBox(msg); err != nil {
			log.Warning("Attempt to launch %s: warning dialog failed: %s", diskUtil, err)
		}
		return
	}

	// We will NEVER return from this function
	clui.Stop()
	clui.DeinitLibrary()
	term := os.Getenv("TERM")
	err = syscall.Exec("/bin/bash", []string{"/bin/bash", "-l", "-c", tmpBash.Name()}, []string{"TERM=" + term})
	if err != nil {
		log.Warning("Could not start disk utility: %v", err)
		msg = stdMsg
		if _, err := CreateWarningDialogBox(msg); err != nil {
			log.Warning("Attempt to launch %s: warning dialog failed: %s", diskUtil, err)
		}
		return
	}
}

func scrubModel(md *model.SystemInstall) *model.SystemInstall {
	// Sanitized the model to remove meida
	var cleanModel model.SystemInstall
	// Marshal current into bytes
	confBytes, bytesErr := yaml.Marshal(md)
	if bytesErr != nil {
		log.Error("Failed to generate a copy of YAML data for %s (%v)", diskUtil, bytesErr)
		return nil
	}
	// Unmarshal into a copy
	if yamlErr := yaml.Unmarshal(confBytes, &cleanModel); yamlErr != nil {
		log.Error("Failed to duplicate YAML data for %s (%v)", diskUtil, bytesErr)
		return nil
	}
	// Sanitize the config data to remove any potential
	// Remove the target media
	cleanModel.TargetMedias = nil

	return &cleanModel
}

func fmtdiskUtilTarget(target storage.InstallTarget) string {
	portion := "[Modify Disk]"

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	return fmt.Sprintf("%-34s  %10s  %-12s  %8s", target.Friendly, target.Name, portion, size)
}
