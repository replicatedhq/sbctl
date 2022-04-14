.PHONY: build
build:
	go build -o bin/sbctl sbctl.go

ginkgo:
	go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@v2.1.3
	go get github.com/onsi/gomega/...@v1.19.0

test:
	ginkgo -v ./tests/...