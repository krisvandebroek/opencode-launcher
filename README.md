# oc

`oc` is a speed-first launcher for OpenCode.

It reads your OpenCode project/session metadata, lets you pick a project and session with a keyboard-first TUI, and then starts `opencode` with the right flags.

## What it does

- Loads projects from `~/.local/share/opencode/storage/project/*.json` (sorted by most-recent update).
- Loads sessions for the selected project from `~/.local/share/opencode/storage/session/{projectId}/*.json` (sorted by most-recent update).
- Loads available models from `~/.config/oc/oc-config.yaml`.
- Launches OpenCode by executing `opencode <projectDir> --model <provider/model> [--session <sessionId>]`.

## Requirements

- macOS
- Go (via Homebrew or the official installer)
- OpenCode installed and available as `opencode` on your `PATH`
- OpenCode storage initialized (run OpenCode once so `~/.local/share/opencode` exists)

## Configuration

Create `~/.config/oc/oc-config.yaml`:

```yaml
default_model: GPT-5.2
models:
  - name: Gemini Pro
    model: google/gemini-3-pro-preview
  - name: GPT-5.2
    model: openai/gpt-5.2
  - name: Gemini Flash
    model: google/gemini-flash
  - name: GPT-5.1
    model: openai/gpt-5.1
```

Notes:
- `models` order is the order shown in the UI.
- `default_model` matches by `name` (case-insensitive). If omitted, the first model is used.

## Build

From the repo root:

```bash
go test ./...
go build -o dist/oc ./cmd/oc
```

## Run

```bash
./dist/oc
```

Useful flags:

- `--dry-run`: print the `opencode ...` command instead of launching.
- `--storage <path>` or `OC_STORAGE_ROOT=<path>`: override the OpenCode storage root.
- `--config <path>` or `OC_CONFIG_PATH=<path>`: override the config path.

TUI layout tuning:

- `OC_TUI_SAFETY_SLACK=<n>`: subtract `<n>` extra columns from the layout budget.
  This can help in terminals that crop the rightmost border (Ghostty often needs `7`).

## Keybindings

- `tab` / `shift+tab`: move between columns
- `up` / `down`: move selection
- type: filter Projects/Sessions
- `enter`: launch
- `ctrl+c`: quit
