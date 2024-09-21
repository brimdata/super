GOIMPORTS_VERSION = v0.19.0
PIGEON_VERSION = v1.2.1

deps = $(CURDIR)/deps

all: parser.go

.PHONY: parser.go
parser.go:
ifeq "$(shell go version -m $(deps)/bin/goimports 2>&1 | fgrep $(GOIMPORTS_VERSION))" ""
	GOBIN=$(deps)/bin go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
endif
ifeq "$(shell go version -m $(deps)/bin/pigeon 2>&1 | fgrep $(PIGEON_VERSION))" ""
	GOBIN=$(deps)/bin go install github.com/mna/pigeon@$(PIGEON_VERSION)
endif
	$(deps)/bin/pigeon -alternate-entrypoints SuperSQL -support-left-recursion -o $@ parser.peg
	$(deps)/bin/goimports -w $@
