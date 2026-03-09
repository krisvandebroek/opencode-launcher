# Execution Plan for OpenCode Launch Directory Navigation

## Goal
Whenever a user opens an existing session or starts a new session in `oc`, the tool should change the working directory (`cd`) to the project directory before launching `opencode`. Crucially, when the user subsequently quits the `opencode` session, their shell should leave them in that project directory.

## Background limitation
A child process (like the `oc` Go binary) cannot inherently change the working directory of its parent process (the user's interactive shell like bash, zsh, etc.). `oc` already calls `os.Chdir(workDir)` before launching `opencode`, but when `opencode` terminates, the parent shell resumes at its original directory.

## Proposed Solution: The Subshell Approach
Because we want to keep `oc` zero-config and intentionally simple (avoiding shell hooks/eval wrappers that require user configuration), we will implement a transparent subshell approach in `cmd/oc/main.go`.

### Implementation Steps (`cmd/oc/main.go`)

1. **Capture Current Directory**
   Inside `execOpencode(workDir string, args []string)`, determine the original working directory using `os.Getwd()` before attempting to change it.

2. **Conditional Execution Logic**
   Compare the original directory to the target `workDir` after resolving any symlinks (using `filepath.EvalSymlinks` for accuracy).

3. **Scenario A: User is already in the project directory**
   - Retain the current behavior.
   - Use `syscall.Exec` to directly hand off execution from `oc` to `opencode`.
   - When the user quits `opencode`, they return to their original shell prompt which is already in the project directory.

4. **Scenario B: User launched `oc` from a different directory**
   - Call `os.Chdir(workDir)` to switch `oc` into the project directory.
   - Run `opencode` as a synchronous child process using `exec.Command(...).Run()`, attaching `os.Stdin`, `os.Stdout`, and `os.Stderr`.
   - Wait for `opencode` to complete.
   - Check the exit code of `opencode`. If it exited cleanly (e.g., exit code 0):
     - Fetch the user's preferred shell from the `SHELL` environment variable (falling back to `/bin/bash` or `/bin/sh`).
     - Use `syscall.Exec` to replace the `oc` process with an interactive session of their `$SHELL`.
   - If `opencode` exited with an error, propagate the error and exit without spawning a subshell, to avoid confusing behavior on failure.

### User Experience
When a user launches `oc` from `~/Desktop` and selects a project in `~/Dev/my-project`:
1. `opencode` launches successfully.
2. The user works inside `opencode` and eventually types `/exit`.
3. Instead of returning to `~/Desktop`, they are dropped into a new bash/zsh prompt in `~/Dev/my-project`.
4. When they are finished working in the project directory, typing `exit` once more will close the nested shell and return them to their original `~/Desktop` prompt.