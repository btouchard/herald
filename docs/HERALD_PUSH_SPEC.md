# Herald Push — Liaison bidirectionnelle via MCP

> Phase 1bis du plan d'actions — complète la vision Herald
> Zéro wrapper, zéro dépendance, pur MCP

---

## Le problème

Herald est unidirectionnel : Chat → Herald → Code. Si le dev bosse directement dans Claude Code en terminal, Herald ne sait pas que la session existe. Le dev quitte son bureau, sort son téléphone — rien.

## L'insight

Herald est un serveur MCP. Claude Code est un client MCP natif. Le pont existe déjà.

On ajoute Herald comme serveur MCP dans la config Claude Code. Claude Code gagne un outil `herald_push`. Le dev dit "push ton contexte à Herald" et le handoff est fait. Zéro wrapper, zéro PTY, zéro nouvelle dépendance.

## Le flow

```
Terminal (bureau)                     Téléphone (canapé)

1. claude
   (session normale)

2. "push ton contexte à Herald"
   → Claude Code appelle
     herald_push

3. Herald enregistre la session
   comme tâche "linked"

4. Tu quittes ton bureau

                                      5. Claude Chat → list_tasks
                                         "Session linked il y a 3min"

                                      6. check_task → résumé complet

                                      7. start_task(session_id=...)
                                         → --resume, continue le travail
```

## Configuration Claude Code

```json
// ~/.claude/settings.json
{
  "mcpServers": {
    "herald": {
      "type": "url",
      "url": "https://herald.home.example.com/mcp"
    }
  }
}
```

C'est tout. Claude Code voit les outils Herald. Le dev peut dire "push à Herald" à n'importe quel moment.

## Nouvel outil MCP : `herald_push`

```json
{
  "name": "herald_push",
  "description": "Push the current Claude Code session context to Herald for remote monitoring and continuation. Call this when the user wants to continue working from another device.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "session_id": {
        "type": "string",
        "description": "Current Claude Code session ID"
      },
      "project": {
        "type": "string",
        "description": "Project name or working directory path"
      },
      "summary": {
        "type": "string",
        "description": "Summary of what has been done in this session so far"
      },
      "files_modified": {
        "type": "array",
        "items": { "type": "string" },
        "description": "List of files created or modified during the session"
      },
      "current_task": {
        "type": "string",
        "description": "What was being worked on (in progress or next step)"
      },
      "git_branch": {
        "type": "string",
        "description": "Current git branch"
      },
      "turns": {
        "type": "integer",
        "description": "Number of conversation turns so far"
      }
    },
    "required": ["session_id", "summary"]
  }
}
```

## Réponse type

```
Session pushed to Herald

- Task ID: herald-8f3a2b1c
- Session: ses_abc123
- Project: herald
- Status: linked

You can now continue this session from Claude Chat:
  list_tasks to find it
  check_task for the full summary
  start_task with session_id "ses_abc123" to resume
```

## Implémentation serveur

### handler : `internal/mcp/handlers/herald_push.go`

```go
func HandleHeraldPush(deps Deps) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        var params struct {
            SessionID     string   `json:"session_id"`
            Project       string   `json:"project"`
            Summary       string   `json:"summary"`
            FilesModified []string `json:"files_modified"`
            CurrentTask   string   `json:"current_task"`
            GitBranch     string   `json:"git_branch"`
            Turns         int      `json:"turns"`
        }
        // parse params...

        task := &task.Task{
            ID:            generateTaskID(),
            Type:          task.TypeLinked,
            Status:        task.StatusLinked,
            SessionID:     params.SessionID,
            Project:       params.Project,
            Output:        params.Summary,
            Progress:      params.CurrentTask,
            FilesModified: params.FilesModified,
            GitBranch:     params.GitBranch,
            Turns:         params.Turns,
            CreatedAt:     time.Now(),
        }

        deps.Store.CreateTask(task)

        return mcp.NewToolResultText(fmt.Sprintf(
            "Session pushed to Herald\n\n"+
                "Task ID: %s\n"+
                "Session: %s\n"+
                "Project: %s\n"+
                "Status: linked\n\n"+
                "Continue from Claude Chat with check_task or start_task(session_id=%q)",
            task.ID, params.SessionID, params.Project, params.SessionID,
        )), nil
    }
}
```

### Nouveau type et statut de tâche

```go
// internal/task/task.go

const (
    TypeDispatched = "dispatched"  // lancée via Chat -> Herald -> Code
    TypeLinked     = "linked"      // poussée depuis Code -> Herald
)

const (
    StatusLinked = "linked"  // session externe enregistrée, pas gérée par Herald
)
```

### Modification de list_tasks et check_task

Les tâches linked apparaissent dans list_tasks avec un indicateur :

```
herald-8f3a2b1c | linked | herald | 3min ago
   "Refactoring auth middleware, added rate limiting tests"
   Session: ses_abc123 — use start_task to resume
```

check_task retourne le résumé poussé par Claude Code + les fichiers modifiés + la branche Git.

## Philosophie

Herald ne change rien aux habitudes du dev :
- Tu utilises `claude` normalement
- Quand tu veux partir, tu dis "push à Herald"
- Tu continues depuis ton téléphone

Pas de wrapper, pas d'alias, pas de nouvelle commande à apprendre. Claude Code fait le travail.

## Tests

- herald_push avec tous les champs -> tâche créée en DB type linked
- herald_push avec champs minimaux (session_id + summary) -> OK
- list_tasks inclut les tâches linked
- check_task sur une tâche linked -> retourne le résumé
- start_task avec session_id d'une tâche linked -> lance --resume
- Doublon : push deux fois le même session_id -> update, pas doublon

## Estimation

~15 min via Herald. Un handler MCP + un type de tâche. Réutilise toute l'infra existante.

## Impact sur ACTION_PLAN.md

Phase 1bis :
```
Phase 1bis herald_push — liaison MCP bidirectionnelle   15 min
```

Critères de done :
```
- [ ] herald_push enregistre une session Claude Code dans Herald
- [ ] Sessions linked visibles dans list_tasks et check_task
- [ ] start_task avec session_id d'une session linked fait un --resume
```
