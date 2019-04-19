// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/storage"
)

const (
	mediaConfigMenuTitle = `Configure Installation Media`
	mediaConfigTitle     = `Select Installation Media`
)

// MediaConfigPage is the Page implementation for the disk partitioning menu page
type MediaConfigPage struct {
	BasePage

	safeRadio             *clui.Radio
	destructiveRadio      *clui.Radio
	group                 *clui.RadioGroup
	isSafeSelected        bool
	isDestructiveSelected bool

	chooserList   *clui.ListBox
	listBackColor term.Attribute
	listTextColor term.Attribute

	safeTargets        []storage.InstallTarget
	destructiveTargets []storage.InstallTarget

	labelWarning     *clui.Label
	labelDestructive *clui.Label

	encryptCheck *clui.CheckBox

	devs         []*storage.BlockDevice
	activeDisk   *storage.BlockDevice
	activeSerial string
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *MediaConfigPage) GetConfiguredValue() string {
	tm := page.getModel().TargetMedias
	if len(tm) == 0 {
		return "No -media- selected"
	}

	target := page.getModel().InstallSelected
	portion := fmtInstallPortion(target)

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	encrypted := ""
	for _, bd := range tm {
		for _, ch := range bd.Children {
			if ch.Type == storage.BlockDeviceTypeCrypt {
				encrypted = " Encryption"
			}
		}
	}

	return fmt.Sprintf("%s (%s) %s%s %s", target.Friendly, target.Name, portion, encrypted, size)
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *MediaConfigPage) GetConfigDefinition() int {
	tm := page.getModel().TargetMedias

	if tm == nil {
		return ConfigNotDefined
	}

	for _, bd := range tm {
		if !bd.IsUserDefined() {
			return ConfigDefinedByConfig
		}

		for _, ch := range bd.Children {
			if !ch.IsUserDefined() {
				return ConfigDefinedByConfig
			}
		}
	}

	return ConfigDefinedByUser
}

// SetDone sets the configured disk into the model and sets the page as done
func (page *MediaConfigPage) SetDone(done bool) bool {
	var installBlockDevice *storage.BlockDevice

	if page.safeRadio.Selected() {
		page.getModel().InstallSelected = page.safeTargets[page.chooserList.SelectedItem()]
		log.Debug("Safe Install Target %v", page.getModel().InstallSelected)
	} else if page.destructiveRadio.Selected() {
		page.getModel().InstallSelected = page.destructiveTargets[page.chooserList.SelectedItem()]
		log.Debug("Destructive Install Target %v", page.getModel().InstallSelected)
	} else {
		log.Warning("Failed to find and save the selected installation media")
	}

	bds, err := storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		log.Error("Failed to find storage media for install during save: %s", err)
	}

	for _, curr := range bds {
		if curr.Name == page.getModel().InstallSelected.Name {
			installBlockDevice = curr.Clone()
			// Using the whole disk
			if page.getModel().InstallSelected.WholeDisk {
				storage.NewStandardPartitions(installBlockDevice)
			} else {
				// Partial Disk, Add our partitions
				size := page.getModel().InstallSelected.FreeEnd - page.getModel().InstallSelected.FreeStart
				size = size - storage.AddBootStandardPartition(installBlockDevice)
				if !installBlockDevice.DeviceHasSwap() {
					size = size - storage.AddSwapStandardPartition(installBlockDevice)
				}
				storage.AddRootStandardPartition(installBlockDevice, size)
			}
			page.getModel().TargetMedias = nil
			page.getModel().AddTargetMedia(installBlockDevice)
			break
		}
	}

	if page.encryptCheck.State() != 0 {
		for _, child := range installBlockDevice.Children {
			if child.MountPoint == "/" {
				child.Type = storage.BlockDeviceTypeCrypt
			}
		}
	}

	// TODO start using new API page.GotoPage() when finished merging
	// disk pages
	page.tui.gotoPage(TuiPageMenu, page)

	return false
}

// Activate updates the UI elements with the most current list of block devices
func (page *MediaConfigPage) Activate() {

	page.confirmBtn.SetEnabled(false)

	if len(page.safeTargets) == 0 && len(page.destructiveTargets) == 0 {
		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}
	}

	if page.isSafeSelected {
		page.activated = page.safeRadio
	} else if page.isDestructiveSelected {
		page.activated = page.destructiveRadio
	} else {
		page.activated = page.cancelBtn
	}

	clui.RefreshScreen()
}

func (page *MediaConfigPage) setConfirmButton() {
	if page.labelWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

// The disk page gives the user the option so select how to set the storage device,
// if to manually configure it or a guided standard partition schema
func newMediaConfigPage(tui *Tui) (Page, error) {
	page := &MediaConfigPage{
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}
	page.setupMenu(tui, TuiPageMediaConfig, mediaConfigMenuTitle, CancelButton|ConfirmButton, TuiPageMenu)

	// Top label for the page
	lbl := clui.CreateLabel(page.content, 2, 1, mediaConfigTitle, clui.Fixed)
	lbl.SetPaddings(0, 1)

	contentFrame := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	contentFrame.SetPack(clui.Vertical)

	// Installation Type Radio button selection
	radioButtonFrame := clui.CreateFrame(contentFrame, AutoSize, AutoSize, BorderNone, Fixed)
	radioButtonFrame.SetPack(clui.Vertical)
	radioButtonFrame.SetPaddings(2, 1)

	page.group = clui.CreateRadioGroup()
	radioLabel := fmt.Sprintf("Safe Installation")
	page.safeRadio = clui.CreateRadio(radioButtonFrame, 50, radioLabel, AutoSize)
	page.safeRadio.SetStyle("Media")
	page.safeRadio.SetPack(clui.Horizontal)
	page.group.AddItem(page.safeRadio)
	page.safeRadio.OnChange(func(active bool) {
		if active {
			page.isDestructiveSelected = false
		}
		// Disable the Confirm Button if we toggled
		if !page.isSafeSelected && active {
			page.confirmBtn.SetEnabled(false)
			page.isSafeSelected = true

			page.buildChooserList()
		}

		if active {
			if len(page.safeTargets) < 1 {
				if active {
					warning := "No media or space available for installation"
					log.Warning(warning)
					warning = fmt.Sprintf("Warning: %s", warning)
					page.labelWarning.SetTitle(warning)
				}
			} else {
				page.labelDestructive.SetTitle("")
			}
		}
	})
	// Description of Safe
	descLabel := clui.CreateLabel(radioButtonFrame, 2, 2,
		"Install on an unallocated disk or alongside existing partitions.", Fixed)
	descLabel.SetMultiline(true)

	radioLabel = fmt.Sprintf("Destructive Installation")
	page.destructiveRadio = clui.CreateRadio(radioButtonFrame, 50, radioLabel, AutoSize)
	page.destructiveRadio.SetStyle("Media")
	page.destructiveRadio.SetPack(clui.Horizontal)
	page.group.AddItem(page.destructiveRadio)
	page.destructiveRadio.OnChange(func(active bool) {
		if active {
			page.isSafeSelected = false
		}
		// Disable the Confirm Button if we toggled
		if !page.isDestructiveSelected && active {
			page.confirmBtn.SetEnabled(false)
			page.isDestructiveSelected = true

			page.buildChooserList()
		}

		if active {
			if len(page.devs) < 1 {
				warning := "No media found for installation"
				log.Warning(warning)
				warning = fmt.Sprintf("Warning: %s", warning)
				page.labelWarning.SetTitle(warning)
			} else {
				page.labelDestructive.SetTitle(descructiveWarning)
			}
		}
	})
	// Description of Destructive
	clui.CreateLabel(radioButtonFrame, 2, 1, "Erase all data on selected media and install Clear Linux* OS.", Fixed)

	listFrame := clui.CreateFrame(contentFrame, 60, 3, BorderNone, Fixed)
	listFrame.SetPack(clui.Vertical)

	page.chooserList = clui.CreateListBox(listFrame, 60, 3, Fixed)
	page.chooserList.SetAlign(AlignLeft)
	page.chooserList.SetStyle("List")
	page.listBackColor = page.chooserList.BackColor()
	page.listTextColor = page.chooserList.TextColor()

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

	// Destructive label
	page.labelDestructive = clui.CreateLabel(contentFrame, 1, 2, "", Fixed)
	page.labelDestructive.SetBackColor(errorLabelBg)
	page.labelDestructive.SetTextColor(errorLabelFg)

	// Encryption Checkbox
	page.encryptCheck = clui.CreateCheckBox(contentFrame, AutoSize, "Enable Encryption", AutoSize)
	page.encryptCheck.OnChange(func(state int) {
		if state != 0 {
			if dialog, err := CreateEncryptPassphraseDialogBox(page.getModel()); err == nil {
				dialog.OnClose(func() {
					if !dialog.Confirmed {
						page.encryptCheck.SetState(0)
					}
				})
			}
		}
	})

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

		page.GotoPage(TuiPageMediaConfig)
	})

	// Add an Advanced Configuration  button
	advancedCfgBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Advanced Configuration", Fixed)
	advancedCfgBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageDiskConfig)
	})

	page.activated = page.backBtn

	page.setConfirmButton()

	return page, nil
}

func (page *MediaConfigPage) buildChooserList() {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	page.chooserList.Clear()
	page.chooserList.SetBackColor(page.listBackColor)
	page.chooserList.SetTextColor(page.listTextColor)

	found := false

	if page.isSafeSelected {
		for _, target := range page.safeTargets {
			page.chooserList.AddItem(fmtInstallTarget(target))
			found = true
		}
	} else if page.isDestructiveSelected {
		for _, target := range page.destructiveTargets {
			page.chooserList.AddItem(fmtInstallTarget(target))
			found = true
		}
		page.chooserList.SetBackColor(errorLabelBg)
		page.chooserList.SetTextColor(errorLabelFg)
	} else {
		log.Warning("buildChooserList: unknown radio button state")
	}

	if found {
		page.chooserList.SelectItem(0)
	}
}

func (page *MediaConfigPage) buildMediaLists() error {
	page.labelWarning.SetTitle("")
	page.labelDestructive.SetTitle("")

	var err error
	page.devs, err = storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		page.Panic(err)
	}

	page.safeTargets = storage.FindSafeInstallTargets(storage.MinimumServerInstallSize, page.devs)
	page.destructiveTargets = storage.FindAllInstallTargets(page.devs)

	if len(page.safeTargets) > 0 {
		if !page.group.SelectItem(page.safeRadio) {
			log.Warning("Could not select the safe install radio button")
		}
		page.isSafeSelected = true
	} else {
		if !page.group.SelectItem(page.destructiveRadio) {
			log.Warning("Could not select the destructive install radio button")
		}
		page.isDestructiveSelected = true
	}

	page.buildChooserList()

	clui.RefreshScreen()

	return nil
}

func fmtInstallPortion(target storage.InstallTarget) string {
	portion := "[Partial]"
	if target.WholeDisk {
		portion = "[Entire Disk]"
	}
	if target.EraseDisk {
		portion = "[Erase Disk]"
	}
	if target.Advanced {
		portion = "[Advanced]"
	}

	return portion
}

func fmtInstallTarget(target storage.InstallTarget) string {
	portion := fmtInstallPortion(target)

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	return fmt.Sprintf("%-32s  %10s  %-14s  %8s", target.Friendly, target.Name, portion, size)
}
