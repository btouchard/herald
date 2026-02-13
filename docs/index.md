# Herald

**Code from your phone. Seriously.**

Herald is a self-hosted MCP server that bridges Claude Chat to Claude Code using Anthropic's official [Custom Connectors](https://support.claude.com/en/articles/11503834-building-custom-connectors-via-remote-mcp-servers) protocol. One Go binary. Zero hacks.

---

You're on the couch. On your phone. You open Claude Chat and type:

> *"Refactor the auth middleware in my-api to use JWT instead of session cookies. Run the tests."*

Four minutes later, it's done. Branch created, code refactored, tests passing, changes committed. Your workstation did all the work. You never opened your laptop.

**That's Herald.**

## How It Works

```
  You (phone/tablet/browser)
       │
       │  "Add rate limiting to the API"
       ▼
  Claude Chat ──── MCP over HTTPS ────► Herald (your workstation)
                                           │
                                           ▼
                                        Claude Code
                                           ├── reads your codebase
                                           ├── writes the code
                                           ├── runs the tests
                                           └── commits to a branch

  You (terminal)
       │
       │  Claude Code calls herald_push
       ▼
  Claude Code ──── MCP ────► Herald ────► Claude Chat picks it up
                                           └── session context, summary,
                                               files modified, git branch
```

The bridge is **bidirectional**. Claude Chat dispatches tasks to Claude Code, and Claude Code can push session context back to Herald for remote monitoring and continuation from another device.

Your code never leaves your machine. Herald just orchestrates.

## Features

### Core

- **Native MCP bridge** — Uses Anthropic's official Custom Connectors protocol. Not a hack, not a wrapper, not a proxy.
- **Async task execution** — Start tasks, check progress, get results. Claude Code runs in the background while you do other things.
- **Git branch isolation** — Each task runs on its own branch. Your main branch stays untouched.
- **Session resumption** — Multi-turn Claude Code conversations. Pick up where you left off.
- **Bidirectional bridge** — Claude Code can push session context to Herald via `herald_push` for remote continuation.

### Multi-Project

- **Multiple projects** — Configure as many projects as you need, each with its own settings.
- **Per-project tool restrictions** — Control exactly which tools Claude Code can use. Full sandboxing per project.

### Operations

- **MCP push notifications** — Herald pushes task updates directly to Claude Chat via MCP server notifications. No polling needed.
- **SQLite persistence** — Tasks survive server restarts. Full history, fully searchable.

### Engineering

- **Single binary** — One Go executable, ~15MB. No Docker, no runtime, no node_modules.
- **Zero CGO** — Pure Go. Cross-compiles to Linux, macOS, Windows, ARM.
- **6 dependencies** — chi, mcp-go, modernc/sqlite, uuid, yaml, testify. That's the entire dependency tree.

## Why Herald?

| | Herald | Copy-paste workflow | Other tools |
|---|---|---|---|
| **Official protocol** | MCP Custom Connectors | N/A | Custom APIs, fragile |
| **Your code stays local** | Always | Yes | Depends |
| **Works from phone** | Native | No | Rarely |
| **Self-hosted** | 100% | N/A | Often SaaS |
| **Dependencies** | 6 | N/A | 50-200+ |
| **Setup time** | ~5 minutes | N/A | 30min+ |
| **CGO required** | No | N/A | Often |

Herald uses the same protocol Anthropic built for their own integrations. No reverse engineering, no unofficial APIs, no hacks that break on the next update.

## Quick Start

```bash
# Build
git clone https://github.com/kolapsis/herald.git
cd herald && make build

# Configure
mkdir -p ~/.config/herald
cp configs/herald.example.yaml ~/.config/herald/herald.yaml

# Run (client secret is auto-generated on first start)
./bin/herald serve
```

Then [connect from Claude Chat](getting-started/connecting.md) and start coding from anywhere.

## Next Steps

- [Installation](getting-started/installation.md) — Prerequisites and build instructions
- [Configuration](getting-started/configuration.md) — Full `herald.yaml` reference
- [Connecting](getting-started/connecting.md) — Hook up Claude Chat to Herald
- [Workflow](guide/workflow.md) — The typical start → check → result loop
- [Tools Reference](guide/tools-reference.md) — All 10 MCP tools in detail
