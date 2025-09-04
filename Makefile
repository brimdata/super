export GO111MODULE=on

VERSION = $(shell git describe --tags --dirty --always)
LDFLAGS = -s -X github.com/brimdata/super/cli.version=$(VERSION)
BUILD_COMMANDS = ./cmd/super

ifeq "$(filter-out 386 arm mips mipsle, $(shell go env GOARCH))" ""
$(error 32-bit architectures are unsupported; see https://github.com/brimdata/super/issues/4044)
endif

# This enables a shortcut to run a single test from the ./ztests suite, e.g.:
#  make TEST=TestZed/ztests/suite/cut/cut
ifneq "$(TEST)" ""
test-one: test-run
endif

# Uncomment this to trigger re-builds of the peg files when the grammar
# is out of date.  We are commenting this out to work around issue #1717.
#PEG_DEP=peg

vet:
	@go vet ./...

fmt:
	gofmt -s -w .
	git diff --exit-code -- '*.go'

tidy:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

SAMPLEDATA:=zed-sample-data/README.md

$(SAMPLEDATA):
	git clone --depth=1 https://github.com/brimdata/zed-sample-data $(@D)

sampledata: $(SAMPLEDATA)

bin/minio: Makefile
	@curl -o $@ --compressed --create-dirs \
		https://dl.min.io/server/minio/release/$$(go env GOOS)-$$(go env GOARCH)/archive/minio.RELEASE.2022-05-04T07-45-27Z
	@chmod +x $@

generate:
	go generate ./...

test-generate: generate
	git diff --exit-code

test-unit:
	@go test -short ./...

test-system: build bin/minio
	@ZTEST_PATH="$(CURDIR)/dist:$(CURDIR)/bin" go test .

test-run: build bin/minio
	@ZTEST_PATH="$(CURDIR)/dist:$(CURDIR)/bin" go test . -v -run $(TEST)

test-heavy: build
	@PATH="$(CURDIR)/dist:$(PATH)" go test -tags=heavy ./mdtest

perf-compare: build $(SAMPLEDATA)
	scripts/perf-compare.sh

output-check: build $(SAMPLEDATA)
	scripts/output-check.sh

build: $(PEG_DEP)
	@mkdir -p dist
	@go build -ldflags='$(LDFLAGS)' -o dist ./cmd/...

install:
	@go install -ldflags='$(LDFLAGS)' $(BUILD_COMMANDS)

.PHONY: installdev
installdev:
	@go install -ldflags='$(LDFLAGS)' ./cmd/...

compiler/parser/parser.go: compiler/parser/parser.peg go.mod
	go tool pigeon -support-left-recursion -o $@ $<
	go tool goimports -w $@

.PHONY: peg
peg: compiler/parser/parser.go

.PHONY: markdown-lint
markdown-lint:
	@npm install --no-save markdownlint-cli@0.35.0
	@npx markdownlint --ignore-path .gitignore .

# CI performs these actions individually since that looks nicer in the UI;
# this is a shortcut so that a local dev can easily run everything.
test-ci: fmt tidy vet test-generate test-unit test-system test-heavy

clean:
	@rm -rf dist

.PHONY: fmt tidy vet test-unit test-system test-heavy sampledata test-ci
.PHONY: perf-compare build install clean generate test-generate
