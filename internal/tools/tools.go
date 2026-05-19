//go:build tools

// Package tools anchors module dependencies that are required by the project
// but not yet imported by any production code. Without this anchor, `go mod
// tidy` would strip these modules from go.mod.
//
// This file is excluded from regular builds via the `tools` build tag.
package tools

import (
	// Pulse is the upstream data layer that Prism visualizes. It is pinned
	// here in P00 so that future packages (P01+) can depend on a known
	// version without a separate `go get` step.
	_ "github.com/frankbardon/pulse"
)
