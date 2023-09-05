BUILDFLAGS = -tags "netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp" -installsuffix netgo
BUILDPATHS = ./pkg/... ./cli/... ./tests/...

.PHONY: build
build:
	go build -o bin/sbctl sbctl.go

ginkgo:
	go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@v2.1.3
	go get github.com/onsi/gomega/...@v1.19.0

test:
	ginkgo -v ./tests/...

.PHONY: fmt
fmt:
	go fmt ${BUILDPATHS}

.PHONY: vet
vet:
	go vet ${BUILDFLAGS} ${BUILDPATHS}