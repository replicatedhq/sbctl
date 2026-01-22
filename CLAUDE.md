# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

sbctl is a command-line tool that creates a local Kubernetes API server from Troubleshoot support bundles, allowing `kubectl` commands to be run against captured cluster state as if it were a live cluster. This enables debugging and investigation of Kubernetes clusters from support bundles without needing access to the actual cluster.

## Related Repositories

### Troubleshoot (`~/go/src/github.com/replicatedhq/troubleshoot`)

sbctl is a companion tool to [Replicated Troubleshoot](https://troubleshoot.sh/), which is the upstream project that **creates** the support bundles that sbctl **consumes**.

**Relationship**:
- **Troubleshoot** provides `kubectl support-bundle` command that collects cluster resources, logs, and diagnostic information into tar.gz archives
- **sbctl** reads those support bundle archives and presents them through a local Kubernetes API server for kubectl querying

**Key Troubleshoot packages relevant to sbctl**:
- `pkg/collect/` - Collectors that gather cluster resources (the data sbctl serves)
- `pkg/supportbundle/` - Support bundle creation and structure (defines the format sbctl reads)
- `pkg/analyze/` - Analyzers that inspect collected data (complementary to sbctl's query approach)

**Support Bundle Structure** (defined by Troubleshoot):
- `cluster-resources/` directory contains JSON files for K8s resources
- `cluster-info/cluster_version.json` contains cluster version information
- Resources are organized by type and namespace following Troubleshoot's collection patterns

**When to reference Troubleshoot**:
- Understanding support bundle file structure and naming conventions
- Adding support for new resource types that Troubleshoot collects
- Debugging why certain resources aren't appearing in sbctl (may not be collected by Troubleshoot)
- Contributing collectors to Troubleshoot to enable sbctl access to new resource types

## Build and Development Commands

### Building
```bash
make build                    # Build the binary to bin/sbctl
make install                  # Build and install to $GOBIN
VERSION=v1.2.3 make build     # Build with specific version
```

### Testing
```bash
make test                     # Run all tests (both ginkgo and go test)
ginkgo -v ./tests/...         # Run integration tests with ginkgo directly
go test -v ./pkg/... ./cli/... # Run unit tests directly
```

To run a single test file:
```bash
ginkgo -v ./tests/ -focus="ConfigMaps"  # Run specific test by name
go test -v ./pkg/api/...                # Run tests in specific package
```

### Code Quality
```bash
make fmt                      # Format code with gofmt
make vet                      # Run go vet with build tags
make lint                     # Run golangci-lint (requires installation)
make lint-and-fix             # Run golangci-lint with auto-fix
make install-golangci-lint    # Install golangci-lint
make scan                     # Run trivy security scan
```

### Dependency Management
```bash
make mod-tidy                 # Run go mod tidy
make ginkgo                   # Install/upgrade ginkgo test runner
```

## Architecture

### Core Components

**CLI Layer** (`cli/`)
- `root.go` - Cobra command initialization and viper configuration
- `serve.go` - Starts the API server in foreground mode, prints KUBECONFIG export command
- `shell.go` - Starts API server and launches an interactive shell with KUBECONFIG preset
- `download.go` - Downloads support bundles from vendor portal URLs with authentication
- `version.go` - Version information

**API Server** (`pkg/api/`)
- `server.go` - HTTP server implementing Kubernetes API endpoints using gorilla/mux
- Serves cluster resources from extracted support bundle files as if they were live K8s resources
- Implements both `/api/v1` (core) and `/apis/{group}/{version}` (extensions) endpoints
- Converts resources to Table format when requested (for `kubectl get` output)
- Handles field and label selectors for filtering resources
- Creates temporary kubeconfig file pointing to local server

**Support Bundle Handling** (`pkg/sbctl/`)
- `support-bundle.go` - Extracts tar.gz support bundles, finds cluster-resources directory
- `compatibility.go` - Maps resource names to support bundle file naming conventions
- Handles both directory and file-based cluster resource storage

**K8s Utilities** (`pkg/k8s/`)
- `objects.go` - Helper functions for creating empty typed K8s resource lists

**Utilities** (`pkg/util/`)
- `support-bundle.go` - Resource name compatibility mapping for support bundle file structure

### Key Architectural Patterns

1. **Local API Server Emulation**: The tool creates a local HTTP server that mimics Kubernetes API endpoints, reading from extracted support bundle files instead of etcd.

2. **Support Bundle Structure**: Support bundles contain a `cluster-resources/` directory with:
   - Single JSON files for cluster-scoped resources (e.g., `namespaces.json`, `nodes.json`)
   - Directories for namespaced resources with per-namespace JSON files (e.g., `pods/default.json`)
   - Custom resources in `custom-resources/{resource}.{group}/` subdirectories

3. **Resource Decoding**: The `sbctl.Decode()` function handles converting raw JSON from support bundles into typed K8s objects, supporting both typed resources (corev1, appsv1) and unstructured resources.

4. **Table Conversion**: When kubectl requests table format (via Accept headers), resources are converted using K8s internal printers to match live cluster output format.

5. **Temporary kubeconfig**: Each invocation creates a temporary kubeconfig file pointing to the local API server, allowing standard kubectl commands to work.

## Testing Strategy

- **Integration tests** (`tests/`) use Ginkgo/Gomega and test against actual support bundle fixtures
- Tests start a real API server against the test support bundle
- Custom matcher `Similar()` uses Jaro string similarity (>0.80 threshold) for flexible output matching
- Test support bundle is checked into `tests/support-bundle/` directory

## Build Tags

The project uses specialized build tags defined in the Makefile:
```
netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp
```

These tags must be included when running `go build`, `go test`, or `go vet` commands directly (not via Makefile).

## Common Workflows

### Testing Changes to API Endpoints
1. Modify handler in `pkg/api/server.go`
2. Run `make test` to verify integration tests pass
3. Manually test with: `go run sbctl.go shell tests/support-bundle`

### Adding Support for New Resource Types
1. Add type handling in `pkg/api/server.go` switch statements (e.g., `getAPIV1ClusterResources`, `getAPIsClusterResources`)
2. Add empty list helper in `pkg/k8s/objects.go` if needed
3. Add table conversion case in `toTable()` function if typed resource
4. Add integration test in `tests/`

### Debugging API Server Responses
- Use `--debug` flag to enable verbose logging including HTTP response bodies
- API server logs are written to stderr or a temp file (when using shell command)
- Check `log.Printf()` statements in handlers for request/response GVK information
