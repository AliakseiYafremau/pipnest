# Project Context

This project is a terminal UI (TUI) package manager for Python.

Its purpose is to provide a convenient text-based interface for managing:
- virtual environments
- installed packages
- package search and installation
- switching between package management backends

## Product Goal

This is NOT a new Python package manager with its own dependency resolver.

This project is a TUI orchestration layer over existing tools such as:
- pip
- uv

The application must provide a unified user experience while delegating actual package management operations to backend implementations.

## Refactoring Status

This project is currently being refactored.

Legacy folders that are considered old and are not planned for future use:
- `internal/cheatsheet`
- `internal/packages`
- `internal/venvs`

New and updated functionality should be implemented in the new architecture layers, not in the legacy folders above.

## Core Product Principles

- The TUI is the main product.
- Package installation logic must be delegated to pluggable backends.
- Do not implement a custom dependency resolver.
- Do not tightly couple UI logic to pip or uv.
- The architecture must support adding new backends later.
- The system should work even if only one backend is available.
- Prefer clear abstractions over tool-specific shortcuts.

## Architecture

The project should follow a layered architecture:

1. Domain layer
   - shared models
   - backend interfaces / protocols
   - domain errors
   - backend-agnostic use cases

2. Backend layer
   - PipBackend
   - UvBackend
   - optional future CustomBackend

3. Service layer
   - orchestration logic
   - backend selection
   - environment discovery
   - package operations
   - command execution wrappers

4. TUI layer
   - screens
   - widgets
   - state management
   - keybindings
   - event handling

## Main Architectural Rule

The UI must never directly call pip or uv commands.

The UI should only interact with service objects or backend interfaces.

Bad:
- TUI screen runs `pip install ...` directly

Good:
- TUI triggers `package_service.install(...)`
- service delegates to selected backend

## Service layer API policy

The `internal/service` package is the application orchestration layer and must remain a stable, backend-agnostic surface for the rest of the codebase.

- Do NOT change `internal/service` to add shortcuts or API surface that exists solely for UI convenience.
- UI-specific APIs, adapters, view models and presentation logic must live in the `ui` layer. If the UI needs a different shape of data, implement an adapter in `internal/ui` (or in the `ui` package) rather than mutating service contracts.
- Service may expose backend-agnostic use-cases (install, uninstall, list, show, freeze). Any UI-driven helpers should be implemented on top of these use-cases in the `ui` layer.

If you feel service must change for a valid cross-cutting reason, first discuss the change in design or a pull request description and prefer additive, backward-compatible changes.


## Backend Model

Each backend should implement a common interface for operations such as:
- create virtual environment
- delete virtual environment
- list installed packages
- install package
- uninstall package
- show package details
- export/freeze dependencies

Backends may also expose optional capabilities, such as:
- Search packages
- Show package metadata

These advanced capabilities must be treated as optional, not universal.

## pyproject.toml Policy

For the MVP, the project is primarily environment-oriented, not project-file-oriented.

This means:
- focus on venv and installed packages first
- do not make pyproject.toml the central source of truth
- support pyproject.toml later as an optional feature for backends that can use it

Important:
- pip is treated as an environment/package installer backend
- uv may support more project-aware workflows
- those differences should be represented through backend capabilities

## Initial Scope (MVP)

The first version should support:
- detecting Python interpreters
- creating virtual environments
- selecting an environment
- listing installed packages
- installing a package
- uninstalling a package
- searching packages
- switching backend between pip and uv
- showing operation output and errors clearly

## Out of Scope for MVP

Do not introduce these unless explicitly requested:
- custom dependency resolution
- full Poetry-like project management
- lockfile design from scratch
- custom package index implementation
- automatic mutation of pyproject.toml
- hidden backend-specific magic

## UX Principles

The UI should feel similar in clarity to tools like lazygit:
- fast navigation
- clear panes
- explicit actions
- visible active backend
- visible active environment
- easy refresh
- searchable package list
- installation popup/dialog with search suggestions

The user should always be able to see:
- current backend
- selected environment
- current action
- success or failure output

## Code Style Expectations

- Write modular Go code.
- Prefer interfaces, structs and implementations through DI.
- Keep subprocess execution isolated in dedicated infrastructure code.
- Separate domain logic from terminal rendering logic.
- Avoid large god classes.
- Keep functions small and testable.
- Design for future backend extensibility.
- Make tests for domain logic independent of specific backends.
- Use clear error handling and propagation.

## Implementation Preference

Default backend strategy:
- use uv if available
- otherwise fall back to pip

However, backend choice must remain user-configurable.

## What the assistant should optimize for

When generating code or suggestions:
- preserve backend abstraction
- preserve separation of concerns
- prioritize maintainability
- avoid overengineering
- avoid introducing project-wide assumptions not stated here
- keep MVP practical and incremental

## Terminology

- "backend" = a package management provider such as pip or uv
- "environment" = a virtual environment / interpreter target
- "package operation" = install, uninstall, list, show, freeze, etc.
- "project mode" = optional future mode for pyproject-aware workflows
- "MVP" = environment-first TUI package manager