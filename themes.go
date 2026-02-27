// Package blog provides the embedded theme files and version info for the Skriva blog engine.
// The themes directory is embedded into the binary at build time and used
// as a fallback when themes are not found in the content directory.
package blog

import "embed"

// Version is the current Skriva release version.
// Update this constant for every release.
const Version = "0.1.0"

// EmbeddedThemes holds the bundled theme files (templates, CSS, etc.).
// Access files with paths like "themes/classic/templates/base.html".
//
//go:embed themes/*
var EmbeddedThemes embed.FS
