// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package crypt

/*
#cgo LDFLAGS: -lcrypt

#define _GNU_SOURCE

#include <stdlib.h>
#include <string.h>
#include <crypt.h>
#include <unistd.h>
#include <fcntl.h>

#define SALT_SIZE 19 // includes id and separators
#define ENTROPY_SIZE SALT_SIZE
#define DICT_SIZE 64

const char dict[DICT_SIZE] =
   "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";

char *__salt_and_crypt(char prefix[3], char *pass, size_t size) {
  int i, fd;
  ssize_t randSize;
  char rand[ENTROPY_SIZE], salt[SALT_SIZE], *enc;

  // TODO change __salt_and_crypt signature to receive an *int so we can write back
  // the errno if getentropy() fails

#if __GLIBC__ >= 2 && __GLIBC_MINOR__ >= 25
  if (getentropy(&rand, ENTROPY_SIZE) != 0) {
    return NULL;
  }
#else
  fd = open("/dev/urandom", O_RDONLY);
  if (fd < 0) {
    return NULL;
  }

  randSize = read(fd, rand, ENTROPY_SIZE);
  close(fd);
  if (randSize != ENTROPY_SIZE) {
    return NULL;
  }
#endif

  salt[0] = prefix[0];
  salt[1] = prefix[1];
  salt[2] = prefix[2];

  for (i = 3; i < SALT_SIZE; i++) {
    salt[i] = dict[rand[i] & (DICT_SIZE - 1)];
  }

  enc = crypt(pass, salt);

  // Ensure that the rand and salt arrays are zeroed out before returning.  The
  // asm statement below will instruct the compiler to not elide the memset() call
  __asm__ __volatile__("" :: "g"(rand) : "memory");
  memset(rand, 0, ENTROPY_SIZE);
  __asm__ __volatile__("" :: "g"(salt) : "memory");
  memset(salt, 0, SALT_SIZE);

  return enc;
}
*/
import "C"
import "unsafe"

const (
	// SHA512Prefix is the glibc' crypt() identifier for sha512
	SHA512Prefix = "$6$"

	// SHA512Size is the glibc' crypt() resulting string size
	SHA512Size = 107
)

// Crypt calls glibc' crypt() and getentropy() functions and create a PAM
// compatible password entry/token. Currently we only support sha512 meaning
// the algorithm is not selectable/customizable.
func Crypt(password string) (string, error) {
	cPassword := C.CString(password)
	defer C.free(unsafe.Pointer(cPassword))

	cPrefix := C.CString(SHA512Prefix)
	defer C.free(unsafe.Pointer(cPrefix))

	cHashed, err := C.__salt_and_crypt(cPrefix, cPassword, SHA512Size)
	if cHashed == nil {
		return "", err
	}

	defer func() {
		C.memset(unsafe.Pointer(cHashed), 0, C.strlen(cHashed))
		C.free(unsafe.Pointer(cHashed))
	}()

	return C.GoString(cHashed), nil
}
