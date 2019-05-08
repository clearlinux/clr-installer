// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package encrypt

import (
	"crypto/rand"
	"fmt"

	"github.com/GehirnInc/crypt"
	// package requires import the hash method to blank
	_ "github.com/GehirnInc/crypt/sha512_crypt"
)

// CreateSalt generates a random salt for encrypting user password
func CreateSalt() (string, error) {
	const saltBytes int = 19

	dict := "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	salt := make([]byte, saltBytes)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	salt[0] = '$'
	salt[1] = '6'
	salt[2] = '$'

	for i := 3; i < saltBytes; i++ {
		salt[i] = dict[salt[i]%byte(len(dict))]
	}

	return string(salt), nil
}

// Crypt take a password and hashes with a random salt using SHA512
func Crypt(password string) (string, error) {
	salt, saltErr := CreateSalt()
	if saltErr != nil {
		return "", fmt.Errorf("Cannot generate salt: %v", saltErr)
	}

	crypt := crypt.SHA512.New()
	hash, hashErr := crypt.Generate([]byte(password), []byte(salt))
	if hashErr != nil {
		return "", fmt.Errorf("Cannot generate salt: %v", hashErr)
	}

	return hash, nil
}
