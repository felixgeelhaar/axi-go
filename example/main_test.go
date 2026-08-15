package main

import "testing"

// Smoke test so `go test -cover ./...` does not fail with
// "no such tool covdata" on this main package.
func TestExamplePackageBuilds(t *testing.T) {
	t.Log("example package is testable")
}
