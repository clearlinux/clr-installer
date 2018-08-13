// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package user

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/crypt"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

// User abstracts a target system definition
type User struct {
	Login    string
	Password string
	Admin    bool
}

const (
	defaultUsersFile = "/usr/share/defaults/etc/passwd"
	minPasswordWidth = 8
)

var (
	loginExp        = regexp.MustCompile("^[0-9,a-z,A-Z,-,_]*$")
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
func NewUser(login string, pwd string, admin bool) (*User, error) {
	hashed, err := crypt.Crypt(pwd)
	if err != nil {
		return nil, err
	}

	return &User{
		Login:    login,
		Password: hashed,
		Admin:    admin,
	}, nil
}

// SetPassword sets a users password
func (u *User) SetPassword(pwd string) error {
	hashed, err := crypt.Crypt(pwd)
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

	prg := progress.NewLoop("Adding extra users")
	if err := setTempTargetPAMConfig(rootDir); err != nil {
		prg.Failure()
		return err
	}

	for _, usr := range users {
		if err := usr.apply(rootDir); err != nil {
			prg.Failure()
			return err
		}
	}

	prg.Success()
	return nil
}

// apply applies the user configuration to the target install
func (u *User) apply(rootDir string) error {
	args := []string{
		"useradd",
		"--root",
		rootDir,
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

	args = []string{
		"chpasswd",
		"--root",
		rootDir,
		"-e",
	}

	pwd := fmt.Sprintf("%s:%s", u.Login, u.Password)

	if err := cmd.PipeRunAndLog(pwd, args...); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// IsValidLogin checks the minimum login requirements
func IsValidLogin(login string) (bool, string) {
	if login == "" {
		return false, "Login is required"
	}

	if !loginExp.MatchString(login) {
		return false, "Login must contain only numbers, letters, - or _"
	}

	return true, ""
}

// IsValidPassword checks the minimum password requirements
func IsValidPassword(pwd string) (bool, string) {
	if pwd == "" {
		return false, "Password is required"
	}

	if len(pwd) < minPasswordWidth {
		return false, fmt.Sprintf("Password must be at least %d characters long",
			minPasswordWidth)
	}

	return true, ""
}
