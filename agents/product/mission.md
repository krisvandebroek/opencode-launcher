# Product Mission

## Pitch
`oc` is a keyboard-first terminal tool that helps experienced OpenCode users switch contexts fast
by letting them instantly choose the project, model, and session (or start a new session) with smart defaults.

## Users

### Primary Customers
- **Experienced developers using OpenCode across many repos:** frequent context switching, wants minimal friction.
- **Consultants / freelancers working across client projects:** jumps between multiple codebases and prefers predictable, fast workflows.

### User Personas
**Power user developer** (25-45)
- **Role:** Staff/principal engineer, consultant, or productivity-focused individual contributor
- **Context:** Works in multiple projects daily; uses OpenCode with different models depending on task constraints
- **Pain Points:** Manual switching of project/model/session; too many steps to resume work; slow selection breaks flow
- **Goals:** Resume the right context in seconds; rely on defaults most of the time; stay fully on keyboard

## The Problem

### Slow, manual context switching
Using OpenCode across many projects and models requires repeated, manual setup steps.
Even small delays compound when switching contexts many times per day, breaking concentration.

**Our Solution:** Provide an ultra-fast, keyboard-driven selection flow with recency-based ordering and sensible defaults so the user can launch OpenCode in the right context immediately.

## Differentiators

### Speed-first, keyboard-first UX
Unlike a generic launcher or multi-step wizard, `oc` optimizes for the common case: a default model and “new session” are preselected, and the user only changes what they need.
This results in fewer keystrokes and faster time-to-first-prompt.

### Context-aware ordering
Unlike static lists or alphabetic sorting, `oc` orders projects and sessions by recent activity.
This results in the right project/session being near the top most of the time.

### Easy to share and run
Unlike tools that require a runtime environment or complex setup, `oc` aims to be a standalone, shippable artifact.
This results in low installation friction and easy sharing with other developers.

## Key Features

### Core Features
- **Project picker (recency-sorted):** Quickly choose the right project first, without scrolling through stale entries.
- **Model picker (fixed list + default):** Pick the right model for the task, with a curated list and predictable ordering.
- **Session picker (continue or new):** Resume prior work instantly or start fresh, with sessions ordered by most recent.

### Collaboration Features
- **Portable distribution:** Share `oc` as a single artifact so teammates can adopt the same fast workflow with minimal setup.

### Advanced Features
- **Keyboard navigation + smart defaults:** Use the tool entirely from the keyboard; launch immediately using defaults when no changes are needed.
