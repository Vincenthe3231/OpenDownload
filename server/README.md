# OpenDownload server package

This directory contains a legacy duplicate of the Wails bridge and internal packages. The active application entry points are at the repository root:

- `main.go` and `app.go` for the Wails desktop app
- `cmd/cli/main.go` for the CLI
- `internal/` for the shared download engine

Use the root [README](../README.md) for current build instructions and the complete desktop, command line, and capture workflows. New changes should target the root packages unless this legacy copy is deliberately being migrated or removed.
