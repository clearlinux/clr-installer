// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"time"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/progress"
)

// InstallPage is the Page implementation for installation progress page, it also implements
// the progress.Client interface
type InstallPage struct {
	BasePage
	rebootBtn *SimpleButton
	exitBtn   *SimpleButton
	prgBar    *clui.ProgressBar
	prgLabel  *clui.Label
	prgMax    int
}

var (
	loopWaitDuration = 2 * time.Second
)

// Success is part of the progress.Client implementation and represents the
// successful progress completion of a task by setting
// the progress bar to "full"
func (page *InstallPage) Success() {
	page.prgBar.SetValue(page.prgMax)
	clui.RefreshScreen()
}

// Failure is part of the progress.Client implementation and represents the
// unsuccessful progress completion of a task by setting
// the progress bar to "fail"
func (page *InstallPage) Failure() {
	bg := page.prgBar.BackColor()
	page.prgBar.SetValue(0)
	for i := 1; i <= 5; i++ {
		page.prgBar.SetBackColor(term.ColorRed)
		clui.RefreshScreen()
		time.Sleep(100 * time.Millisecond)
		page.prgBar.SetBackColor(bg)
		clui.RefreshScreen()
		time.Sleep(100 * time.Millisecond)
	}
}

// Step is part of the progress.Client implementation and moves the progress bar one step
// case it becomes full it starts again
func (page *InstallPage) Step() {
	if page.prgBar.Value() == page.prgMax {
		page.prgBar.SetValue(0)
	} else {
		page.prgBar.Step()
	}
	clui.RefreshScreen()
}

// Desc is part of the progress.Client implementation and sets the progress bar label
func (page *InstallPage) Desc(desc string) {
	page.prgLabel.SetTitle(desc)
	clui.RefreshScreen()
}

// Partial is part of the progress.Client implementation and adjusts the progress bar to the
// current completion percentage
func (page *InstallPage) Partial(total int, step int) {
	perc := (step / total)
	value := page.prgMax * perc
	page.prgBar.SetValue(int(value))
}

// LoopWaitDuration is part of the progress.Client implementation and returns the time duration
// each step should wait until calling Step again
func (page *InstallPage) LoopWaitDuration() time.Duration {
	return loopWaitDuration
}

// Activate is called when the page is "shown"
func (page *InstallPage) Activate() {
	go func() {
		progress.Set(page)

		err := controller.Install(page.tui.rootDir, page.getModel(), page.tui.options)
		if err != nil {
			page.Panic(err)
			return // In a panic state, do not continue
		}

		go func() {
			_ = network.DownloadInstallerMessage("Post-Installation",
				network.PostInstallConf)
		}()
		page.prgLabel.SetTitle("Installation complete")
		page.rebootBtn.SetEnabled(true)
		page.exitBtn.SetEnabled(true)
		clui.ActivateControl(page.GetWindow(), page.rebootBtn)
		clui.RefreshScreen()

		page.tui.installReboot = true
	}()
}

func newInstallPage(tui *Tui) (Page, error) {
	page := &InstallPage{}
	page.setup(tui, TuiPageInstall, NoButtons, TuiPageMenu)

	lbl := clui.CreateLabel(page.content, 2, 2, "Installing Clear Linux* OS", Fixed)
	lbl.SetPaddings(0, 2)

	progressFrame := clui.CreateFrame(page.content, AutoSize, 3, BorderNone, clui.Fixed)
	progressFrame.SetPack(clui.Vertical)

	page.prgBar = clui.CreateProgressBar(progressFrame, AutoSize, AutoSize, clui.Fixed)

	page.prgMax, _ = page.prgBar.Size()
	page.prgBar.SetLimits(0, page.prgMax)

	page.prgLabel = clui.CreateLabel(progressFrame, 1, 1, "Installing", Fixed)
	page.prgLabel.SetPaddings(0, 3)

	page.rebootBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Reboot", Fixed)
	page.rebootBtn.OnClick(func(ev clui.Event) {
		go clui.Stop()
	})
	page.rebootBtn.SetEnabled(false)

	page.exitBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Exit", Fixed)
	page.exitBtn.OnClick(func(ev clui.Event) {
		page.tui.installReboot = false
		go clui.Stop()
	})
	page.exitBtn.SetEnabled(false)

	return page, nil
}
