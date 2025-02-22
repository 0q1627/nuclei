# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOMOD=$(GOCMD) mod
GOTEST=$(GOCMD) test
GOFLAGS := -v
# This should be disabled if the binary uses pprof
LDFLAGS := -s -w

ifneq ($(shell go env GOOS),darwin)
LDFLAGS := -extldflags "-static"
endif
    
all: build
build:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "nuclei" cmd/nuclei/main.go
docs:
	if ! which dstdocgen > /dev/null; then
		echo -e "Command not found! Install? (y/n) \c"
		go get -v github.com/projectdiscovery/yamldoc-go/cmd/docgen/dstdocgen
	fi
	$(GOCMD) generate pkg/templates/templates.go
	$(GOBUILD) -o "cmd/docgen/docgen" cmd/docgen/docgen.go
	./cmd/docgen/docgen docs.md nuclei-jsonschema.json
test:
	$(GOTEST) $(GOFLAGS) ./...
integration:
	cd integration_tests; bash run.sh
functional:
	cd cmd/functional-test; bash run.sh
tidy:
	$(GOMOD) tidy
devtools:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "bindgen" pkg/js/devtools/bindgen/cmd/bindgen/main.go
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "tsgen" pkg/js/devtools/tsgen/cmd/tsgen/main.go
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "scrapefuncs" pkg/js/devtools/scrapefuncs/main.go
jsupdate:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "bindgen" pkg/js/devtools/bindgen/cmd/bindgen/main.go
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "tsgen" pkg/js/devtools/tsgen/cmd/tsgen/main.go
	./bindgen -dir pkg/js/libs -out pkg/js/generated
	./tsgen -dir pkg/js/libs -out pkg/js/generated/ts
ts:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "tsgen" pkg/js/devtools/tsgen/cmd/tsgen/main.go
	./tsgen -dir pkg/js/libs -out pkg/js/generated/ts
memogen:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "memogen" cmd/memogen/memogen.go
	./memogen -src pkg/js/libs -tpl cmd/memogen/function.tpl

