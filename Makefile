# This is the InfluxDB Makefile
#
# USAGE -
# For usage information, simply run `make` from the root directory of
# the repository.
#
# Please file all issues against the InfluxDB repository at:
# https://github.com/influxdata/influxdb
#

PACKAGES = $(shell find . -name '*.go' -print0 | xargs -0 -n1 dirname | sort --unique)
TARGETS := $(shell find ./cmd/* -maxdepth 0 -type d | xargs -L1 basename)

GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
GIT_TAG = $(shell git describe --always --tags --abbrev=0 | tr -d 'v')
GIT_COMMIT = $(shell git rev-parse HEAD)

all: envcheck restore $(TARGETS) ## Create a build for each InfluxDB binary (set 'static=true' to generate a static binary)

$(TARGETS): generate ## Generate a build for each target
ifeq ($(shell go version | grep -q '1.4'; echo $$?),0)
	$(eval INFLUX_LINKER_FLAGS = -X main.version $(GIT_TAG) -X main.branch $(GIT_BRANCH) -X main.commit $(GIT_COMMIT))
else
	$(eval INFLUX_LINKER_FLAGS = -X main.version=$(GIT_TAG) -X main.branch=$(GIT_BRANCH) -X main.commit=$(GIT_COMMIT))
endif
ifeq ($(static), true)
	$(eval INFLUX_COMPILE_PREPEND = CGO_ENABLED=0 )
	$(eval INFLUX_COMPILE_PARAMS = -ldflags "-s $(INFLUX_LINKER_FLAGS)" -a -installsuffix cgo )
else
	$(eval INFLUX_COMPILE_PARAMS = -ldflags "$(INFLUX_LINKER_FLAGS)" )
endif
	@echo "Building '$@' -> $$GOPATH/bin/$@"
	$(INFLUX_COMPILE_PREPEND)go build -o $$GOPATH/bin/$@ $(INFLUX_COMPILE_PARAMS)./cmd/$@

generate: get ## Generate static assets
	PATH=$$PATH:$$GOPATH/bin go generate ./services/admin

release: cleanroom ## Tag and generate a release build, must specify a version (example: make release version=0.1.2)

envcheck: ## Check environment for any common issues
ifneq ($(shell which go &>/dev/null; echo $$?),0)
	$(error "Go not installed.")
endif
ifeq ($$GOPATH,)
	$(error "No GOPATH set.")
endif
ifneq ($(shell pwd | grep -q $$GOPATH; echo $$?),0)
	$(eval CWD = $(shell pwd))
	$(error "Current directory ($(CWD)) is not under your GOPATH ($(GOPATH)).")
endif

cleanroom: ## Create a 'clean room' build (copies repository to temporary directory and rebuilds environment)
ifneq ($(shell git diff-files --quiet --ignore-submodules -- ; echo $$?), 0)
	$(error "Uncommitted changes in the current directory.")
endif
	$(eval CURR_DIR = $(shell pwd))
	$(eval TEMP_DIR = $(shell mktemp -d))
	mkdir -p $(TEMP_DIR)/src/github.com/influxdata/influxdb
	cp -r . $(TEMP_DIR)/src/github.com/influxdata/influxdb
	cd $(TEMP_DIR)/src/github.com/influxdata/influxdb
	GOPATH="$(TEMP_DIR)" make all
	cd $(CURR_DIR)
	cp $(TEMP_DIR)/bin/influx* .

restore: ## Restore pinned version dependencies with gdm
ifneq ($(shell PATH=$$PATH:$$GOPATH/bin which gdm &>/dev/null; echo $$?), 0)
	go get github.com/sparrc/gdm
	mkdir -p $$GOPATH/bin
	go build -o $$GOPATH/bin/gdm github.com/sparrc/gdm
endif
	PATH=$$PATH:$$GOPATH/bin gdm restore

get: ## Retrieve Go dependencies
	go get -t -d ./...

get-update: ## Retrieve updated Go dependencies
	go get -t -u -d ./...

deb: ## Create a Debian package

nightly: ## Create a nightly build

metalint: get-dev-tools deadcode cyclo aligncheck defercheck structcheck lint errcheck

deadcode:
	@deadcode $(PACKAGES) 2>&1

cyclo:
	@gocyclo -over 10 $(PACKAGES)

aligncheck:
	@aligncheck $(PACKAGES)

defercheck:
	@defercheck $(PACKAGES)

structcheck:
	@structcheck $(PACKAGES)

lint:
	@for pkg in $(PACKAGES); do golint $$pkg; done

errcheck:
	@for pkg in $(PACKAGES); do \
		errcheck -ignorepkg=bytes,fmt -ignore=":(Rollback|Close)" $$pkg \
	done

get-dev-tools: ## Download development tools
	go get github.com/remyoudompheng/go-misc/deadcode
	go get github.com/alecthomas/gocyclo
	go get github.com/opennota/check/...
	go get github.com/golang/lint/golint
	go get github.com/kisielk/errcheck
	go get github.com/sparrc/gdm

clean: ## Remove existing binaries
	@for target in $(TARGETS); do \
		rm -f $$target ; \
	done

help: ## Display usage information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[31m%-31s\033[0m %s\n", $$1, $$2}'

.PHONY: metalint,deadcode,cyclo,aligncheck,defercheck,structcheck,lint,errcheck,help,cleanroom,envcheck,get-dev-tools,restore,deb
.DEFAULT_GOAL := help
