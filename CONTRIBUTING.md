# Contributing

Thanks for helping improve opencode-launcher.

## Development setup

Requirements:

- Go 1.22+
- OpenCode installed as `opencode` on your `PATH` (only needed to run the actual launcher)

Build and test:

```bash
go test ./...
go build -o dist/oc ./cmd/oc
```

## Demo mode

Use the included fake storage/config so you don't expose your personal projects:

```bash
go build -o dist/oc ./cmd/oc
OC_STORAGE_ROOT="$PWD/demo/opencode-storage" OC_CONFIG_PATH="$PWD/demo/oc-config.yaml" ./dist/oc
```

## Making changes

- Keep the tool simple: no network calls, no telemetry, no background processes.
- Prefer small, focused PRs.
- Update `README.md` if you change user-facing behavior.

## Reporting security issues

Please do not open public issues for security-sensitive problems.
Instead, contact the maintainer via GitHub (or open a private security advisory if enabled).
