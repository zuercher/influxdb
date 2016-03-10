PACKAGES = $(shell find . -name '*.go' -print0 | xargs -0 -n1 dirname | sort --unique)
TARGETS := $(shell find ./cmd -type d -depth 1 | xargs basename)

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
GIT_TAG := $(shell git describe --always --tags --abbrev=0)
GIT_COMMIT := $(shell git rev-parse HEAD)

build: $(TARGETS) ## Create a build for each InfluxDB binary (set 'static=true' to generate a static binary)

$(TARGETS): generate
	$(eval INFLUX_LINKER_FLAGS = -X main.version=$(GIT_TAG) -X main.branch=$(GIT_BRANCH) -X main.commit=$(GIT_COMMIT))
ifeq ($(static), true)
	$(eval INFLUX_COMPILE_PREPEND = CGO_ENABLED=0 )
	$(eval INFLUX_COMPILE_PARAMS = -ldflags "-s $(INFLUX_LINKER_FLAGS)" -a -installsuffix cgo )
else
	$(eval INFLUX_COMPILE_PARAMS = -ldflags "$(INFLUX_LINKER_FLAGS)" )
endif
	$(INFLUX_COMPILE_PREPEND)go build $(INFLUX_COMPILE_PARAMS)-o $@ ./cmd/$@

generate: get ## Generate static assets
	go generate ./services/admin

release: cleanroom ## Generate a release build

cleanroom:
ifneq ($(shell git diff-files --quiet --ignore-submodules -- ; echo $$?), 0)
	$(error "Uncommitted changes in the current directory.")
endif

get: ## Retrieve Go dependencies
	go get -t -d ./...

get-update: ## Retrieve updated Go dependencies
	go get -t -u -d ./...

metalint: get-tools deadcode cyclo aligncheck defercheck structcheck lint errcheck

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

get-tools: ## Download development tools
	go get github.com/remyoudompheng/go-misc/deadcode
	go get github.com/alecthomas/gocyclo
	go get github.com/opennota/check/...
	go get github.com/golang/lint/golint
	go get github.com/kisielk/errcheck
	go get github.com/sparrc/gdm

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: metalint,deadcode,cyclo,aligncheck,defercheck,structcheck,lint,errcheck,help,cleanroom,build
