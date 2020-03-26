// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/clearlinux/clr-installer/utils"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
)

const (
	// MinPassphraseLength is the shortest possible password
	MinPassphraseLength = 8
	// MaxPassphraseLength is the shortest possible password
	MaxPassphraseLength = 94

	// RequiredBundle the bundle needed if encrypted partitions are used
	RequiredBundle = "boot-encrypted"
	// KernelArgument is kernel argument needed if encrypted partitions are used
	KernelArgument = "rootflags=x-systemd.device-timeout=0"

	// EncryptHash use for LUKS encryption
	EncryptHash = "sha256"
	// EncryptCipher use for LUKS encryption
	EncryptCipher = "aes-xts-plain64"
	// EncryptKeySize use for LUKS encryption
	EncryptKeySize = 512
)

// EncryptionRequiresPassphrase checks all partition to see if encryption was enabled
func (bd *BlockDevice) EncryptionRequiresPassphrase(isAdvanced bool) bool {
	enabled := (bd.Type == BlockDeviceTypeCrypt && bd.FsType != "swap")

	for _, ch := range bd.Children {
		if len(ch.Children) > 0 {
			enabled = enabled || ch.EncryptionRequiresPassphrase(isAdvanced)
		} else {
			enabled = enabled ||
				(ch.Type == BlockDeviceTypeCrypt &&
					ch.LabeledAdvanced == isAdvanced &&
					ch.FsTypeNotSwap())
		}
	}

	return enabled
}

// MapEncrypted uses cryptsetup to format (initialize) and open (map) the
// physical partion to an encrypted partition
func (bd *BlockDevice) MapEncrypted(passphrase string) error {
	if bd.Type != BlockDeviceTypeCrypt {
		return errors.Errorf("Trying to run cryptsetup() against a non crypt partition")
	}

	args := []string{
		"cryptsetup",
		"--batch-mode",
		fmt.Sprintf("--hash=%s", EncryptHash),
		fmt.Sprintf("--cipher=%s", EncryptCipher),
		fmt.Sprintf("--key-size=%d", EncryptKeySize),
	}

	if bd.Label != "" {
		args = append(args, "--label="+bd.Label)
	}

	args = append(args, "luksFormat", bd.GetDeviceFile(), "-")

	if err := cmd.PipeRunAndLog(passphrase, args...); err != nil {
		return errors.Wrap(err)
	}

	mapped, err := bd.getMappedName()
	if err != nil {
		return errors.Wrap(err)
	}

	args = []string{
		"cryptsetup",
		"--batch-mode",
		"luksOpen",
	}

	args = append(args, bd.GetDeviceFile(), mapped, "-")

	if err := cmd.PipeRunAndLog(passphrase, args...); err != nil {
		return errors.Wrap(err)
	}

	log.Debug("Disk partition %q is mapped to encrypted partition %q", bd.Name, mapped)

	// Store the mapped point for later unmounting
	mountedEncrypts = append(mountedEncrypts, mapped)

	bd.MappedName = filepath.Join("mapper", mapped)

	return nil
}

// unMapEncrypted uses cryptsetup to close (unmap) an encrypted partition
func unMapEncrypted(mapped string) error {
	args := []string{
		"cryptsetup",
		"--batch-mode",
		"luksClose",
		mapped,
	}

	if err := cmd.RunAndLog(args...); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// getMappedName uses dmsetup to find already mapped encrypted
// names and return and available, unique name
func (bd *BlockDevice) getMappedName() (string, error) {
	var mapped string

	// Special case for mapping 'root'
	if bd.MountPoint == "/" {
		mapped = "root"
	} else {
		// make the mapped device all lower case
		// drop the leading '/'
		mapped = strings.TrimPrefix(strings.ToLower(bd.MountPoint), "/")
		// replace '/' with '_'
		mapped = strings.Replace(mapped, "/", "_", -1)
	}

	args := []string{
		"dmsetup",
		"ls",
		"--target",
		"crypt",
	}

	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, args...)
	if err != nil {
		return "", errors.Wrap(err)
	}

	lines := strings.Split(w.String(), "\n")
	nameOkay := false
	try := 0
	for !nameOkay && try < 5 {
		try++
		inList := false

		for _, curr := range lines {
			log.Debug("line is: %s", curr)
			parts := strings.Fields(curr)
			if len(parts) < 1 {
				continue
			}

			log.Debug("Comparing: %s", parts[0])

			if mapped == parts[0] {
				inList = true
				log.Debug("Found InList: %s", mapped)
			}
		}

		if inList {
			log.Debug("Still Found InList: %s", mapped)
			mapped = getNewMappedName(mapped)
			log.Debug("New Mapped Name is : %s", mapped)
		} else {
			nameOkay = true
		}
	}

	if try >= 5 && !nameOkay {
		return "", errors.Errorf("Tried 5 times, but could not map encrypted name")
	}

	return mapped, nil
}

// getNewMappedName converts the ending string to an integer and increment
func getNewMappedName(input string) string {
	if input == "" {
		return input
	}

	matchLabel := regexp.MustCompile(`^(.*?)([0-9]{0,})$`)

	matches := matchLabel.FindStringSubmatch(input)

	mapped := ""
	if len(matches) == 3 {
		suffix, err := strconv.ParseUint(matches[2], 10, 64)
		if err == nil {
			suffix = suffix + 1
		} else {
			suffix = 1
		}
		mapped = matches[1] + strconv.FormatUint(suffix, 10)
	} else {
		mapped = input
	}

	return mapped
}

// IsValidPassphrase checks the minimum passphrase requirements
func IsValidPassphrase(phrase string) (bool, string) {
	if phrase == "" {
		return false, utils.Locale.Get("Passphrase is required")
	}

	if !isPrintable(phrase) {
		return false, utils.Locale.Get("Passphrase may only contain 7-bit, printable characters")
	}

	if len(phrase) < MinPassphraseLength {
		return false, utils.Locale.Get("Passphrase must be at least %d characters long", MinPassphraseLength)
	}

	if len(phrase) > MaxPassphraseLength {
		return false, utils.Locale.Get("Passphrase may be at most %d characters long", MaxPassphraseLength)
	}

	if status, errstring := cmd.CracklibCheck(phrase, "Passphrase"); !status {
		return false, errstring
	}

	return true, ""
}

// GetPassPhrase prompts to the user interactively for the pass phrase
// via the command line.
// This is intended to be used to get a pass phrase for encrypting
// file systems on the installation target while using the command
// line (aka massinstall)
func GetPassPhrase() string {
	passphrase := ""
	confirm := ""
	done := false

	for !done {
		passphrase = askPassPhrase(utils.Locale.Get("Disk Encryption Passphrase"))
		confirm = askPassPhrase(utils.Locale.Get("Confirm Passphrase"))

		if passphrase != confirm {
			fmt.Print("Passphrases do not match!\n\n")
		} else {
			done = true
		}
	}

	return passphrase
}

func askPassPhrase(prompt string) string {
	passphrase := ""
	done := false

	// Get the initial state of the terminal.
	initialTermState, termErr := terminal.GetState(syscall.Stdin)
	if termErr != nil {
		log.Warning("Unable to get terminal state for recovery: %v", termErr)
	}

	// Restore it in the event of an interrupt.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP,
		syscall.SIGABRT, syscall.SIGSTKFLT, syscall.SIGSYS)

	go func() {
		<-c
		_ = terminal.Restore(syscall.Stdin, initialTermState)
		signal.Stop(c)
	}()

	for !done {
		fmt.Print(prompt + ": ")
		bytePassphrase, err := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Print("\n")
		if err == nil {
			passphrase = string(bytePassphrase)
			strings.TrimSpace(passphrase)

			errMsg := ""
			if done, errMsg = IsValidPassphrase(passphrase); !done {
				fmt.Println(errMsg)
			}
		} else {
			done = true
			fmt.Printf("Error getting passphrase: %v", err)
			passphrase = ""
		}
	}

	signal.Stop(c)

	return passphrase
}

func isPrintable(s string) bool {
	for _, c := range s {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}
