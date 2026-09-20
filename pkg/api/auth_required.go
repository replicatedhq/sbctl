//go:build auth_required
// +build auth_required

package api

// AuthRequired indicates whether authentication is required at compile time.
// This file is included when the auth_required build tag IS set.
// Build with: go build -tags auth_required
const AuthRequired = true
