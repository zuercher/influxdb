PACKAGES = $(shell find . -name '*.go' -print0 | xargs -0 -n1 dirname | sort --unique)
TARGETS := $(shell find ./cmd -type d -depth 1 | xargs basename)

GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
GIT_TAG = $(shell git describe --always --tags --abbrev=0 | tr -d 'v')
GIT_COMMIT = $(shell git rev-parse HEAD)

all: envcheck restore $(TARGETS) ## Create a build for each InfluxDB binary (set 'static=true' to generate a static binary)

$(TARGETS): generate ## Generate a build for each target
ifeq ($(shell grep -q '1.4'<<< $$(go version); echo $$?),0)
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
	$(INFLUX_COMPILE_PREPEND)go install $(INFLUX_COMPILE_PARAMS)./cmd/$@

generate: get ## Generate static assets
	go generate ./services/admin

release: cleanroom all ## Tag and generate a release build, must specify a version (example: make release version=0.1.2)

envcheck: ## Check environment for any common issues
ifeq ($$GOPATH,)
	$(error "No GOPATH set!")
endif
	$(eval CURR_DIR = $(shell pwd))
ifneq ($(shell grep -q $$GOPATH <<< $$(pwd); echo $$?),0)
	$(error "Current directory ($(PWD)) is not under your GOPATH ($(GOPATH))")
endif

cleanroom: envcheck ## Create a 'clean room' environment for generating a release
ifneq ($(shell git diff-files --quiet --ignore-submodules -- ; echo $$?), 0)
	$(error "Uncommitted changes in the current directory.")
endif
	$(eval CURR_DIR = $(shell pwd))
	$(eval TEMP_DIR = $(shell mktemp -d))
	mkdir -p $(TEMP_DIR)/src/github.com/influxdata/influxdb
	cp -r . $(TEMP_DIR)/src/github.com/influxdata/influxdb
	cd $(TEMP_DIR)
	GOPATH="$(TEMP_DIR)" make --file=$(TEMP_DIR)/src/github.com/influxdata/influxdb/Makefile all
	cd $(CURR_DIR)

restore: ## Restore pinned version dependencies with gdm
	$$GOPATH/bin/gdm restore

get: ## Retrieve Go dependencies
	go get -t -d ./...

get-update: ## Retrieve updated Go dependencies
	go get -t -u -d ./...

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

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: metalint,deadcode,cyclo,aligncheck,defercheck,structcheck,lint,errcheck,help,cleanroom,envcheck,get-dev-tools
