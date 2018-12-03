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
	bd      *storage.BlockDevice
	part    *storage.BlockDevice
	addMode bool
}

const (
	diskConfigTitle = `Configure Media`
)
const (
	columnDisk = iota
	columnPartition
	columnFsType
	columnMount
	columnSize
	columnCount
)

var (
	diskColumns []columnInfo
)

func init() {
	diskColumns = make([]columnInfo, columnCount)

	diskColumns[columnDisk].title = "Disk"
	diskColumns[columnDisk].minWidth = 16

	diskColumns[columnPartition].title = "Partition"
	diskColumns[columnPartition].minWidth = columnWidthDefault

	diskColumns[columnFsType].title = "File System"
	diskColumns[columnFsType].minWidth = columnWidthDefault

	diskColumns[columnMount].title = "Mount Point"
	diskColumns[columnMount].minWidth = -1 // This column get all free space

	diskColumns[columnSize].title = "Size"
	diskColumns[columnSize].minWidth = 8
	diskColumns[columnSize].rightJustify = true
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
	lastAddButton   *SimpleButton
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

		selected.Children = sel.bd.Children
		page.getModel().AddTargetMedia(selected)
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

				if status == storage.ConfiguredFull {
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
	page.lastAddButton = nil
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
	page := &DiskConfigPage{
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}
	page.setupMenu(tui, TuiPageDiskConfig, diskConfigTitle, CancelButton|ConfirmButton, TuiPageMenu)

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

	// Build the Header Title and full row format string
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
		page.lastAddButton = nil
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
		page.lastAddButton = nil
		page.lastPartButtons = nil

		// Check if the active device is still present
		var found bool
		for _, bd := range page.blockDevices {
			if bd.Serial == page.activeSerial {
				found = true
				selected := &SelectedBlockDevice{bd: bd, part: nil, addMode: false}
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
	for _, partition := range bd.Children {
		pSize, pErr := partition.HumanReadableSize()
		if pErr != nil {
			return pErr
		}

		label := ""
		if partition.Label != "" {
			label = partition.Label
		}
		encrypt := ""
		if partition.Type == storage.BlockDeviceTypeCrypt {
			encrypt = "*"
		}

		partitionTitle := fmt.Sprintf(page.columnFormat,
			label, partition.Name, partition.FsType+encrypt, partition.MountPoint, pSize)

		partitionButton := CreateSimpleButton(rowFrame, 1, 1, partitionTitle, Fixed)
		partitionButton.SetAlign(AlignLeft)
		partitionButton.SetStyle("Partition")
		partitionButton.SetTabStop(false)
		partitionButton.SetEnabled(false)

		selected := &SelectedBlockDevice{bd: bd, part: partition, addMode: false}

		partitionButton.OnClick(func(ev clui.Event) {
			page.data = selected
			page.GotoPage(TuiPageDiskPart)
		})

		partButtons = append(partButtons, partitionButton)
	}

	page.rowFrames = append(page.rowFrames, rowFrame)

	buttonFrame := clui.CreateFrame(rowFrame, 1, 1, clui.BorderNone, clui.Fixed)
	buttonFrame.SetPack(clui.Horizontal)
	buttonFrame.SetGaps(2, 0)
	autoButton := CreateSimpleButton(buttonFrame, AutoSize, 1, "Auto Partition", Fixed)
	autoButton.SetVisible(false)
	autoButton.OnClick(func(ev clui.Event) {
		storage.NewStandardPartitions(bd)
		selected := &SelectedBlockDevice{bd: bd, part: nil, addMode: false}
		page.data = selected

		page.GotoPage(TuiPageDiskConfig)
		page.confirmBtn.SetEnabled(true)

		page.activeDisk = diskButton
		page.activeRow = rowFrame
		page.activeSerial = bd.Serial
	})

	addButton := CreateSimpleButton(buttonFrame, AutoSize, 1, "Add Partition", Fixed)
	addButton.SetVisible(false)
	freeSpace, err := bd.FreeSpace()
	if err != nil {
		return err
	}
	if freeSpace <= 0 {
		addButton.SetEnabled(false)
	} else {
		freeSpaceTxt, err := storage.HumanReadableSize(freeSpace)
		if err != nil {
			return err
		}
		availableTxt := "Available Space: " + freeSpaceTxt

		clui.CreateLabel(buttonFrame, AutoSize, 1, availableTxt, clui.Fixed)

		addButton.OnClick(func(ev clui.Event) {
			newPart := &storage.BlockDevice{
				FsType:     "ext4",
				Type:       storage.BlockDeviceTypePart,
				MountPoint: "",
				Size:       freeSpace,
				Parent:     bd,
			}
			bd.AddChild(newPart)
			page.data = &SelectedBlockDevice{bd: bd, part: newPart, addMode: true}
			page.GotoPage(TuiPageDiskPart)
		})
	}

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
		if sel, ok := page.data.(*SelectedBlockDevice); ok {
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
				addButton.SetVisible(false)
				for _, pButton := range partButtons {
					pButton.SetTabStop(false)
					pButton.SetEnabled(false)
				}
			} else {
				page.diskOpen = true
				diskButton.SetStyle("DiskSelected")
				autoButton.SetVisible(true)
				addButton.SetVisible(true)
				for _, pButton := range partButtons {
					pButton.SetTabStop(true)
					pButton.SetEnabled(true)
				}
			}
		} else {
			// Collapse the last row
			page.lastDiskButton.SetStyle("")
			page.lastAutoButton.SetVisible(false)
			page.lastAddButton.SetVisible(false)
			for _, pButton := range page.lastPartButtons {
				pButton.SetTabStop(false)
				pButton.SetEnabled(false)
			}

			// Expand our row
			page.diskOpen = true
			diskButton.SetStyle("DiskSelected")
			autoButton.SetVisible(true)
			addButton.SetVisible(true)
			for _, pButton := range partButtons {
				pButton.SetTabStop(true)
				pButton.SetEnabled(true)
			}
		}

		page.lastDiskButton = diskButton
		page.lastAutoButton = autoButton
		page.lastAddButton = addButton
		page.lastPartButtons = partButtons

		page.activeDisk = diskButton
		page.activeRow = rowFrame
		page.activeSerial = bd.Serial

		selected := &SelectedBlockDevice{bd: bd, part: nil, addMode: false}
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
