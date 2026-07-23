// Package assets provides embedded static assets for the PROBE web UI.
// The //go:embed directive must be in a package at the repo root to reach web/dist/.
package assets

import "embed"

// FS contains the embedded web/dist/ directory (production React build).
//
//go:embed all:web/dist
var FS embed.FS