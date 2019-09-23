package gui

import (
	"time"

	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// networkTestDialog is a network test pop-up box
type networkTestDialog struct {
	box           *gtk.Box
	label         *gtk.Label
	confirmButton *gtk.Widget
	dialog        *gtk.Dialog
	pbar          *gtk.ProgressBar
}

// createNetworkTestDialog creates a pop-up window for the network test
func createNetworkTestDialog() (*networkTestDialog, error) {
	var err error
	netDialog := &networkTestDialog{}
	progress.Set(netDialog)

	// Create box
	netDialog.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	netDialog.box.SetHAlign(gtk.ALIGN_FILL)
	netDialog.box.SetMarginBottom(common.TopBottomMargin)
	if err != nil {
		log.Error("Error creating box", err)
		return nil, err
	}

	// Create progress bar
	netDialog.pbar, err = gtk.ProgressBarNew()
	if err != nil {
		return nil, err
	}
	_, err = glib.IdleAdd(func() {
		netDialog.pbar.SetFraction(0.0)
	})
	if err != nil {
		return nil, err
	}
	netDialog.pbar.SetHAlign(gtk.ALIGN_FILL)
	netDialog.pbar.SetMarginBottom(12)
	netDialog.pbar.SetMarginTop(12)
	netDialog.pbar.SetPulseStep(0.3)
	netDialog.box.PackStart(netDialog.pbar, false, false, 0)

	// Create label
	text := utils.Locale.Get("Testing connectivity")
	netDialog.label, err = common.SetLabel(text, "label-warning", 0.0)
	if err != nil {
		log.Error("Error creating label", err)
		return nil, err
	}
	netDialog.label.SetUseMarkup(true)
	netDialog.label.SetHAlign(gtk.ALIGN_START)
	netDialog.box.PackStart(netDialog.label, false, true, 0)

	// Create dialog
	netDialog.dialog, err = common.CreateDialogOneButton(netDialog.box, text, utils.Locale.Get("OK"), "button-confirm")
	if err != nil {
		log.Error("Error creating dialog", err)
		return nil, err
	}
	netDialog.dialog.SetDeletable(false)

	// Configure confirm button
	netDialog.confirmButton, err = netDialog.dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
	if err != nil {
		log.Error("Error getting confirm button", err)
		return nil, err
	}
	_, err = netDialog.confirmButton.Connect("clicked", func() {
		netDialog.dialog.Destroy()
	})
	if err != nil {
		return nil, err
	}
	netDialog.confirmButton.SetSensitive(false)

	netDialog.dialog.ShowAll()

	return netDialog, nil
}

// RunNetworkTest creates pop-up window that runs a network check
func RunNetworkTest(md *model.SystemInstall) error {
	netDialog, err := createNetworkTestDialog()
	if err != nil {
		return err
	}

	go func() {
		if err = controller.ConfigureNetwork(md); err != nil {
			// Network check failed
			log.Error("Network Testing: %s", err)
		}

		// Automatically close the dialog on success
		if controller.NetworkPassing {
			time.Sleep(time.Second)
			_, err = glib.IdleAdd(func() {
				netDialog.dialog.Destroy()
			})
			if err != nil {
				log.ErrorError(err)
				netDialog.dialog.Destroy()
			}
		}
	}()
	netDialog.dialog.Run()

	return nil
}

// Desc will push a description box into the view for later marking
func (netDialog *networkTestDialog) Desc(printPrefix, desc string) {
	_, err := glib.IdleAdd(func() {
		// Ignore printPrefix which is primarily used to separate
		// mass installer installation steps.
		netDialog.label.SetText(desc)
		netDialog.label.ShowAll()
	})
	if err != nil {
		log.ErrorError(err)
		return
	}
}

// Failure handles failure to install
func (netDialog *networkTestDialog) Failure() {
	_, err := glib.IdleAdd(func() {
		netDialog.label.SetText(utils.Locale.Get("Network check failed."))
		netDialog.confirmButton.SetSensitive(true)
		netDialog.label.ShowAll()
	})
	if err != nil {
		log.ErrorError(err)
		return
	}
}

// Success notes the install was successful
func (netDialog *networkTestDialog) Success() {
	_, err := glib.IdleAdd(func() {
		netDialog.label.SetText(utils.Locale.Get("Success"))
		netDialog.confirmButton.SetSensitive(true)
		netDialog.label.ShowAll()
	})
	if err != nil {
		log.ErrorError(err)
		return
	}
}

// LoopWaitDuration will return the duration for step-waits
func (netDialog *networkTestDialog) LoopWaitDuration() time.Duration {
	return common.LoopWaitDuration
}

// Partial handles an actual progress update
func (netDialog *networkTestDialog) Partial(total int, step int) {
	return
}

// Step will step the progressbar in indeterminate mode
func (netDialog *networkTestDialog) Step() {
	_, err := glib.IdleAdd(func() {
		netDialog.pbar.Pulse()
	})
	if err != nil {
		log.ErrorError(err)
		return
	}
}
