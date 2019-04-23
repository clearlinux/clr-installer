// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"fmt"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/utils"
)

// DiskConfig is a simple page to help with DiskConfig settings
type DiskConfig struct {
	devs               []*storage.BlockDevice
	safeTargets        []storage.InstallTarget
	destructiveTargets []storage.InstallTarget
	activeDisk         *storage.BlockDevice
	activeSerial       string
	controller         Controller
	model              *model.SystemInstall
	box                *gtk.Box
	scroll             *gtk.ScrolledWindow
	scrollBox          *gtk.Box
	mediaGrid          *gtk.Grid
	safeButton         *gtk.RadioButton
	destructiveButton  *gtk.RadioButton
	chooserCombo       *gtk.ComboBox
	encryptCheck       *gtk.CheckButton
	errorMessage       *gtk.Label
	rescanButton       *gtk.Button
}

func newListStoreMedia() (*gtk.ListStore, error) {
	store, err := gtk.ListStoreNew(glib.TYPE_OBJECT, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING)
	return store, err
}

// addListStoreMediaRow adds new row to the ListStore widget for the given media
func addListStoreMediaRow(store *gtk.ListStore, installMedia storage.InstallTarget) error {

	// Create icon image
	mediaType := "drive-harddisk-system"
	if installMedia.Removable {
		mediaType = "media-removable"
	}
	mediaType = mediaType + "-symbolic"
	image, err := gtk.ImageNewFromIconName(mediaType, gtk.ICON_SIZE_DIALOG)
	if err != nil {
		log.Warning("gtk.ImageNewFromIconName failed for icon %q", mediaType)
		return err
	}

	iter := store.Append()

	err = store.SetValue(iter, 0, image.GetPixbuf())
	if err != nil {
		log.Warning("SetValue store failed for icon %q", mediaType)
		return err
	}

	// Name string
	nameString := installMedia.Friendly

	err = store.SetValue(iter, 1, nameString)
	if err != nil {
		log.Warning("SetValue store failed for name string: %q", nameString)
		return err
	}

	// Portion string
	portionString := storage.FormatInstallPortion(installMedia)
	err = store.SetValue(iter, 2, portionString)
	if err != nil {
		log.Warning("SetValue store failed for portion string: %q", portionString)
		return err
	}

	// Size string
	sizeString, _ := storage.HumanReadableSizeWithPrecision(installMedia.FreeEnd-installMedia.FreeStart, 1)

	err = store.SetValue(iter, 3, sizeString)
	if err != nil {
		log.Warning("SetValue store failed for size string: %q", sizeString)
		return err
	}

	return nil
}

// NewDiskConfigPage returns a new DiskConfigPage
func NewDiskConfigPage(controller Controller, model *model.SystemInstall) (Page, error) {
	disk := &DiskConfig{
		controller: controller,
		model:      model,
	}
	var err error

	disk.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	disk.box.SetBorderWidth(8)

	// Build storage for scrollBox
	disk.scroll, err = gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	disk.box.PackStart(disk.scroll, true, true, 0)
	disk.scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	// Build scrollBox
	disk.scrollBox, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 20)
	if err != nil {
		return nil, err
	}

	disk.scroll.Add(disk.scrollBox)

	// Media Grid
	disk.mediaGrid, err = gtk.GridNew()
	if err != nil {
		return nil, err
	}

	// Build the Safe Install Section
	disk.safeButton, err = gtk.RadioButtonNewFromWidget(nil)
	if err != nil {
		return nil, err
	}

	safeBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	safeBox.PackStart(disk.safeButton, false, false, 10)
	if _, err := disk.safeButton.Connect("toggled", func() {
		// Enable/Disable the Combo Choose Box based on the radio button
		//disk.safeCombo.SetSensitive(disk.safeButton.GetActive())
		if err := disk.populateComboBoxes(); err != nil {
			log.Warning("Problem populating possible disk selections")
		}
	}); err != nil {
		return nil, err
	}

	safeHortzBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	safeBox.PackStart(safeHortzBox, true, true, 0)
	safeTitle := utils.Locale.Get("Safe Installation")
	safeDescription := utils.Locale.Get("Install on an unallocated disk or alongside existing partitions.")
	text := fmt.Sprintf("<big>%s</big>\n", safeTitle)
	text = text + safeDescription
	safeLabel, err := gtk.LabelNew(text)
	if err != nil {
		return nil, err
	}
	safeLabel.SetXAlign(0.0)
	safeLabel.SetHAlign(gtk.ALIGN_START)
	safeLabel.SetUseMarkup(true)
	safeHortzBox.PackStart(safeLabel, false, false, 0)

	log.Debug("Before safeBox ShowAll")
	safeBox.ShowAll()
	disk.mediaGrid.Attach(safeBox, 0, 0, 1, 1)

	// Build the Destructive Install Section
	log.Debug("Before disk.destructiveButton")
	disk.destructiveButton, err = gtk.RadioButtonNewFromWidget(disk.safeButton)
	if err != nil {
		return nil, err
	}

	destructiveBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	destructiveBox.PackStart(disk.destructiveButton, false, false, 10)
	if _, err := disk.destructiveButton.Connect("toggled", func() {
		// Enable/Disable the Combo Choose Box based on the radio button
		//disk.destructiveCombo.SetSensitive(disk.destructiveButton.GetActive())
		if err := disk.populateComboBoxes(); err != nil {
			log.Warning("Problem populating possible disk selections")
		}
	}); err != nil {
		return nil, err
	}

	destructiveHortzBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	destructiveBox.PackStart(destructiveHortzBox, true, true, 0)
	destructiveTitle := utils.Locale.Get("Destructive Installation")
	destructiveDescription := utils.Locale.Get("Erase all data on selected media and install Clear Linux* OS.")
	text = fmt.Sprintf("<big><b><span foreground=\"red\">%s</span></b></big>\n", destructiveTitle)
	text = text + destructiveDescription
	destructiveLabel, err := gtk.LabelNew(text)
	if err != nil {
		return nil, err
	}
	destructiveLabel.SetXAlign(0.0)
	destructiveLabel.SetHAlign(gtk.ALIGN_START)
	destructiveLabel.SetUseMarkup(true)
	destructiveHortzBox.PackStart(destructiveLabel, false, false, 0)

	destructiveBox.ShowAll()
	disk.mediaGrid.Attach(destructiveBox, 0, 1, 1, 1)

	log.Debug("Before making ComboBox")
	disk.chooserCombo, err = gtk.ComboBoxNew()
	if err != nil {
		log.Warning("Failed to make disk.chooserCombo")
		return nil, err
	}

	// Add the renderers to the ComboBox
	mediaRenderer, _ := gtk.CellRendererPixbufNew()
	disk.chooserCombo.PackStart(mediaRenderer, true)
	disk.chooserCombo.AddAttribute(mediaRenderer, "pixbuf", 0)

	nameRenderer, _ := gtk.CellRendererTextNew()
	disk.chooserCombo.PackStart(nameRenderer, true)
	disk.chooserCombo.AddAttribute(nameRenderer, "text", 1)

	portionRenderer, _ := gtk.CellRendererTextNew()
	disk.chooserCombo.PackStart(portionRenderer, true)
	disk.chooserCombo.AddAttribute(portionRenderer, "text", 2)

	sizeRenderer, _ := gtk.CellRendererTextNew()
	disk.chooserCombo.PackStart(sizeRenderer, true)
	disk.chooserCombo.AddAttribute(sizeRenderer, "text", 3)

	disk.mediaGrid.Attach(disk.chooserCombo, 1, 0, 1, 2)

	disk.mediaGrid.SetRowSpacing(10)
	disk.mediaGrid.SetColumnSpacing(10)
	disk.mediaGrid.SetColumnHomogeneous(true)
	disk.scrollBox.Add(disk.mediaGrid)

	separator, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	if err != nil {
		return nil, err
	}
	separator.ShowAll()
	disk.scrollBox.Add(separator)

	// Error Message Label
	disk.errorMessage, err = gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	disk.errorMessage.SetXAlign(0.0)
	disk.errorMessage.SetHAlign(gtk.ALIGN_START)
	disk.errorMessage.SetUseMarkup(true)
	disk.scrollBox.Add(disk.errorMessage)

	// Encryption button
	disk.encryptCheck, err = gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}
	/*
		disk.encryptCheck.SetLabel("   " + utils.Locale.Get("Enable Encryption"))
		sc, err := disk.encryptCheck.GetStyleContext()
		if err != nil {
			log.Warning("Error getting style context: ", err) // Just log trivial error
		} else {
			sc.AddClass("label-entry")
		}
		disk.encryptCheck.SetMarginStart(CommonSetting + StartEndMargin)
		disk.encryptCheck.SetMarginEnd(StartEndMargin)
		disk.scrollBox.Add(disk.encryptCheck)
	*/

	// Buttons
	disk.rescanButton, err = setButton(utils.Locale.Get("RESCAN MEDIA"), "button-page")
	if err != nil {
		return nil, err
	}

	if _, err = disk.rescanButton.Connect("clicked", func() {
		log.Debug("rescan")
		_ = disk.scanMediaDevices()
		// Check if the active device is still present
		var found bool
		for _, bd := range disk.devs {
			if bd.Serial == disk.activeSerial {
				found = true
				disk.activeDisk = bd
			}
		}
		if !found {
			disk.activeSerial = ""
			disk.activeDisk = nil
			disk.model.TargetMedias = nil
		}

		if err := disk.populateComboBoxes(); err != nil {
			log.Warning("Problem populating possible disk selections")
		}
	}); err != nil {
		return nil, err
	}

	rescanBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	rescanBox.PackStart(disk.rescanButton, false, false, 10)

	rescanHortzBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	rescanBox.PackStart(rescanHortzBox, true, true, 0)
	text = fmt.Sprintf("<big>" + utils.Locale.Get("Rescan Media") + "</big>\n")
	text = text + utils.Locale.Get("Rescan for changes to hot swappable media.")
	rescanLabel, err := gtk.LabelNew(text)
	if err != nil {
		return nil, err
	}
	rescanLabel.SetXAlign(0.0)
	rescanLabel.SetHAlign(gtk.ALIGN_START)
	rescanLabel.SetUseMarkup(true)
	rescanHortzBox.PackStart(rescanLabel, false, false, 0)

	rescanBox.ShowAll()
	disk.scrollBox.Add(rescanBox)

	disk.box.ShowAll()

	_ = disk.scanMediaDevices()

	return disk, nil
}

// This is time intensive, mitigate calls
func (disk *DiskConfig) scanMediaDevices() error {
	var err error

	disk.devs, err = storage.RescanBlockDevices(disk.model.TargetMedias)
	if err != nil {
		return err
	}

	return nil
}

// populateComboBoxes populates the scrollBox with usable widget things
func (disk *DiskConfig) populateComboBoxes() error {
	safeStore, err := newListStoreMedia()
	if err != nil {
		log.Warning("ListStoreNew safeStore failed")
		return err
	}
	destructiveStore, err := newListStoreMedia()
	if err != nil {
		log.Warning("ListStoreNew destructiveStore failed")
		return err
	}

	if len(disk.devs) < 1 {
		warning := utils.Locale.Get("No media found for installation")
		log.Warning(warning)
		warning = fmt.Sprintf("<big><b><span foreground=\"red\">" + utils.Locale.Get("Warning: %s", warning) + "</span></b></big>")
		disk.errorMessage.SetMarkup(warning)
		return nil
	}

	disk.safeTargets = storage.FindSafeInstallTargets(storage.MinimumDesktopInstallSize, disk.devs)
	disk.destructiveTargets = storage.FindAllInstallTargets(disk.devs)

	if disk.safeButton.GetActive() {
		if len(disk.safeTargets) > 0 {
			for _, target := range disk.safeTargets {
				log.Debug("Adding safe install target %s", target.Name)
				err := addListStoreMediaRow(safeStore, target)
				if err != nil {
					log.Warning("SetValue safeStore")
					return err
				}
			}
			disk.chooserCombo.SetModel(safeStore)
			disk.chooserCombo.SetActive(0)
		}
	} else if disk.destructiveButton.GetActive() {
		for _, target := range disk.destructiveTargets {
			log.Debug("Adding destructive install target %s", target.Name)
			err := addListStoreMediaRow(destructiveStore, target)
			if err != nil {
				log.Warning("SetValue destructiveStore")
				return err
			}
		}
		disk.chooserCombo.SetModel(destructiveStore)
		disk.chooserCombo.SetActive(0)
	}

	return nil
}

// IsRequired will return true as we always need a DiskConfig
func (disk *DiskConfig) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (disk *DiskConfig) IsDone() bool {
	return disk.model.TargetMedias != nil
}

// GetID returns the ID for this page
func (disk *DiskConfig) GetID() int {
	return PageIDDiskConfig
}

// GetIcon returns the icon for this page
func (disk *DiskConfig) GetIcon() string {
	return "drive-harddisk-system"
}

// GetRootWidget returns the root embeddable widget for this page
func (disk *DiskConfig) GetRootWidget() gtk.IWidget {
	return disk.box
}

// GetSummary will return the summary for this page
func (disk *DiskConfig) GetSummary() string {
	return utils.Locale.Get("Select Installation Media")
}

// GetTitle will return the title for this page
func (disk *DiskConfig) GetTitle() string {
	return disk.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (disk *DiskConfig) StoreChanges() {
	var installBlockDevice *storage.BlockDevice

	if disk.safeButton.GetActive() {
		log.Debug("Safe Install chooserCombo selected %v", disk.chooserCombo.GetActive())
		disk.model.InstallSelected = disk.safeTargets[disk.chooserCombo.GetActive()]
		log.Debug("Safe Install Target %v", disk.model.InstallSelected)
	} else if disk.destructiveButton.GetActive() {
		log.Debug("Destructive Install chooserCombo selected %v", disk.chooserCombo.GetActive())
		disk.model.InstallSelected = disk.destructiveTargets[disk.chooserCombo.GetActive()]
		log.Debug("Destructive Install Target %v", disk.model.InstallSelected)
	} else {
		log.Warning("Failed to find and save the selected installation media")
	}

	bds, err := storage.ListAvailableBlockDevices(disk.model.TargetMedias)
	if err != nil {
		log.Error("Failed to find storage media for install during save: %s", err)
	}

	for _, curr := range bds {
		if curr.Name == disk.model.InstallSelected.Name {
			installBlockDevice = curr.Clone()
			// Using the whole disk
			if disk.model.InstallSelected.WholeDisk {
				storage.NewStandardPartitions(installBlockDevice)
			} else {
				// Partial Disk, Add our partitions
				size := disk.model.InstallSelected.FreeEnd - disk.model.InstallSelected.FreeStart
				size = size - storage.AddBootStandardPartition(installBlockDevice)
				if !installBlockDevice.DeviceHasSwap() {
					size = size - storage.AddSwapStandardPartition(installBlockDevice)
				}
				storage.AddRootStandardPartition(installBlockDevice, size)
			}
			disk.model.TargetMedias = nil
			// Give the active disk to the model
			disk.model.AddTargetMedia(installBlockDevice)
			break
		}
	}

	/*
		if disk.encryptCheck.GetActive() {
			for _, child := range installBlockDevice.Children {
				if child.MountPoint == "/" {
					child.Type = storage.BlockDeviceTypeCrypt
				}
			}
		}
	*/
}

// ResetChanges will reset this page to match the model
func (disk *DiskConfig) ResetChanges() {
	disk.activeDisk = nil
	disk.controller.SetButtonState(ButtonConfirm, true)

	disk.chooserCombo.SetSensitive(false)

	if err := disk.populateComboBoxes(); err != nil {
		log.Warning("Problem populating possible disk selections")
	}

	// Choose the most appropriate button
	if len(disk.safeTargets) > 0 {
		disk.safeButton.SetActive(true)
		disk.chooserCombo.SetSensitive(true)
	} else if len(disk.destructiveTargets) > 0 {
		disk.destructiveButton.SetActive(true)
		disk.chooserCombo.SetSensitive(true)
	} else {
		//disk.rescanButton.SetActive(true)
		//TODO: Make this button have focus/default
		log.Debug("Need to make the rescan button default")
	}

	// TODO: Match list to target medias. But we have an ugly
	// list of root target medias and you can only select one
	// right now as our manual partitioning is missing.
	if disk.model.TargetMedias == nil {
		return
	}
}

// GetConfiguredValue returns our current config
func (disk *DiskConfig) GetConfiguredValue() string {
	tm := disk.model.TargetMedias
	if len(tm) == 0 {
		return utils.Locale.Get("No Media Selected")
	}

	target := disk.model.InstallSelected
	portion := storage.FormatInstallPortion(target)

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	encrypted := ""
	for _, bd := range tm {
		for _, ch := range bd.Children {
			if ch.Type == storage.BlockDeviceTypeCrypt {
				encrypted = " " + utils.Locale.Get("Encryption")
			}
		}
	}

	return fmt.Sprintf("%s (%s) %s%s %s", target.Friendly, target.Name, portion, encrypted, size)
}
