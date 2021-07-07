// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/gui/network"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// IconDirectory is where we can find bundle icons
	IconDirectory = "/usr/share/clear/bundle-icons"
)

var (
	// IconSuffixes is the supported set of suffixes for the
	// current Clear Bundles
	IconSuffixes = []string{
		".svg",
		".png",
	}
)

// Bundle is a simple page to help with Bundle settings
type Bundle struct {
	model            *model.SystemInstall
	windowController Controller
	bundles          []*swupd.Bundle     // Known bundles
	box              *gtk.Box            // Main layout
	checks           *gtk.FlowBox        // Where to store checks
	scroll           *gtk.ScrolledWindow // Scroll the checks

	selections []*gtk.CheckButton
	clearPage  bool
}

type decisionDialog struct {
	box           *gtk.Box
	label         *gtk.Label
	dialog        *gtk.Dialog
	confirmButton *gtk.Widget
	cancelButton  *gtk.Widget
}

// LookupBundleIcon attempts to find the icon for the given bundle.
// If it is found, we'll return true and the icon path, otherwise
// we'll return false with an empty string.
func LookupBundleIcon(bundle *swupd.Bundle) (string, bool) {
	for _, suffix := range IconSuffixes {
		path := filepath.Join(IconDirectory, fmt.Sprintf("%s%s", bundle.Name, suffix))
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

// createBundleWidget creates new displayable widget for the given bundle
func createBundleWidget(bundle *swupd.Bundle) (*gtk.CheckButton, error) {
	// Create the root layout
	root, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}

	// Create display check
	check, err := gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}
	check.SetMarginTop(6)
	check.SetMarginStart(12)

	// Create display image
	img, err := gtk.ImageNew()
	img.SetMarginStart(12)
	img.SetMarginEnd(6)
	if err != nil {
		return nil, err
	}
	icon, set := LookupBundleIcon(bundle)
	if set {
		pbuf, err := gdk.PixbufNewFromFileAtSize(icon, 48, 48)
		if err != nil {
			set = false
		} else {
			img.SetFromPixbuf(pbuf)
		}
	}

	// Still not set? Fallback.
	if !set {
		img.SetFromIconName("package-x-generic", gtk.ICON_SIZE_INVALID)
	}
	img.SetPixelSize(48)
	img.SetSizeRequest(48, 48)
	root.PackStart(img, false, false, 0)

	txt := fmt.Sprintf("<b>%s</b>\n%s", bundle.Name, utils.Locale.Get(bundle.Desc))
	label, err := gtk.LabelNew(txt)
	if err != nil {
		return nil, err
	}
	label.SetMarginStart(6)
	label.SetMarginEnd(12)
	label.SetXAlign(0.0)
	root.PackStart(label, false, false, 0)
	label.SetUseMarkup(true)

	check.Add(root)
	return check, nil
}

func createDecisionBox(model *model.SystemInstall, bundle *Bundle) (*decisionDialog, error) {
	var err error
	decisionMaker := &decisionDialog{}

	decisionMaker.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	decisionMaker.box.SetHAlign(gtk.ALIGN_FILL)
	decisionMaker.box.SetMarginBottom(common.TopBottomMargin)

	text := utils.Locale.Get("This requires a working network connection.\nProceed with a network test?")
	decisionMaker.label, err = common.SetLabel(text, "label-warning", 0.0)

	if err != nil {
		return nil, err
	}
	decisionMaker.label.SetUseMarkup(true)
	decisionMaker.label.SetHAlign(gtk.ALIGN_START)
	decisionMaker.box.PackStart(decisionMaker.label, false, true, 0)

	decisionMaker.dialog, err = common.CreateDialogOkCancel(decisionMaker.box,
		utils.Locale.Get("Network Required"), utils.Locale.Get("CONFIRM"), utils.Locale.Get("CANCEL"))

	if err != nil {
		return nil, err
	}

	decisionMaker.dialog.SetDeletable(false)

	// Configure confirm button
	var buttonIWidget gtk.IWidget
	buttonIWidget, err = decisionMaker.dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
	if err != nil {
		return nil, err
	}
	decisionMaker.confirmButton = buttonIWidget.ToWidget()

	// Configure cancel button
	buttonIWidget, err = decisionMaker.dialog.GetWidgetForResponse(gtk.RESPONSE_CANCEL)
	if err != nil {
		return nil, err
	}
	decisionMaker.cancelButton = buttonIWidget.ToWidget()

	_ = decisionMaker.confirmButton.Connect("clicked", func() {
		if ret, _ := network.RunNetworkTest(model); ret != network.NetTestSuccess {
			bundle.clearPage = true
			bundle.ResetChanges()
			decisionMaker.dialog.Destroy()
			return
		}
		bundle.clearPage = false
		bundle.windowController.SetButtonState(ButtonConfirm, controller.NetworkPassing)
		decisionMaker.dialog.Destroy()
	})

	_ = decisionMaker.cancelButton.Connect("clicked", func() {
		bundle.clearPage = true
		decisionMaker.dialog.Destroy()
		bundle.ResetChanges()
	})

	decisionMaker.confirmButton.SetSensitive(true)
	decisionMaker.cancelButton.SetSensitive(true)
	decisionMaker.dialog.ShowAll()

	return decisionMaker, nil
}

// NewBundlePage returns a new BundlePage
func NewBundlePage(windowController Controller, model *model.SystemInstall) (Page, error) {
	var err error
	bundle := &Bundle{
		windowController: windowController,
		model:            model,
	}

	// Load our bundles
	bundle.bundles, err = swupd.LoadBundleList(model)
	if err != nil {
		return nil, err
	}

	// main layout
	bundle.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	bundle.box.SetBorderWidth(8)

	// check list
	bundle.checks, err = gtk.FlowBoxNew()
	if err != nil {
		return nil, err
	}
	bundle.checks.SetSelectionMode(gtk.SELECTION_MULTIPLE)
	bundle.scroll, err = gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	// no horizontal scrolling
	bundle.scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	bundle.scroll.Add(bundle.checks)
	bundle.box.PackStart(bundle.scroll, true, true, 0)

	// Match the bundle set to our ticks
	for _, b := range bundle.bundles {
		wid, err := createBundleWidget(b)
		if err != nil {
			return nil, err
		}
		bundle.checks.Add(wid)
		bundle.selections = append(bundle.selections, wid)
	}

	for i := range bundle.selections {
		if !controller.NetworkPassing {
			_ = bundle.selections[i].Connect("toggled", func() {
				if !controller.NetworkPassing && !bundle.clearPage {
					// we dont want to fire any checkbox signals
					// when we want to clear the page. if encounter
					// clearPage set to true, we set to
					// false so that it fire nexttime
					// onwards
					_, err := createDecisionBox(model, bundle)
					if err != nil {
						return
					}
				}

				if !controller.NetworkPassing && bundle.clearPage {
					bundle.clearPage = false
				}
			})
		}
	}

	return bundle, nil
}

// IsDone checks if all the steps are completed
func (bundle *Bundle) IsDone() bool {
	return true
}

// IsRequired will return false as we have default values
func (bundle *Bundle) IsRequired() bool {
	return false
}

// GetID returns the ID for this page
func (bundle *Bundle) GetID() int {
	return PageIDBundle
}

// GetIcon returns the icon for this page
func (bundle *Bundle) GetIcon() string {
	return "applications-system"
}

// GetRootWidget returns the root embeddable widget for this page
func (bundle *Bundle) GetRootWidget() gtk.IWidget {
	return bundle.box
}

// GetSummary will return the summary for this page
func (bundle *Bundle) GetSummary() string {
	return utils.Locale.Get("Select Additional Bundles")
}

// GetTitle will return the title for this page
func (bundle *Bundle) GetTitle() string {
	return bundle.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (bundle *Bundle) StoreChanges() {
	// Match model selection to our selections
	for n, b := range bundle.bundles {
		set := bundle.selections[n].GetActive()
		if set {
			bundle.model.AddUserBundle(b.Name)
		} else {
			bundle.model.RemoveUserBundle(b.Name)
		}
	}
}

// ResetChanges will reset this page to match the model
func (bundle *Bundle) ResetChanges() {
	// Match selection to what's in the model
	for n, b := range bundle.bundles {
		bundle.selections[n].SetActive(bundle.model.ContainsUserBundle(b.Name))
	}
	bundle.windowController.SetButtonState(ButtonConfirm, controller.NetworkPassing)
}

// GetConfiguredValue returns our current config
func (bundle *Bundle) GetConfiguredValue() string {
	if len(bundle.model.UserBundles) == 0 {
		return utils.Locale.Get("No additional bundles selected")
	}
	return " • " + strings.Join(bundle.model.UserBundles, "\n • ")
}
