# AGENTS.md

This file is for AI coding agents working on this repository.

## Project info

opencode-launcher is a small Go CLI that ships as the `oc` binary.

It provides a keyboard-first TUI to:

- pick an OpenCode project (from local OpenCode storage)
- optionally pick a previous session
- pick a model
- then exec `opencode` with the appropriate flags

The tool is intentionally simple: no network calls, no telemetry, no background services.

Repo layout:

- `cmd/oc/main.go`: CLI entry point; prints version; validates inputs; execs `opencode`
- `internal/opencodestorage/`: reads OpenCode storage JSON (projects + sessions)
- `internal/config/`: reads YAML config of available models
- `internal/tui/`: Bubble Tea UI
- `install.sh`: installer/upgrader that fetches GitHub Release assets
- `demo/`: fake OpenCode storage + config for screenshots/demo

## Tech stack

- Language: Go (see `go.mod`)
- TUI: Bubble Tea + Bubbles + Lip Gloss
- Config: YAML (`gopkg.in/yaml.v3`)
- Storage parsing: JSON (`encoding/json`)
- Releases: GitHub Actions + GitHub Releases

## Non-functional requirements

- Safety: do not read or embed user secrets; do not commit local state (OpenCode storage, `.opencode/`, `.env`, etc.)
- Deterministic UX: fast startup, stable keyboard-first navigation
- Minimal dependencies: keep the binary small; avoid CGO unless required
- Cross-platform: at least `darwin` and `linux` (amd64 + arm64) for release artifacts

## Install script (`install.sh`)

Purpose:

- Install or upgrade `oc` without cloning the repo
- Detect OS/arch; download the matching GitHub Release asset
- Verify download integrity using `checksums.txt` (sha256)
- Handle name collisions: default installs as `oc`; if another `oc` exists and is not opencode-launcher, prompt for a different `--name`

Notes for changes:

- Keep it POSIX-ish Bash (no external deps beyond `curl`, `tar`, `install`, and sha256 tool)
- Must remain safe to run via `bash -c "$(curl -fsSL ...)"`
- Do not add interactive prompts that break non-interactive installs; always provide a flag alternative

## Build + test

**Important** always build to the dist folder.

```bash
go test ./...
go build -o dist/oc ./cmd/oc
```

Versioning:

- `cmd/oc/main.go` uses `var version = "dev"`
- Release builds set it at build time via `-ldflags "-X main.version=vX.Y.Z"`

## Releasing

Release flow:

1. Ensure `main` is green
2. Tag a SemVer version and push it (e.g. `v1.0.2`)
3. GitHub Actions workflow `.github/workflows/release.yml`:
   - runs tests once on the runner
   - cross-compiles `darwin/linux` x `amd64/arm64` with `CGO_ENABLED=0`
   - creates `oc_<tag>_<goos>_<goarch>.tar.gz` archives
   - publishes GitHub Release assets + `checksums.txt`

Local release build example:

```bash
TAG="$(git describe --tags --always --dirty)"
go build -trimpath -ldflags "-s -w -X main.version=${TAG}" -o dist/oc ./cmd/oc
```

## Demo

The `demo/` directory contains fake OpenCode storage and a sample model config.

Run the demo:

```bash
go build -o dist/oc ./cmd/oc
OC_STORAGE_ROOT="$PWD/demo/opencode-storage" OC_CONFIG_PATH="$PWD/demo/oc-config.yaml" ./dist/oc
```

This is intended for:

- taking screenshots without exposing real projects
- reproducing UI behavior deterministically
