package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"oc/internal/config"
	"oc/internal/opencodestorage"
	"oc/internal/tui"
)

var version = "dev"

const installScriptURL = "https://raw.githubusercontent.com/krisvandebroek/opencode-launcher/main/install.sh"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 && args[0] == "upgrade" {
		return runUpgrade(args[1:])
	}

	fs := flag.NewFlagSet("oc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	showHelp := fs.Bool("help", false, "show help")
	fs.BoolVar(showHelp, "h", false, "show help")
	showVersion := fs.Bool("version", false, "show version")
	fs.BoolVar(showVersion, "v", false, "show version")
	dryRun := fs.Bool("dry-run", false, "print opencode command and exit")
	legacyFlag := fs.Bool("legacy", false, "also read legacy JSON storage (storage/**) and merge with SQLite")

	storageRootFlag := fs.String("storage", "", "OpenCode storage root (default: ~/.local/share/opencode)")
	configPathFlag := fs.String("config", "", "Config path (default: ~/.config/oc/oc-config.yaml)")
	dbPathFlag := fs.String("db", "", "OpenCode database path (default: <storageRoot>/opencode.db)")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "oc - speed-first OpenCode launcher")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Usage:")
		fmt.Fprintln(fs.Output(), "  oc            launch project picker")
		fmt.Fprintln(fs.Output(), "  oc upgrade    upgrade oc via install script")
		fmt.Fprintln(fs.Output(), "  oc --help     show this help")
		fmt.Fprintln(fs.Output(), "  oc --version  show version")
		fmt.Fprintln(fs.Output(), "  oc --storage <path>  override OpenCode storage root")
		fmt.Fprintln(fs.Output(), "  oc --config <path>   override model config path")
		fmt.Fprintln(fs.Output(), "  oc --db <path>       override OpenCode SQLite database path")
		fmt.Fprintln(fs.Output(), "  oc --legacy          also read legacy JSON storage (storage/**)")
		fmt.Fprintln(fs.Output(), "  oc --dry-run         print opencode command, do not launch")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Data sources:")
		fmt.Fprintln(fs.Output(), "  OpenCode storage: ~/.local/share/opencode")
		fmt.Fprintln(fs.Output(), "  Model config:     ~/.config/oc/oc-config.yaml")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Environment overrides:")
		fmt.Fprintln(fs.Output(), "  OC_STORAGE_ROOT")
		fmt.Fprintln(fs.Output(), "  OC_CONFIG_PATH")
		fmt.Fprintln(fs.Output(), "  OC_DB_PATH")
		fmt.Fprintln(fs.Output(), "  OC_DISABLE_SQLITE=1")
	}

	if err := fs.Parse(args); err != nil {
		// flag package already prints a useful error.
		return 2
	}

	if *showHelp {
		fs.Usage()
		return 0
	}
	if *showVersion {
		fmt.Fprintf(os.Stdout, "oc %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine home directory: %v\n", err)
		return 1
	}

	storageRoot := strings.TrimSpace(os.Getenv("OC_STORAGE_ROOT"))
	if storageRoot == "" {
		storageRoot = strings.TrimSpace(*storageRootFlag)
	}
	if storageRoot == "" {
		storageRoot = filepath.Join(home, ".local", "share", "opencode")
	}

	configPath := strings.TrimSpace(os.Getenv("OC_CONFIG_PATH"))
	if configPath == "" {
		configPath = strings.TrimSpace(*configPathFlag)
	}
	if configPath == "" {
		configPath = filepath.Join(home, ".config", "oc", "oc-config.yaml")
	}

	useLegacy := *legacyFlag
	disableSQLite := strings.TrimSpace(os.Getenv("OC_DISABLE_SQLITE")) == "1"
	dbPath := strings.TrimSpace(os.Getenv("OC_DB_PATH"))
	if dbPath == "" {
		dbPath = strings.TrimSpace(*dbPathFlag)
	}
	if dbPath == "" {
		dbPath = filepath.Join(storageRoot, "opencode.db")
	}

	if err := opencodestorage.CheckStorageReadable(storageRoot, dbPath, useLegacy, disableSQLite); err != nil {
		fmt.Fprintln(os.Stderr, "error: OpenCode storage missing/unreadable")
		fmt.Fprintf(os.Stderr, "  storage:  %s\n", storageRoot)
		fmt.Fprintf(os.Stderr, "  db:       %s\n", dbPath)
		fmt.Fprintf(os.Stderr, "  legacy:   %v\n", useLegacy)
		fmt.Fprintf(os.Stderr, "  sqlite:   %v\n", !disableSQLite)
		fmt.Fprintf(os.Stderr, "  detail:   %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Fix:")
		fmt.Fprintln(os.Stderr, "  - Install and run OpenCode once to initialize storage")
		fmt.Fprintln(os.Stderr, "  - Or create the directory and ensure it is readable")
		return 1
	}

	store, err := opencodestorage.OpenStore(opencodestorage.OpenOptions{
		StorageRoot:   storageRoot,
		DBPath:        dbPath,
		UseLegacy:     useLegacy,
		DisableSQLite: disableSQLite,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to open storage: %v\n", err)
		return 1
	}
	defer func() {
		// NOTE: if we exec into opencode, defers don't run; we'll also close
		// explicitly after the TUI returns.
		_ = store.Close()
	}()

	modelCfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: model config missing/unreadable")
		fmt.Fprintf(os.Stderr, "  expected: %s\n", configPath)
		fmt.Fprintf(os.Stderr, "  detail:   %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Create it with something like:")
		fmt.Fprintln(os.Stderr, strings.TrimSpace(config.MinimalExampleYAML()))
		return 1
	}
	defaultModel, err := modelCfg.Default()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: invalid model config")
		fmt.Fprintf(os.Stderr, "  detail: %v\n", err)
		return 1
	}

	projects, err := store.Projects(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load projects: %v\n", err)
		return 1
	}
	if len(projects) == 0 {
		fmt.Fprintln(os.Stderr, "error: no projects found (JSON or SQLite)")
		return 1
	}

	plan, err := tui.Run(tui.Input{
		Store:                    store,
		Projects:                 projects,
		Models:                   modelCfg.Models,
		DefaultModel:             defaultModel,
		HideGlobalProjects:       modelCfg.UI.HideGlobalProjects,
		GlobalSessionsMaxAgeDays: modelCfg.UI.GlobalSessionsMaxAgeDays,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if plan == nil {
		return 0
	}
	// Close DB handles before exec'ing into opencode.
	_ = store.Close()
	if *dryRun {
		// Print a shell-friendly line (quote values, not flags).
		fmt.Fprintf(os.Stdout, "opencode %q --model %q", plan.ProjectDir, plan.Model.Model)
		if plan.SessionID != "" {
			fmt.Fprintf(os.Stdout, " --session %q", plan.SessionID)
		}
		fmt.Fprintln(os.Stdout)
		return 0
	}

	if _, err := exec.LookPath("opencode"); err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot find 'opencode' in PATH")
		fmt.Fprintln(os.Stderr, "Fix: install OpenCode so the 'opencode' binary is available")
		return 1
	}

	args2 := []string{plan.ProjectDir, "--model", plan.Model.Model}
	if plan.SessionID != "" {
		args2 = append(args2, "--session", plan.SessionID)
	}

	if err := execOpencode(plan.ProjectDir, args2); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error: failed to start opencode: %v\n", err)
		return 1
	}

	return 0
}

func runUpgrade(args []string) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(os.Stderr, "oc upgrade - upgrade oc via install script")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Usage:")
			fmt.Fprintln(os.Stderr, "  oc upgrade")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Notes:")
			fmt.Fprintf(os.Stderr, "  - Runs: bash -c \"$(curl -fsSL %s)\"\n", installScriptURL)
			fmt.Fprintln(os.Stderr, "  - macOS/Linux only")
			return 0
		}
	}

	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "error: oc upgrade does not accept arguments")
		fmt.Fprintln(os.Stderr, "If you need installer flags, run the install.sh command from the README.")
		return 2
	}

	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "error: oc upgrade is not supported on Windows")
		fmt.Fprintln(os.Stderr, "Run the install.sh command from the README instead.")
		return 1
	}

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot find 'bash' in PATH")
		fmt.Fprintln(os.Stderr, "Fix: install bash or run the README install command manually")
		return 1
	}
	if _, err := exec.LookPath("curl"); err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot find 'curl' in PATH")
		fmt.Fprintln(os.Stderr, "Fix: install curl or run the README install command manually")
		return 1
	}

	// Match the README install line.
	cmdStr := fmt.Sprintf("bash -c \"$(curl -fsSL %s)\"", installScriptURL)
	cmd := exec.Command(bashPath, "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error: upgrade failed: %v\n", err)
		return 1
	}
	return 0
}

func execOpencode(workDir string, args []string) error {
	opencodePath, err := exec.LookPath("opencode")
	if err != nil {
		return err
	}

	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to chdir to %s: %w", workDir, err)
	}

	// Prefer exec-style handoff so the user drops straight into OpenCode.
	if err := syscall.Exec(opencodePath, append([]string{"opencode"}, args...), os.Environ()); err != nil {
		// Fallback for environments where Exec isn't supported as expected.
		cmd := exec.Command(opencodePath, args...)
		cmd.Dir = workDir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Not reachable on success (process replaced).
	return nil
}
