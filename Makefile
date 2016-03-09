PACKAGES=$(shell find . -name '*.go' -print0 | xargs -0 -n1 dirname | sort --unique)
TARGETS=$(shell find ./cmd -type d -depth 1 | xargs basename)

build: $(TARGETS) ## Create a build for each InfluxDB binary

$(TARGETS): generate
	go build -o $@ ./cmd/$@

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

metalint: deadcode cyclo aligncheck defercheck structcheck lint errcheck

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

.PHONY: metalint,deadcode,cyclo,aligncheck,defercheck,structcheck,lint,errcheck,help,cleanroom
