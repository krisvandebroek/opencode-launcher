package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"strings"

	"oc/internal/config"
	"oc/internal/opencodestorage"
	"oc/internal/tui"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("oc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	showHelp := fs.Bool("help", false, "show help")
	fs.BoolVar(showHelp, "h", false, "show help")
	showVersion := fs.Bool("version", false, "show version")
	fs.BoolVar(showVersion, "v", false, "show version")
	dryRun := fs.Bool("dry-run", false, "print opencode command and exit")

	storageRootFlag := fs.String("storage", "", "OpenCode storage root (default: ~/.local/share/opencode)")
	configPathFlag := fs.String("config", "", "Config path (default: ~/.config/oc/oc-config.yaml)")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "oc - speed-first OpenCode launcher")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Usage:")
		fmt.Fprintln(fs.Output(), "  oc            launch project picker")
		fmt.Fprintln(fs.Output(), "  oc --help     show this help")
		fmt.Fprintln(fs.Output(), "  oc --version  show version")
		fmt.Fprintln(fs.Output(), "  oc --storage <path>  override OpenCode storage root")
		fmt.Fprintln(fs.Output(), "  oc --config <path>   override model config path")
		fmt.Fprintln(fs.Output(), "  oc --dry-run         print opencode command, do not launch")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Data sources:")
		fmt.Fprintln(fs.Output(), "  OpenCode storage: ~/.local/share/opencode")
		fmt.Fprintln(fs.Output(), "  Model config:     ~/.config/oc/oc-config.yaml")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Environment overrides:")
		fmt.Fprintln(fs.Output(), "  OC_STORAGE_ROOT")
		fmt.Fprintln(fs.Output(), "  OC_CONFIG_PATH")
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

	if err := opencodestorage.CheckStorageReadable(storageRoot); err != nil {
		fmt.Fprintln(os.Stderr, "error: OpenCode storage missing/unreadable")
		fmt.Fprintf(os.Stderr, "  expected: %s\n", storageRoot)
		fmt.Fprintf(os.Stderr, "  detail:   %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Fix:")
		fmt.Fprintln(os.Stderr, "  - Install and run OpenCode once to initialize storage")
		fmt.Fprintln(os.Stderr, "  - Or create the directory and ensure it is readable")
		return 1
	}

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

	projects, err := opencodestorage.LoadProjects(storageRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load projects: %v\n", err)
		return 1
	}
	if len(projects) == 0 {
		fmt.Fprintf(os.Stderr, "error: no projects found in %s\n", filepath.Join(storageRoot, "storage", "project"))
		return 1
	}

	plan, err := tui.Run(tui.Input{
		StorageRoot: storageRoot,
		Projects:    projects,
		Models:      modelCfg.Models,
		DefaultModel: defaultModel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if plan == nil {
		return 0
	}
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

	if err := execOpencode(args2); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error: failed to start opencode: %v\n", err)
		return 1
	}

	return 0
}

func execOpencode(args []string) error {
	opencodePath, err := exec.LookPath("opencode")
	if err != nil {
		return err
	}
	// Prefer exec-style handoff so the user drops straight into OpenCode.
	if err := syscall.Exec(opencodePath, append([]string{"opencode"}, args...), os.Environ()); err != nil {
		// Fallback for environments where Exec isn't supported as expected.
		cmd := exec.Command(opencodePath, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Not reachable on success (process replaced).
	return nil
}
