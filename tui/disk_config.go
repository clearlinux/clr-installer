// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/storage"
)

// SelectedBlockDevice holds the shared date between the Disk Configuration page
// and the partition configuration page
type SelectedBlockDevice struct {
	bd            *storage.BlockDevice
	part          *storage.BlockDevice
	addMode       bool
	wholeDisk     bool
	dataLoss      bool
	freePartition *storage.PartedPartition
}

const (
	diskConfigTitle = `Advanced Configuration`
)
const (
	diskColumnDisk = iota
	diskColumnPartition
	diskColumnFsType
	diskColumnMount
	diskColumnSize
	diskColumnCount
)

var (
	diskColumns []columnInfo
)

// Clone makes a copy of the SelectedBlockDevice
func (sbd *SelectedBlockDevice) Clone() *SelectedBlockDevice {
	clone := &SelectedBlockDevice{
		addMode:   sbd.addMode,
		wholeDisk: sbd.wholeDisk,
		dataLoss:  sbd.dataLoss,
	}

	if sbd.bd != nil {
		clone.bd = sbd.bd.Clone()
	}

	if sbd.part != nil {
		clone.part = sbd.part.Clone()
	}

	if sbd.freePartition != nil {
		clone.freePartition = sbd.freePartition.Clone()
	}

	return clone
}

func init() {
	diskColumns = make([]columnInfo, diskColumnCount)

	diskColumns[diskColumnDisk].title = "Disk"
	diskColumns[diskColumnDisk].minWidth = 16

	diskColumns[diskColumnPartition].title = "Partition"
	diskColumns[diskColumnPartition].minWidth = columnWidthDefault

	diskColumns[diskColumnFsType].title = "File System"
	diskColumns[diskColumnFsType].minWidth = columnWidthDefault

	diskColumns[diskColumnMount].title = "Mount Point"
	diskColumns[diskColumnMount].minWidth = -1 // This column get all free space

	diskColumns[diskColumnSize].title = "Size"
	diskColumns[diskColumnSize].minWidth = 8
	diskColumns[diskColumnSize].rightJustify = true
}

// DiskConfigPage is the Page implementation for the disk partitioning menu page
type DiskConfigPage struct {
	BasePage
	scrollingFrame *clui.Frame // content scrolling frame

	columnFormat string

	blockDevices []*storage.BlockDevice

	rowFrames    []*clui.Frame
	activeRow    *clui.Frame
	activeDisk   *SimpleButton
	activeSerial string
	diskOpen     bool

	lastPartButtons []*SimpleButton
	lastDiskButton  *SimpleButton
	lastAutoButton  *SimpleButton
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *DiskConfigPage) GetConfiguredValue() string {
	if len(page.getModel().TargetMedias) == 0 {
		return "No -media- configured"
	}

	res := []string{}

	for _, curr := range page.getModel().TargetMedias {
		for _, part := range curr.Children {
			tks := []string{part.Name}

			if part.Type == storage.BlockDeviceTypeCrypt {
				tks = append(tks, part.FsType+"*")
			} else {
				tks = append(tks, part.FsType)
			}

			if part.MountPoint != "" {
				tks = append(tks, part.MountPoint)
			}

			res = append(res, strings.Join(tks, ":"))
		}
	}

	return strings.Join(res, ", ")
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *DiskConfigPage) GetConfigDefinition() int {
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
func (page *DiskConfigPage) SetDone(done bool) bool {
	if sel, ok := page.data.(*SelectedBlockDevice); ok {
		var selected *storage.BlockDevice

		bds, err := storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
		if err != nil {
			page.Panic(err)
		}

		for _, curr := range bds {
			if !curr.Equals(sel.bd) {
				continue
			}

			selected = curr
			break
		}

		installBlockDevice := selected.Clone()
		page.getModel().TargetMedias = nil
		page.getModel().AddTargetMedia(installBlockDevice)

		page.getModel().InstallSelected = storage.InstallTarget{
			Name: installBlockDevice.Name, Friendly: installBlockDevice.Model,
			WholeDisk: sel.wholeDisk, Removable: installBlockDevice.RemovableDevice,
			DataLoss: sel.dataLoss, Advanced: true, FreeStart: 0, FreeEnd: installBlockDevice.Size}
	}

	// TODO start using new API page.GotoPage() when finished merging
	// disk pages
	page.tui.gotoPage(TuiPageMenu, page)

	return false
}

// Activate updates the UI elements with the most current list of block devices
func (page *DiskConfigPage) Activate() {

	page.redrawRows()
	page.confirmBtn.SetEnabled(false)

	if page.activeDisk != nil {
		page.activated = page.activeDisk
		page.scrollToRow(page.activeRow)
		if page.lastAutoButton != nil {
			page.lastAutoButton.SetEnabled(true)
		}

		for _, curr := range page.blockDevices {
			if status := curr.GetConfiguredStatus(); status != storage.ConfiguredNone {
				// A disk beside the active is configured
				if page.activeSerial != curr.Serial && page.lastAutoButton != nil {
					// Disable Auto Partitioning
					page.lastAutoButton.SetEnabled(false)
				}

				if status == storage.ConfiguredEntire {
					page.confirmBtn.SetEnabled(true)
					page.activated = page.confirmBtn
					break
				}
			}
		}

		// If we have an active disk, but not done configuring, then
		// put the disk in the selective/active state
		if page.activated == page.activeDisk {
			if page.diskOpen {
				x, y := page.activeDisk.Pos()
				page.activeDisk.ProcessEvent(clui.Event{Type: clui.EventMouse, Key: term.MouseLeft, X: x, Y: y})
				page.activeDisk.ProcessEvent(clui.Event{Type: clui.EventMouse, Key: term.MouseRelease, X: x, Y: y})
			}
		}
	}

	clui.RefreshScreen()
}

func (page *DiskConfigPage) scrollToRow(rowFrame *clui.Frame) {
	_, cy, _, ch := page.scrollingFrame.Clipper()
	vx, vy := page.scrollingFrame.Pos()

	_, ry := rowFrame.Pos()
	_, rh := rowFrame.Size()

	if ry+rh > cy+ch {
		diff := (cy + ch) - (ry + rh)
		ty := vy + diff
		page.scrollingFrame.ScrollTo(vx, ty)
	} else if ry < cy {
		page.scrollingFrame.ScrollTo(vx, cy)
	}
}

func (page *DiskConfigPage) redrawRows() {
	for _, curr := range page.rowFrames {
		curr.Destroy()
	}
	page.rowFrames = []*clui.Frame{}
	// Clear last selected row
	page.lastDiskButton = nil
	page.lastAutoButton = nil
	page.lastPartButtons = nil

	if len(page.blockDevices) > 0 {
		for _, bd := range page.blockDevices {
			if err := page.addDiskRow(bd); err != nil {
				page.Panic(err)
			}
		}
	} else {
		rowFrame := clui.CreateFrame(page.scrollingFrame, 1, AutoSize, clui.BorderNone, clui.Fixed)
		rowFrame.SetPack(clui.Vertical)
		_ = clui.CreateLabel(rowFrame, 2, 1, "*** No Usable Media Detected ***", clui.Fixed)
		page.rowFrames = append(page.rowFrames, rowFrame)
		page.scrollToRow(rowFrame)
		page.activated = page.cancelBtn
	}

	clui.RefreshScreen()
}

// The disk page gives the user the option so select how to set the storage device,
// if to manually configure it or a guided standard partition schema
func newDiskConfigPage(tui *Tui) (Page, error) {
	page := &DiskConfigPage{}

	page.setup(tui, TuiPageDiskConfig, CancelButton|ConfirmButton, TuiPageMenu)

	cWidth, cHeight := page.content.Size()
	// Calculate the Scrollable frame area
	sWidth := cWidth - (2 * 2) // Buffer of 2 characters from each side
	sHeight := cHeight + 1     // Add back the blank line from content to menu buttons

	// Top label for the page
	lbl := clui.CreateLabel(page.content, 2, 2, diskConfigTitle, clui.Fixed)
	lbl.SetPaddings(0, 2)
	_, lHeight := lbl.Size()
	sHeight -= lHeight // Remove the label from total height

	remainingColumns := sWidth - 2
	allFree := -1
	for i, info := range diskColumns {
		// Should this column get all extra space?
		if info.minWidth == -1 {
			if allFree == -1 {
				allFree = i
				continue // Do not format this column
			} else {
				log.Warning("More than one disk partition column set for all free space: %s", info.title)
				info.minWidth = columnWidthDefault
			}
		}

		l, format := getColumnFormat(info)
		diskColumns[i].format = format
		diskColumns[i].width = l
		remainingColumns -= l
	}

	// remove the column spacers
	remainingColumns -= ((len(diskColumns) - 1) * len(columnSpacer))

	// If we had a column which get the remaining space
	if allFree != -1 {
		diskColumns[allFree].minWidth = remainingColumns
		diskColumns[allFree].width = remainingColumns
		_, diskColumns[allFree].format = getColumnFormat(diskColumns[allFree])
	}

	// Build the Header Title and Entire row format string
	titles := []interface{}{""} // need to use an interface for Sprintf
	formats := []string{}
	dividers := []interface{}{""} // need to use an interface for Sprintf
	for _, info := range diskColumns {
		titles = append(titles, info.title)
		formats = append(formats, info.format)
		dividers = append(dividers, strings.Repeat(rowDividor, info.width))
	}
	titles = titles[1:] // pop the first empty string
	page.columnFormat = strings.Join(formats, columnSpacer)
	dividers = dividers[1:] // pop the first empty string

	// Create the frame for the header label
	headerFrame := clui.CreateFrame(page.content, sWidth, 1, clui.BorderNone, clui.Fixed)
	headerFrame.SetPack(clui.Vertical)
	headerFrame.SetPaddings(1, 0)
	columnsTitle := fmt.Sprintf(page.columnFormat, titles...)
	columnsLabel := clui.CreateLabel(headerFrame, AutoSize, 1, columnsTitle, clui.Fixed)
	columnsLabel.SetPaddings(0, 0)
	_, lHeight = columnsLabel.Size()
	sHeight -= lHeight // Remove the label from total height
	columnsDividors := fmt.Sprintf(page.columnFormat, dividers...)
	columnsDividorLabel := clui.CreateLabel(headerFrame, AutoSize, 1, columnsDividors, clui.Fixed)
	columnsDividorLabel.SetPaddings(0, 0)
	_, lHeight = columnsDividorLabel.Size()
	sHeight -= lHeight // Remove the label from total height

	page.scrollingFrame = clui.CreateFrame(page.content, sWidth, sHeight, clui.BorderNone, clui.Fixed)
	page.scrollingFrame.SetPack(clui.Vertical)
	page.scrollingFrame.SetScrollable(true)
	page.scrollingFrame.SetGaps(0, 1)
	page.scrollingFrame.SetPaddings(1, 0)

	var err error
	page.blockDevices, err = storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		page.Panic(err)
	}

	// Add a Revert button
	revertBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Revert", Fixed)
	revertBtn.OnClick(func(ev clui.Event) {
		var err error
		page.blockDevices, err = storage.RescanBlockDevices(nil)
		if err != nil {
			page.Panic(err)
		}

		// Clear last selected row as it might be removed
		page.lastDiskButton = nil
		page.lastAutoButton = nil
		page.lastPartButtons = nil
		page.activeSerial = ""
		page.data = nil
		page.getModel().TargetMedias = nil

		page.GotoPage(TuiPageDiskConfig)
	})

	// Add a Rescan media button
	rescanBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Rescan Media", Fixed)
	rescanBtn.OnClick(func(ev clui.Event) {
		var err error
		page.blockDevices, err = storage.RescanBlockDevices(page.getModel().TargetMedias)
		if err != nil {
			page.Panic(err)
		}

		// Clear last selected row as it might be removed
		page.lastDiskButton = nil
		page.lastAutoButton = nil
		page.lastPartButtons = nil

		// Check if the active device is still present
		var found bool
		for _, bd := range page.blockDevices {
			if bd.Serial == page.activeSerial {
				found = true
				selected := &SelectedBlockDevice{bd: bd, part: nil, addMode: false}
				if sel, ok := page.data.(*SelectedBlockDevice); ok {
					selected = sel.Clone()
					selected.part = nil
					selected.addMode = false
					selected.freePartition = nil
					page.data = selected
				}
				page.data = selected
			}
		}
		if !found {
			page.activeSerial = ""
			page.data = nil
			page.getModel().TargetMedias = nil
		}

		page.GotoPage(TuiPageDiskConfig)
	})

	page.activated = page.backBtn

	return page, nil
}

func (page *DiskConfigPage) findPartitionRow(bd *storage.BlockDevice, partNum uint64) *storage.BlockDevice {
	// Replace the last set of digits with the current partition number
	partName := bd.GetNewPartitionName(partNum)

	for _, partition := range bd.Children {
		if partition.Name == partName {
			return partition
		}
	}

	log.Warning("Did not find partition number %d and name %s", partNum, partName)
	return nil
}

func (page *DiskConfigPage) formatPartitionRow(partition *storage.BlockDevice) string {
	if partition == nil {
		log.Error("formatPartitionRow: partition was nil")
		return "ERROR"
	}
	pSize, pErr := partition.HumanReadableSize()
	if pErr != nil {
		log.Warning("formatPartitionRow: could not read size")
	}

	label := ""
	if partition.Label != "" {
		label = partition.Label
	}
	encrypt := ""
	if partition.Type == storage.BlockDeviceTypeCrypt {
		encrypt = "*"
	}

	return fmt.Sprintf(page.columnFormat,
		label, partition.Name, partition.FsType+encrypt, partition.MountPoint, pSize)
}

func (page *DiskConfigPage) addDiskRow(bd *storage.BlockDevice) error {
	size, err := bd.HumanReadableSizeWithPrecision(1)
	if err != nil {
		return err
	}

	rowFrame := clui.CreateFrame(page.scrollingFrame, 1, AutoSize, clui.BorderNone, clui.Fixed)
	rowFrame.SetPack(clui.Vertical)

	diskTitle := fmt.Sprintf(page.columnFormat,
		strings.TrimSpace(bd.Model), bd.GetDeviceFile(),
		"", "", size)

	diskButton := CreateSimpleButton(rowFrame, 1, 1, diskTitle, Fixed)
	diskButton.SetAlign(AlignLeft)

	partButtons := []*SimpleButton{}
	for _, part := range bd.PartTable {
		if part.Number == 0 || part.FileSystem == "free" {
			// Skip small free spaces
			if part.Size < (10 * 1024 * 1024) { // 10MiB
				continue
			}

			freeSpaceTxt, err := storage.HumanReadableSize(part.Size)
			if err != nil {
				log.Warning("Failed to get free space: %s", err)
			}
			partitionTitle := fmt.Sprintf(page.columnFormat, "", "Free Space",
				"", "", freeSpaceTxt)

			partitionButton := CreateSimpleButton(rowFrame, 1, 1, partitionTitle, Fixed)
			partitionButton.SetAlign(AlignLeft)
			partitionButton.SetStyle("Partition")
			partitionButton.SetTabStop(false)
			partitionButton.SetEnabled(false)

			newPart := &storage.BlockDevice{
				FsType:          "",
				UserDefined:     true,
				MakePartition:   true,
				FormatPartition: true,
				Type:            storage.BlockDeviceTypePart,
				MountPoint:      "",
				Size:            part.Size,
				Parent:          bd,
			}

			newParted := part.Clone()
			selected := &SelectedBlockDevice{bd: bd, part: newPart, addMode: true, freePartition: newParted}
			if sel, ok := page.data.(*SelectedBlockDevice); ok {
				selected = sel.Clone()
				selected.part = newPart
				selected.addMode = true
				selected.freePartition = newParted
			}

			partitionButton.OnClick(func(ev clui.Event) {
				page.data = selected
				page.GotoPage(TuiPageDiskPart)
			})

			partButtons = append(partButtons, partitionButton)
		} else {
			partition := page.findPartitionRow(bd, part.Number)
			partitionTitle := page.formatPartitionRow(partition)

			partitionButton := CreateSimpleButton(rowFrame, 1, 1, partitionTitle, Fixed)
			partitionButton.SetAlign(AlignLeft)
			partitionButton.SetStyle("Partition")
			partitionButton.SetTabStop(false)
			partitionButton.SetEnabled(false)

			selected := &SelectedBlockDevice{bd: bd, part: partition, addMode: false}
			if sel, ok := page.data.(*SelectedBlockDevice); ok {
				selected = sel.Clone()
				selected.part = partition
				selected.addMode = false
				selected.freePartition = nil
			}

			partitionButton.OnClick(func(ev clui.Event) {
				page.data = selected
				page.GotoPage(TuiPageDiskPart)
			})

			partButtons = append(partButtons, partitionButton)
		}
	}

	page.rowFrames = append(page.rowFrames, rowFrame)

	buttonFrame := clui.CreateFrame(rowFrame, 1, 1, clui.BorderNone, clui.Fixed)
	buttonFrame.SetPack(clui.Horizontal)
	buttonFrame.SetGaps(2, 0)
	autoButton := CreateSimpleButton(buttonFrame, AutoSize, 1, "Auto Partition", Fixed)
	autoButton.SetVisible(false)
	autoButton.OnClick(func(ev clui.Event) {
		storage.NewStandardPartitions(bd)
		selected := &SelectedBlockDevice{bd: bd, part: nil, addMode: false, wholeDisk: true, dataLoss: true}
		page.data = selected

		message := "Auto-partitioning results in data loss"
		if dialog, err := CreateWarningDialogBox(message); err != nil {
			log.Warning("%s: %s", message, err)
		} else {
			dialog.OnClose(func() {
				page.GotoPage(TuiPageDiskConfig)
				page.confirmBtn.SetEnabled(true)

				page.activeDisk = diskButton
				page.activeRow = rowFrame
				page.activeSerial = bd.Serial
			})
		}
	})

	diskButton.OnActive(func(active bool) {
		if active {
			page.scrollToRow(rowFrame)
		}
	})
	rowFrame.OnActive(func(active bool) {
		if active {
			page.scrollToRow(rowFrame)
		}
	})

	diskButton.OnClick(func(ev clui.Event) {
		sel, ok := page.data.(*SelectedBlockDevice)
		if ok {
			// Currently selected disk is partially or fully configured
			if status := sel.bd.GetConfiguredStatus(); status != storage.ConfiguredNone {
				// Do not allow selecting a different disk
				if sel.bd != bd {
					message := "Disk '" + sel.bd.GetDeviceFile() + "' already configured\n" +
						"as Installation Media. Use the 'Revert' button\n" +
						"or manually remove '/' and '/boot' mounts to\n" +
						"use a different disk."
					if _, err := CreateWarningDialogBox(message); err != nil {
						log.Warning("Attempt to use second disk: %s", err)
					}
					return
				}
			}
		}

		// The last frame changed was this frame
		if page.lastAutoButton == nil || page.lastAutoButton == autoButton {
			// toggle
			if autoButton.Visible() {
				page.diskOpen = false
				diskButton.SetStyle("")
				autoButton.SetVisible(false)
				for _, pButton := range partButtons {
					pButton.SetTabStop(false)
					pButton.SetEnabled(false)
				}
			} else {
				page.diskOpen = true
				diskButton.SetStyle("DiskSelected")
				autoButton.SetVisible(true)
				for _, pButton := range partButtons {
					pButton.SetTabStop(true)
					pButton.SetEnabled(true)
				}
			}
		} else {
			// Collapse the last row
			page.lastDiskButton.SetStyle("")
			page.lastAutoButton.SetVisible(false)
			for _, pButton := range page.lastPartButtons {
				pButton.SetTabStop(false)
				pButton.SetEnabled(false)
			}

			// Expand our row
			page.diskOpen = true
			diskButton.SetStyle("DiskSelected")
			autoButton.SetVisible(true)
			for _, pButton := range partButtons {
				pButton.SetTabStop(true)
				pButton.SetEnabled(true)
			}
		}

		page.lastDiskButton = diskButton
		page.lastAutoButton = autoButton
		page.lastPartButtons = partButtons

		page.activeDisk = diskButton
		page.activeRow = rowFrame
		page.activeSerial = bd.Serial

		selected := &SelectedBlockDevice{bd: bd, part: nil, addMode: false}
		if ok {
			selected = sel.Clone()
			selected.part = nil
			selected.addMode = false
			selected.freePartition = nil
		}
		page.data = selected

		clui.RefreshScreen()
	})

	// We do not have an active serial, so default to the current
	if page.activeSerial == "" {
		page.activeDisk = diskButton
		page.activeRow = rowFrame
		page.activeSerial = bd.Serial
		page.diskOpen = false
	} else if bd.Serial == page.activeSerial {
		// This is the active serial number, so set the
		// Active Disk button and Row Frame
		page.activeDisk = diskButton
		page.activeRow = rowFrame
	}

	return nil
}
