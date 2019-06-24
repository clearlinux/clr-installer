// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package proxy

import (
	"fmt"
	"os"
	"strings"

	"github.com/clearlinux/clr-installer/log"
)

// GetProxyValueFunc is the type of the GetProxyValue function
type GetProxyValueFunc func(prefix string) string

var (
	proxyPrefixes     = [...]string{"ftp", "http", "https", "socks", "no"}
	httpsProxy        string
	getProxyValueFunc GetProxyValueFunc
	preProxyFunc      func()
)

// SetHTTPSProxy defines the HTTPS_PROXY env var value for all the cmd executions
func SetHTTPSProxy(addr string) {
	log.Debug("proxy.SetHTTPSProxy = %s", addr)
	httpsProxy = addr
}

// SetPreProxyFunc save the function used to run before processing proxies
// This is currently used by Gnome UI to copy Network Proxy from install user
func SetPreProxyFunc(f func()) {
	preProxyFunc = f
}

// SetGetProxyValueFunc save the function used to return a string value
// for a Proxy based on the string prefix passed
func SetGetProxyValueFunc(f GetProxyValueFunc) {
	getProxyValueFunc = f
}

// GetProxyValues returns a set of environment variable for a Bash shell command
func GetProxyValues() []string {
	values := []string{}

	if preProxyFunc != nil {
		preProxyFunc()
	}

	myGetProxyValueFunc := getProxyValueFunc
	if myGetProxyValueFunc == nil {
		myGetProxyValueFunc = DefaultGetProxyValue
	}

	var value string
	for _, prefix := range proxyPrefixes {

		if prefix == "https" && httpsProxy != "" {
			value = httpsProxy
		} else {
			value = myGetProxyValueFunc(prefix)
		}

		if value != "" {
			values = append(values, fmt.Sprintf("%s_proxy=%s", prefix, value))

			upperValue := os.Getenv(strings.ToUpper(prefix) + "_PROXY")
			if upperValue != "" {
				values = append(values, fmt.Sprintf("%s_PROXY=%s", strings.ToUpper(prefix), upperValue))
			} else {
				values = append(values, fmt.Sprintf("%s_PROXY=%s", strings.ToUpper(prefix), value))
			}
		}
	}

	return values
}

// DefaultGetProxyValue default implementation which only pulls from
// the current environment variables
func DefaultGetProxyValue(prefix string) string {
	log.Debug("Using default shellProxy.DefaultGetProxyValue")
	return os.Getenv(prefix + "_proxy")
}
