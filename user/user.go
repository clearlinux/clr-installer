// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package user

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/encrypt"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

// User abstracts a target system definition
type User struct {
	Login    string   `yaml:"login,omitempty"`
	UserName string   `yaml:"username,omitempty,flow"`
	Password string   `yaml:"password,omitempty,flow"`
	Admin    bool     `yaml:"admin,omitempty,flow"`
	SSHKeys  []string `yaml:"ssh-keys,omitempty,flow"`
}

const (
	defaultUsersFile = "/usr/share/defaults/etc/passwd"
	// MaxUsernameLength is the longest possible username
	MaxUsernameLength = 64
	// MaxLoginLength is the longest possible login
	MaxLoginLength = 31
	// MinPasswordLength is the shortest possible password
	MinPasswordLength = 8
	// MaxPasswordLength is the shortest possible password
	MaxPasswordLength = 255

	// RequiredBundle the bundle needed to enable non-root user accounts
	RequiredBundle = "sysadmin-basic"
)

var (
	usernameExp     = regexp.MustCompile("^([a-zA-Z]+[0-9a-zA-Z-_ ,'.]*|)$")
	loginExp        = regexp.MustCompile("^[a-z]+[0-9a-z-_]*$")
	sysDefaultUsers = []string{}
)

// IsSysDefaultUser checks if a given login is in the list of default users
func IsSysDefaultUser(login string) (bool, error) {
	if login == "" {
		return false, nil
	}

	if err := loadSysDefaultUsers(); err != nil {
		return false, err
	}

	for _, curr := range sysDefaultUsers {
		if curr == login {
			return true, nil
		}
	}

	return false, nil
}

func loadSysDefaultUsers() error {
	if len(sysDefaultUsers) != 0 {
		return nil
	}

	content, err := ioutil.ReadFile(defaultUsersFile)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(content), "\n") {
		tks := strings.Split(line, ":")

		if len(tks) == 0 {
			return errors.Errorf("Could not parse passwd file, line: %s", line)
		}

		sysDefaultUsers = append(sysDefaultUsers, tks[0])
	}

	return nil
}

// NewUser creates/allocates a new user handle
func NewUser(login string, username string, pwd string, admin bool) (*User, error) {
	hashed, err := encrypt.Crypt(pwd)
	if err != nil {
		return nil, err
	}

	return &User{
		Login:    login,
		UserName: username,
		Password: hashed,
		Admin:    admin,
	}, nil
}

// SetPassword sets a users password
func (u *User) SetPassword(pwd string) error {
	hashed, err := encrypt.Crypt(pwd)
	if err != nil {
		return err
	}

	u.Password = hashed
	return nil
}

// Equals returns true if u and usr point to the same struct or if both have
// the same Login string
func (u *User) Equals(usr *User) bool {
	return u == usr || u.Login == usr.Login
}

// setTempTargetPAMConfig copy the temporary chpasswd PAM config to target system
// this is required for changing user's password into target system.
func setTempTargetPAMConfig(rootDir string) error {
	var chpasswdFile string
	var err error

	pamDir := filepath.Join(rootDir, "etc", "pam.d")

	if err = utils.MkdirAll(pamDir, 0755); err != nil {
		return err
	}

	if chpasswdFile, err = conf.LookupChpasswdConfig(); err != nil {
		return err
	}

	targetPamFile := filepath.Join(pamDir, conf.ChpasswdPAMFile)
	if err = utils.CopyFile(chpasswdFile, targetPamFile); err != nil {
		return err
	}

	return nil
}

// Apply creates the user and sets their password into chroot'ed rootDir
func Apply(rootDir string, users []*User) error {
	if len(users) == 0 {
		return nil
	}

	prg := progress.NewLoop(utils.Locale.Get("Adding extra users"))
	if err := setTempTargetPAMConfig(rootDir); err != nil {
		prg.Failure()
		return err
	}

	// Should we lock out the root account?
	haveAdmins := false
	rootPassSet := false
	rootSSHOnly := false

	for _, usr := range users {
		log.Info("Adding extra user '%s'", usr.Login)
		if err := usr.apply(rootDir); err != nil {
			prg.Failure()
			return err
		}

		if usr.Admin {
			haveAdmins = true
		}

		// This should not be possible in the TUI as all system accounts
		// are not allowed to be defined, but is possible via the command
		// line (aka mass installer)
		if usr.Login == "root" {
			if usr.Password == "" {
				if len(usr.SSHKeys) > 0 {
					rootSSHOnly = true
				}
			} else {
				rootPassSet = true
			}
		}
	}

	// If the root account is not defined with an encrypted password and
	// we have user account which are Admin (sudo)
	// OR
	// The root account is defined with SSH Keys, no password
	if (!rootPassSet && haveAdmins) || rootSSHOnly {
		log.Info("Disabling the 'root' account.")
		if err := disableRoot(rootDir); err != nil {
			prg.Failure()
			return err
		}
	}

	prg.Success()
	return nil
}

// disableRoot will lockout the root account
// should be called only when adding an account which
// has been granted admin privileges (sudo)
func disableRoot(rootDir string) error {
	// Lock the account
	args := []string{
		"chroot",
		rootDir,
		"usermod",
		"--lock",
		"root",
	}

	if err := cmd.RunAndLog(args...); err != nil {
		return errors.Wrap(err)
	}

	// How many days since the beginning of (UNIX) time
	beginning := time.Date(1970, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	days := fmt.Sprintf("%d", int64(now.Sub(beginning).Hours()/24))

	// Set a password change date so we are not prompted
	// when sudo'ing to root account or when ssh'ing at root
	args = []string{
		"chroot",
		rootDir,
		"chage",
		"--lastday",
		days,
		"root",
	}

	if err := cmd.RunAndLog(args...); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// userExist will lockout the root account
// should be called only when adding an account which
// has been granted admin privileges (sudo)
func (u *User) userExist(rootDir string) bool {
	exists := true

	args := []string{
		"chroot",
		rootDir,
		"getent",
		"passwd",
		u.Login,
	}

	if err := cmd.RunAndLog(args...); err != nil {
		exists = false
	}

	return exists
}

// getUserHome returns the home directory of the user
// on the installation target
func (u *User) getUserHome(rootDir string) string {
	home := filepath.Join("/home", u.Login)

	// Ask for the accounts passwd entry and parse the home directory
	args := []string{
		"chroot",
		rootDir,
		"getent",
		"passwd",
		u.Login,
	}

	w := bytes.NewBuffer(nil)

	err := cmd.Run(w, args...)
	if err != nil {
		return home
	}

	getent := bytes.Split(w.Bytes(), []byte(":"))
	homeDir := string(getent[len(getent)-2])
	if len(homeDir) > 0 {
		home = homeDir
	}

	return home
}

// apply applies the user configuration to the target install
func (u *User) apply(rootDir string) error {
	accountAdded := false

	if u.userExist(rootDir) {
		log.Info("Account '%s' already a defined system account, skipping add.", u.Login)
	} else {
		args := []string{
			"chroot",
			rootDir,
			"useradd",
			"--comment",
			u.UserName,
			u.Login,
		}

		if u.Admin {
			args = append(args, []string{
				"-G",
				"wheel",
			}...)
		}

		if err := cmd.RunAndLog(args...); err != nil {
			return errors.Wrap(err)
		}

		accountAdded = true
	}

	if u.Password != "" {
		if !accountAdded {
			// Unlock the account
			// This is hack to ensure the account gets added to the
			// /etc/passwd file before trying to set the password with
			// chpasswd as the ch* commands only look in /etc
			args := []string{
				"chroot",
				rootDir,
				"usermod",
				"--unlock",
				u.Login,
			}

			if err := cmd.RunAndLog(args...); err != nil {
				return errors.Wrap(err)
			}
		}

		args := []string{
			"chroot",
			rootDir,
			"chpasswd",
			"-e",
		}

		pwd := fmt.Sprintf("%s:%s", u.Login, u.Password)

		if err := cmd.PipeRunAndLog(pwd, args...); err != nil {
			return errors.Wrap(err)
		}
	}

	if len(u.SSHKeys) > 0 {
		if err := writeSSHKey(rootDir, u); err != nil {
			return err
		}
	}

	return nil
}

func writeSSHKey(rootDir string, u *User) error {
	sshDir := filepath.Join(u.getUserHome(rootDir), ".ssh")
	dpath := filepath.Join(rootDir, sshDir)
	fpath := filepath.Join(dpath, "authorized_keys")

	if err := utils.MkdirAll(dpath, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	cnt := fmt.Sprintf("%s\n", strings.Join(u.SSHKeys, "\n"))
	bt := []byte(cnt)
	n, err := f.Write(bt)
	if err != nil {
		return err
	}

	if n != len(bt) {
		return errors.Errorf("Failed to write ssh key, wrote %d of %d bytes", n, len(bt))
	}

	args := []string{
		"chroot",
		rootDir,
		"/usr/bin/chown",
		"-R",
		fmt.Sprintf("%s:%s", u.Login, u.Login),
		sshDir,
	}

	if err := cmd.RunAndLog(args...); err != nil {
		return err
	}

	return nil
}

// IsValidUsername checks the username restrictions
func IsValidUsername(username string) (bool, string) {
	if !usernameExp.MatchString(username) {
		return false, utils.Locale.Get("Username must contain only numbers, letters, commas, - or _")
	}

	if len(username) > MaxUsernameLength {
		return false, utils.Locale.Get("UserName maximum length is %d", MaxUsernameLength)
	}

	return true, ""
}

// IsValidLogin checks the minimum login requirements
func IsValidLogin(login string) (bool, string) {
	if login == "" {
		return false, utils.Locale.Get("Login is required")
	}

	if len(login) > MaxLoginLength {
		return false, utils.Locale.Get("Login maximum length is %d", MaxLoginLength)
	}

	if !loginExp.MatchString(login) {
		return false, utils.Locale.Get("Login must contain only numbers, lower case letters, - or _")
	}

	return true, ""
}

// IsValidPassword checks the minimum password requirements
func IsValidPassword(pwd string) (bool, string) {
	if pwd == "" {
		return false, utils.Locale.Get("Password is required")
	}

	if len(pwd) < MinPasswordLength {
		return false, utils.Locale.Get("Password must be at least %d characters long", MinPasswordLength)
	}

	if len(pwd) > MaxPasswordLength {
		return false, utils.Locale.Get("Password may be at most %d characters long", MaxPasswordLength)
	}

	return true, ""
}
