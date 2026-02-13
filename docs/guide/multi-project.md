# Multi-Project Setup

Herald supports multiple projects, each with its own configuration, tool restrictions, and Git settings.

## Configuring Multiple Projects

```yaml
projects:
  my-api:
    path: "/home/user/projects/my-api"
    description: "Go backend API"
    default: true
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
      - "Bash(git *)"
      - "Bash(go *)"
      - "Bash(make *)"
    max_concurrent_tasks: 1
    git:
      auto_branch: true
      auto_stash: true
      auto_commit: true
      branch_prefix: "herald/"

  frontend:
    path: "/home/user/projects/frontend"
    description: "React frontend"
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
      - "Bash(npm *)"
      - "Bash(npx *)"
      - "Bash(git *)"
    max_concurrent_tasks: 2
    git:
      auto_branch: true
      branch_prefix: "herald/"

  docs:
    path: "/home/user/projects/docs"
    description: "Documentation site"
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
    max_concurrent_tasks: 1
    git:
      auto_branch: false
```

## Default Project

Mark one project as `default: true`. When you don't specify a project in your prompt, Herald uses the default.

> *"Add rate limiting to the API"*

This targets the default project (`my-api`).

> *"Add a dark mode toggle to the frontend project"*

Claude Chat passes `project: "frontend"` to `start_task`.

!!! tip
    If no project is marked as default, you must always specify which project to target.

## Per-Project Tool Restrictions

Each project defines exactly which Claude Code tools are available. This is your primary security control.

### Tool Format

```yaml
allowed_tools:
  - "Read"                # File reading
  - "Write"               # File creation
  - "Edit"                # File editing
  - "Bash(git *)"         # Git commands only
  - "Bash(go *)"          # Go commands only
  - "Bash(make *)"        # Make commands only
```

The `Bash(pattern)` syntax restricts shell commands to matching patterns. This prevents Claude Code from running arbitrary commands.

### Examples by Project Type

=== "Go Backend"

    ```yaml
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
      - "Bash(git *)"
      - "Bash(go *)"
      - "Bash(make *)"
    ```

=== "Node.js Frontend"

    ```yaml
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
      - "Bash(npm *)"
      - "Bash(npx *)"
      - "Bash(git *)"
    ```

=== "Read-Only / Docs"

    ```yaml
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
    ```

!!! warning "No blanket Bash access"
    Never use `"Bash"` without a pattern. Always restrict to specific command prefixes. Herald explicitly does not pass `--dangerously-skip-permissions` to Claude Code.

## Per-Project Concurrency

```yaml
max_concurrent_tasks: 1
```

Limits how many tasks can run simultaneously on this project. Useful for projects where concurrent changes would conflict. Additional tasks are queued.

The global `execution.max_concurrent` setting applies across all projects.

## Per-Project Git Settings

```yaml
git:
  auto_branch: true       # Create a new branch for each task (default: false)
  auto_stash: true        # Stash uncommitted changes before branching (default: false)
  auto_commit: true       # Commit changes when task completes (default: false)
  branch_prefix: "herald/"  # Branch naming: herald/{task-id}-{description}
```

| Setting | Default | Description |
|---|---|---|
| `auto_branch` | `false` | Create an isolated branch per task |
| `auto_stash` | `false` | Stash dirty working tree before branching |
| `auto_commit` | `false` | Auto-commit on task completion |
| `branch_prefix` | `"herald/"` | Prefix for generated branch names |

!!! tip "Disable Git integration for docs"
    For documentation or config projects where Git branching adds friction, set `auto_branch: false`.

## Listing Projects

From Claude Chat:

> *"What projects do I have configured?"*

This calls `list_projects` and shows all projects with their current Git status (branch, clean/dirty).
