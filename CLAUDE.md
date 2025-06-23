# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

sbctl is a command-line tool for examining Kubernetes resources in Troubleshoot's support bundles. It creates a local API server that mimics a Kubernetes API, allowing users to run kubectl commands against static support bundle data.

### Key Architecture Components

1. **CLI Layer** (`cli/`): Cobra-based commands (serve, shell, download)
2. **API Server** (`pkg/api/server.go`): HTTP server that implements Kubernetes API endpoints
3. **Support Bundle Processing** (`pkg/sbctl/`): Extracts and processes support bundle archives
4. **Kubernetes Objects** (`pkg/k8s/`): Helper functions for K8s resource types
5. **Utilities** (`pkg/util/`): Common utility functions

### Core Workflow

1. User provides a support bundle (local file or URL)
2. sbctl extracts the bundle and creates a `ClusterData` structure
3. A local API server starts that serves K8s resources from the bundle data
4. A temporary kubeconfig is generated pointing to the local server
5. Users can run kubectl commands against this temporary cluster

## Development Commands

Build the project:
```bash
make build
```

Run tests:
```bash
make test
```

Run linting:
```bash
make lint
```

Install ginkgo (required for tests):
```bash
make ginkgo
```

Format code:
```bash
make fmt
```

Vet code:
```bash
make vet
```

Run security scan:
```bash
make scan
```

### Test Structure

- Tests use Ginkgo/Gomega framework
- Integration tests in `tests/` directory start a real API server
- Test data includes sample support bundle structure in `tests/support-bundle/`
- Run single test: `ginkgo -v ./tests/... --focus="specific test"`

## Code Conventions

- Go 1.23.4 with modules
- Uses standard Go project layout
- Cobra for CLI commands with viper for configuration
- Gorilla mux for HTTP routing
- Logrus for structured logging
- Environment variables prefixed with `SBCTL_`

### Key Files

- `sbctl.go`: Main entry point
- `cli/root.go`: CLI command structure
- `pkg/api/server.go`: Core API server implementation (1700+ lines)
- `pkg/sbctl/support-bundle.go`: Bundle extraction logic
- `Makefile`: Build and development commands

### Linting Configuration

Uses golangci-lint with these enabled linters:
- gocritic
- gofmt  
- gosec
- govet

### Dependencies

Core dependencies include:
- Kubernetes client libraries (k8s.io/*)
- Cobra/Viper for CLI
- Gorilla for HTTP handling
- Logrus for logging
- Ginkgo/Gomega for testing

The project mimics a real Kubernetes API server, so it imports many k8s.io packages for type definitions and conversions.