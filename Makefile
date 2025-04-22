BUILDTAGS = "netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp"
BUILDFLAGS = -tags ${BUILDTAGS} -installsuffix netgo
BUILDPATHS = ./pkg/... ./cli/... ./tests/...

# Set version from git tag, or use "dev" if not a tagged commit
VERSION ?= $(shell git describe --tags 2>/dev/null || echo "dev")

.PHONY: build
build: mod-tidy fmt vet
	go build -o bin/sbctl -ldflags "-X github.com/replicatedhq/sbctl/cli.Version=${VERSION}" sbctl.go

.PHONY: mod-tidy
mod-tidy:
	go mod tidy

# Install/upgrade ginkgo. This version must be the same as
# the one on go.mod. We'll rely on dependabot to upgrade go.mod
.PHONY: ginkgo
ginkgo:
	go install github.com/onsi/ginkgo/v2/ginkgo

.PHONY: test
test: fmt vet
	ginkgo -v ./tests/...
	go test -v ${BUILDFLAGS} ./pkg/... ./cli/...

.PHONY: fmt
fmt:
	go fmt ${BUILDPATHS}

.PHONY: vet
vet:
	go vet ${BUILDFLAGS} ${BUILDPATHS}

# Compile and install sbctl locally in you GOBIN path
.PHONY: install
install: build
	go install ${BUILDFLAGS} -ldflags "-X github.com/replicatedhq/sbctl/cli.Version=${VERSION}" sbctl.go

.PHONY: lint
lint:
ifeq (, $(shell which golangci-lint))
	$(error "Install golangci-lint by either running 'make install-golangci-lint' or by other means")
endif
	golangci-lint run --new -c .golangci.yaml --build-tags ${BUILDTAGS} ${BUILDPATHS}

.PHONY: lint-and-fix
lint-and-fix:
ifeq (, $(shell which golangci-lint))
	$(error "Install golangci-lint by either running 'make install-golangci-lint' or by other means")
endif
	golangci-lint run --new --fix -c .golangci.yaml --build-tags ${BUILDTAGS} ${BUILDPATHS}

# Not pinning to a particular version so as not to forget updating it
# If unrelated warnings arise due to linter updates or newly introduced linters
# we'll consider pinning.
.PHONY: install-golangci-lint
install-golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: scan
scan:
	trivy fs \
		--scanners vuln \
		--exit-code=1 \
		--severity="HIGH,CRITICAL" \
		--ignore-unfixed \
		./
