BUILDFLAGS = -tags "netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp" -installsuffix netgo
BUILDPATHS = ./pkg/... ./cli/... ./tests/...

.PHONY: build
build:
	go build -o bin/sbctl sbctl.go

# Install/upgrade ginkgo. This version must be the same as
# the one on on go.mod. We'll rely on dependabot to upgrade go.mod
.PHONY: ginkgo
ginkgo:
	go install github.com/onsi/ginkgo/v2/ginkgo

.PHONY: test
test:
	ginkgo -v ./tests/...

.PHONY: fmt
fmt:
	go fmt ${BUILDPATHS}

.PHONY: vet
vet:
	go vet ${BUILDFLAGS} ${BUILDPATHS}

# Compile and install sbctl locally in you GOBIN path
.PHONY: install
install:
	go install sbctl.go
