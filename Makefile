# Copyright 2019 Intel Corporation
#
# SPDX-License-Identifier: GPL-3.0-only

top_srcdir = $(abspath .)
MAKEFLAGS += -r --no-print-directory

build_dir = $(top_srcdir)/build
build_bin_dir = $(build_dir)/bin
pkg_dir = $(top_srcdir)
cov_dir = $(top_srcdir)/.coverage

nproc = $(shell nproc)
orig_go_path = $(shell go env GOPATH)

# The -Wno-deprecated-declarations option is required to suppress deprecation
# warnings in the gotk3 package. https://github.com/gotk3/gotk3/issues/420
ifneq ($(CGO_CFLAGS),)
export CGO_CFLAGS += -Wno-deprecated-declarations
else
export CGO_CFLAGS = $(shell go env CGO_CFLAGS) -Wno-deprecated-declarations
endif

export GOPATH=$(pkg_dir)
export GO_PACKAGE_PREFIX := github.com/clearlinux/clr-installer
export TESTS_DIR := $(top_srcdir)/tests/
export TRAVIS_CONF = $(top_srcdir)/.travis.yml
export UPDATE_COVERAGE = 1

CLR_INSTALLER_TEST_HTTP_PORT ?= 8181

export TEST_HTTP_PORT = ${CLR_INSTALLER_TEST_HTTP_PORT}


THEME_DIR=$(DESTDIR)/usr/share/clr-installer/themes/
LOCALE_DIR=$(DESTDIR)/usr/share/locale
ISO_TEMPLATE_DIR=$(DESTDIR)/usr/share/clr-installer/iso_templates/

DESKTOP_DIR=$(DESTDIR)/usr/share/applications/
CONFIG_DIR=$(DESTDIR)/usr/share/defaults/clr-installer/
SYSTEMD_DIR=$(DESTDIR)/usr/lib/systemd/system/
PKIT_DIR=$(DESTDIR)/usr/share/polkit-1/

BUILDDATE=$(shell date -u "+%Y-%m-%d_%H:%M:%S_%Z")
# Are we running from a Git Repo?
$(shell [ -d .git ] || git rev-parse --is-inside-work-tree > /dev/null 2>&1)
ifeq ($(.SHELLSTATUS),0)
IS_GIT_REPO=1
else
IS_GIT_REPO=0
endif

ifeq ($(IS_GIT_REPO),1)
# Use the git tag and SHA
# Standard build case from Git repo
VERSION=$(shell git describe --tags --always --dirty  --match '[0-9]*.[0-9]*.[0-9]*' --exclude '[0-9]*.[0-9]*.[0-9]*.*[0-9]')
else
# If VERSION is defined in the environment, use it; otherwise...
ifeq ($(VERSION),)
# Attempt to parse from the directory name
# Building from a versioned source archive
mkfile_dir=$(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
VERSION=$(shell basename $(mkfile_dir) | awk -F- '{print $$NF}')
endif
endif

# Validate the Version
validate_version:
ifeq ($(IS_GIT_REPO),0)
ifeq (,$(shell echo $(VERSION) | egrep '^[0-9]+.[0-9]+.[0-9]+$$' 2> /dev/null))
	@echo "Invalid version string: $(VERSION)"
	@exit 1
endif
endif

.PHONY: gopath
LOCAL_GOPATH := ${CURDIR}/.gopath
export GOPATH := ${LOCAL_GOPATH}
gopath:
	@rm -rf ${LOCAL_GOPATH}/src
	@mkdir -p ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
ifeq ($(IS_GIT_REPO),1)
# Smart copy only files under version control
	@tar cf - `git ls-files` | tar xf - --directory=${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
else
	@cp -af * ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
endif

install: install-tui install-gui

install-common:
	@install -D -m 644  $(top_srcdir)/themes/clr-installer.theme $(THEME_DIR)/clr-installer.theme
	@mkdir -p -m 755 $(LOCALE_DIR)/
	@cp -rp --no-preserve=ownership  $(top_srcdir)/locale/* $(LOCALE_DIR)/
	@install -D -m 644  $(top_srcdir)/iso_templates/initrd_init_template $(ISO_TEMPLATE_DIR)/initrd_init_template
	@install -D -m 644  $(top_srcdir)/iso_templates/isolinux.cfg.template $(ISO_TEMPLATE_DIR)/isolinux.cfg.template
	@install -D -m 644  $(top_srcdir)/etc/clr-installer.yaml $(CONFIG_DIR)/clr-installer.yaml
	@install -D -m 644  $(top_srcdir)/etc/bundles.json $(CONFIG_DIR)/bundles.json
	@install -D -m 644  $(top_srcdir)/etc/kernels.json $(CONFIG_DIR)/kernels.json
	@install -D -m 644  $(top_srcdir)/etc/chpasswd $(CONFIG_DIR)/chpasswd

install-tui: build-tui install-common
	@install -D -m 755 $(top_srcdir)/.gopath/bin/clr-installer-tui $(DESTDIR)/usr/bin/clr-installer

install-gui: build-gui install-common
	@install -D -m 755 $(top_srcdir)/.gopath/bin/clr-installer-gui $(DESTDIR)/usr/bin/clr-installer-gui
	@install -D -m 644 $(top_srcdir)/etc/org.clearlinux.clr-installer-gui.policy $(PKIT_DIR)/actions/org.clearlinux.clr-installer-gui.policy
	@install -D -m 644 $(top_srcdir)/etc/org.clearlinux.clr-installer-gui.rules $(PKIT_DIR)/rules.d/org.clearlinux.clr-installer-gui.rules
	@install -D -m 644 $(top_srcdir)/themes/clr.png $(THEME_DIR)/clr.png
	@install -D -m 644 $(top_srcdir)/themes/style.css $(THEME_DIR)/style.css
	@install -D -m 644 $(top_srcdir)/etc/clr-installer-gui.desktop $(DESKTOP_DIR)/clr-installer-gui.desktop

uninstall:
	@rm -f $(DESTDIR)/usr/bin/clr-installer
	@rm -f $(PKIT_DIR)/actions/org.clearlinux.clr-installer-gui.policy
	@rm -f $(PKIT_DIR)/rules.d/org.clearlinux.clr-installer-gui.rules
	@rm -f $(THEME_DIR)/clr-installer.theme
	@rm -f $(THEME_DIR)/clr.png
	@rm -f $(THEME_DIR)/style.css
	@rm -f $(LOCALE_DIR)/*/LC_MESSAGES/clr-installer.po
	@rm -f $(CONFIG_DIR)/clr-installer.yaml
	@rm -f $(CONFIG_DIR)/bundles.json
	@rm -f $(CONFIG_DIR)/kernels.json
	@rm -f $(DESKTOP_DIR)/clr-installer-gui.desktop
	@rm -f $(CONFIG_DIR)/chpasswd
	@rm -f $(DESTDIR)/var/lib/clr-installer/clr-installer.yaml

build-pkgs: build
	@for pkg in `find -path ./vendor -prune -o -path ./.gopath -prune -o -name "*.go" \
	   -printf "%h\n" | sort -u | sed 's/\.\///g'`; do \
	   go install -p ${nproc} -v $${GO_PACKAGE_PREFIX}/$$pkg; \
   done

build-vendor: build
	@cp -a vendor/* .gopath/src/
	@for pkg in `find ./vendor -name "*.go" \
	   -printf "%h\n" | sort -u | sed 's/\.\/vendor\///g'`; do \
	   go install -p ${nproc} -v $$pkg; \
   done
	@rm -rf .gopath/src/*

build: build-tui build-gui
	ln ${GOPATH}/bin/clr-installer-tui ${GOPATH}/bin/clr-installer

build-go-get-tui: validate_version gopath
	go get -p ${nproc} -v ${GO_PACKAGE_PREFIX}/clr-installer

build-tui: build-go-get-tui
	@echo "MAKEFLAGS=${MAKEFLAGS}"
	go install -p ${nproc} -v \
		-ldflags="-X github.com/clearlinux/clr-installer/model.Version=${VERSION} \
		-X github.com/clearlinux/clr-installer/model.BuildDate=${BUILDDATE}" \
		${GO_PACKAGE_PREFIX}/clr-installer
	mv ${GOPATH}/bin/clr-installer ${GOPATH}/bin/clr-installer-tui

build-gui: build-go-get-tui
	go get -p ${nproc} -v -tags guiBuild ${GO_PACKAGE_PREFIX}/clr-installer
	go install -p ${nproc} -v -tags guiBuild \
		-ldflags="-X github.com/clearlinux/clr-installer/model.Version=${VERSION} \
		-X github.com/clearlinux/clr-installer/model.BuildDate=${BUILDDATE}" \
		${GO_PACKAGE_PREFIX}/clr-installer
	mv ${GOPATH}/bin/clr-installer ${GOPATH}/bin/clr-installer-gui

build-local-travis: validate_version gopath
	@go get -p ${nproc} -v ${GO_PACKAGE_PREFIX}/local-travis
	@go install -p ${nproc} -v \
		-ldflags="-X github.com/clearlinux/clr-installer/model.Version=${VERSION} \
		-X github.com/clearlinux/clr-installer/model.BuildDate=${BUILDDATE}" \
		${GO_PACKAGE_PREFIX}/local-travis

check-coverage: build-local-travis
	@echo "local-travis simulation:"
	@$(top_srcdir)/.gopath/bin/local-travis

check: gopath bundle-check
	@# Ensure no temp files are left behind
	@LSCMD='ls -lart --ignore="." /tmp'; \
	SHACMD='ls -art --ignore="." /tmp | sha512sum'; \
	BEFORELS=`eval $$LSCMD`; \
	BEFORESHA=`eval $$SHACMD`; \
	go test ${CHECK_VERBOSE} -cover ${GO_PACKAGE_PREFIX}/...; \
	AFTERSHA=`eval $$SHACMD`; \
	AFTERLS=`eval $$LSCMD`; \
	if [ "$$BEFORESHA" != "$$AFTERSHA" ] ; then \
		echo "Test Failed: Temporary directory may not be clean!"; \
		echo "Left-over files:"; \
		echo "$$BEFORELS" > /tmp/beforels; \
		echo "$$AFTERLS" > /tmp/afterls; \
		diff -Nr /tmp/beforels /tmp/afterls; \
		/bin/false ; \
	fi; \

check-clean: gopath
	go clean -testcache

check-root: gopath
	sudo -E go test ${CHECK_VERBOSE} -cover ${GO_PACKAGE_PREFIX}/...

PHONY += bundle-check
bundle-check:
	@${top_srcdir}/scripts/bundle-check.sh

PHONY += coverage
coverage: build
	@# Test for coverage and race conditions
	@rm -rf ${cov_dir}; \
	mkdir -p ${cov_dir}; \
	for pkg in $$(go list $$GO_PACKAGE_PREFIX/...); do \
		file="${cov_dir}/$$(echo $$pkg | tr / -).cover"; \
		go test -race -covermode="atomic" -coverprofile="$$file" "$$pkg"; \
		if [ "$$?" -ne 0 ]; then \
			exit 1; \
		fi \
	done; \
	echo "mode: count" > ${cov_dir}/cover.out; \
	grep -h -v "^mode:" ${cov_dir}/*.cover >>"${cov_dir}/cover.out"; \

PHONY += coverage-func
coverage-func: coverage
	@go tool cover -func="${cov_dir}/cover.out"

PHONY += coverage-html
coverage-html: coverage
	@go tool cover -html="${cov_dir}/cover.out"

PHONY += install-linters
install-linters:
	@if ! ${orig_go_path}/bin/golangci-lint --version &>/dev/null; then \
		echo "Installing linters..."; \
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b ${orig_go_path}/bin; \
	fi

PHONY += install-linters-force
install-linters-force:
	echo "Force Installing linters..."
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b ${orig_go_path}/bin

PHONY += update-linters
update-linters:
	@if ${orig_go_path}/bin/golangci-lint --version &>/dev/null; then \
		echo "Updating linters..."; \
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b ${orig_go_path}/bin; \
	else \
		echo "Linters not installed"; \
		exit 1; \
	fi

PHONY += lint
lint: lint-travis-checkers
	@echo "Travis Linters complete"

PHONY += lint-release
lint-release: lint-checkers
	@echo "Linters complete"

PHONY += lint-core
lint-core: build install-linters gopath
	@rm -rf ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/vendor
	@cp -af vendor/* ${LOCAL_GOPATH}/src/
	@echo "Running linters"

PHONY += lint-travis-checkers
lint-travis-checkers: lint-checkers
# lint-travis-checkers: lint-mispell lint-ineffassign lint-gocyclo lint-gofmt \
# lint-golint lint-deadcode lint-varcheck \
# lint-unused lint-vetshadow lint-errcheck

PHONY += lint-checkers
lint-checkers: lint-mispell lint-vet lint-ineffassign lint-gocyclo lint-gofmt \
lint-golint lint-deadcode lint-varcheck lint-structcheck \
lint-unused lint-vetshadow lint-errcheck

PHONY += lint-mispell
lint-mispell: lint-core
	@echo "Running linter lint-mispell"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=misspell \
	./...

PHONY += lint-vet
lint-vet: lint-core
	@echo "Running linter lint-vet"
	@${orig_go_path}/bin/golangci-lint run --deadline=20m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=govet \
	./...

PHONY += lint-ineffassign
lint-ineffassign: lint-core
	@echo "Running linter lint-ineffassign"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=ineffassign \
	./...

# TODO: Resolve gocyclo errors for skipped directories and files
PHONY += lint-gocyclo
lint-gocyclo: lint-core
	@echo "Running linter lint-gocyclo"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=gocyclo \
	--skip-dirs clr-installer,storage,controller \
	--skip-files gui/pages/disk_config.go,model/model_ister.go \
	./...

PHONY += lint-gofmt
lint-gofmt: lint-core
	@echo "Running linter lint-gofmt"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=gofmt \
	./...

PHONY += lint-golint
lint-golint: lint-core
	@echo "Running linter lint-golint"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=golint \
	./...

PHONY += lint-deadcode
lint-deadcode: lint-core
	@echo "Running linter lint-deadcode"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=deadcode \
	./...

PHONY += lint-varcheck
lint-varcheck: lint-core
	@echo "Running linter lint-varcheck"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=varcheck \
	./...

PHONY += lint-structcheck
lint-structcheck: lint-core
	@echo "Running linter lint-structcheck"
	@${orig_go_path}/bin/golangci-lint run --deadline=20m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=structcheck \
	./...

PHONY += lint-unused
lint-unused: lint-core
	@echo "Running linter lint-unused"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=unused \
	./...

PHONY += lint-vetshadow
lint-vetshadow: lint-core
	@echo "Running linter lint-vetshadow"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=vetshadow \
	./...

PHONY += lint-errcheck
lint-errcheck: lint-core
	@echo "Running linter lint-errcheck"
	@${orig_go_path}/bin/golangci-lint run --deadline=10m --tests \
	--skip-dirs-use-default \
	--disable-all \
	--enable=errcheck \
	./...

PHONY += dep-install
dep-install: gopath
	@if ! dep version &>/dev/null; then \
		echo "Installing dep..."; \
		mkdir -p ${orig_go_path}/bin; \
		curl https://raw.githubusercontent.com/golang/dep/master/install.sh 2>/dev/null \
		| GOPATH=${orig_go_path} bash; \
	fi

PHONY += dep-check
dep-check: dep-install
	@if ! dep version &>/dev/null; then \
		echo "Installing dep..."; \
		mkdir -p ${orig_go_path}/bin; \
		curl https://raw.githubusercontent.com/golang/dep/master/install.sh 2>/dev/null \
		| GOPATH=${orig_go_path} bash; \
	fi
	@cd ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX} ; GOPATH=${LOCAL_GOPATH} dep check

PHONY += dep-update
dep-update: dep-install
	@if dep version &>/dev/null; then \
		echo "Updating dep..."; \
		curl https://raw.githubusercontent.com/golang/dep/master/install.sh 2>/dev/null \
		| GOPATH=${orig_go_path} bash; \
	else \
		echo "Dep not installed"; \
		exit 1; \
	fi

PHONY += vendor-init
vendor-init: gopath dep-install
	@rm -rf ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/vendor
	@rm -f ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/Gopkg.*
	@cd ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX} ; GOPATH=${LOCAL_GOPATH} dep init
	@cp -a ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/vendor ${top_srcdir}
	@cp -a ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/Gopkg.* ${top_srcdir}

PHONY += vendor-status
vendor-status: gopath dep-install
	@cd ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX} ; GOPATH=${LOCAL_GOPATH} dep status

PHONY += vendor-check
vendor-check: dep-check

PHONY += vendor-update
vendor-update: gopath dep-install
	@# Copy the updated files from revision control area
	@cp -a ${top_srcdir}/Gopkg.* ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
	@# Pull updates
	@cd ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX} ; GOPATH=${LOCAL_GOPATH} dep ensure -update
	@# Copy results back to revision control area
	@cp -a ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/vendor ${top_srcdir}
	@cp -a ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/Gopkg.* ${top_srcdir}

PHONY += vendor-add
vendor-add: gopath dep-install
	@# Copy the updated files from revision control area
	@cp -a ${top_srcdir}/Gopkg.* ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
	@# Pull updates
	@cd ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX} ; GOPATH=${LOCAL_GOPATH} dep ensure -add ${GOADD}
	@# Copy results back to revision control area
	@cp -a ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/vendor ${top_srcdir}
	@cp -a ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}/Gopkg.* ${top_srcdir}


PHONY += tag
ifeq ($(IS_GIT_REPO),1)
tag:
	@if git diff-index --quiet HEAD &>/dev/null; then \
		if git diff @{upstream}.. --quiet &>/dev/null; then \
			echo "Create and push the Tag to GitHub"; \
			echo "git tag <version>"; \
			echo "git push <remote> <version>"; \
		else \
			echo "Unpushed changes; git push upstream and try again."; \
			exit 1; \
		fi \
	else \
		echo "Uncomiited changes; git commit and try again."; \
		exit 1; \
	fi
else
tag:
	@echo "Not running from Git Repo; tag will not work."
	@exit 1
endif

PHONY += clean
ifeq ($(IS_GIT_REPO),1)
clean:
	@go clean -i -r
	@git clean -fdXq
else
clean:
	@go clean -i -r
endif

PHONY += distclean
ifeq ($(IS_GIT_REPO),1)
dist-clean: clean
	@if [ "$$(git status -s)" = "" ]; then \
		git clean -fdxq; \
		git reset HEAD; \
		go clean -testcache; \
		go clean -modcache; \
	else \
		echo "There are pending changes in the repository!"; \
		git status -s; \
		echo "Please check in changes or stash, and try again."; \
	fi
else
dist-clean: clean
	@go clean -testcache
	@go clean -modcache
endif

all: build

PHONY += all

.PHONY = $(PHONY)
.DEFAULT_GOAL = all
