# Tech Stack

## Runtime + Language
- **Language:** Go
- **Target platform:** macOS
- **Distribution:** single standalone `oc` binary (no external runtime required)

## CLI / TUI
- **UI style:** terminal TUI, keyboard-first, optimized for speed (defaults preselected; minimal steps)
- **TUI framework:** Charm Bubble Tea (`github.com/charmbracelet/bubbletea`)
- **Common UI components:** Bubbles (`github.com/charmbracelet/bubbles`) for lists/inputs
- **Styling:** Lip Gloss (`github.com/charmbracelet/lipgloss`) for lightweight terminal styling

## Configuration + Data Sources
- **Model list config:** YAML at `~/.config/oc/oc-config.yaml`
- **OpenCode data source:** `~/.local/share/opencode`
- **Projects:** `storage/project/{projectId}.json` (sorted by `updated`)
- **Sessions:** `storage/session/{projectId}/{sessionId}.json` (sorted by recency; display `title`, fallback `untitled`)

## Integrations
- **OpenCode execution:** invoke `opencode` with selected project directory, model, and session parameters (continue existing session or start new)

## Data Formats
- **JSON:** parse project/session metadata using Go standard library (`encoding/json`)
- **YAML:** parse model configuration using `gopkg.in/yaml.v3`

## Build, Test, and Release
- **Build:** `go build`
- **Tests:** `go test ./...`
- **Release packaging (optional):** GoReleaser for repeatable local builds and sharable artifacts
- **CI (optional):** GitHub Actions to build/test and attach release binaries
