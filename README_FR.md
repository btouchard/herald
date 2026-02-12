# Herald

**Reliez Claude Chat à Claude Code. Pilotez votre poste de travail depuis votre téléphone.**

[![Go 1.26+](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Status: Alpha](https://img.shields.io/badge/Status-Alpha-orange)]()

:fr: Version française — [English version](README.md)

---

Claude Chat et Claude Code vivent dans deux mondes séparés. L'un tourne dans votre navigateur et sur votre téléphone. L'autre tourne dans votre terminal et écrit du vrai code. Ils ne se parlent pas.

Herald règle ce problème. C'est un serveur MCP auto-hébergé qui connecte Claude Chat à Claude Code via le protocole officiel [Custom Connectors](https://docs.anthropic.com/en/docs/claude-code/mcp) d'Anthropic. Vous restez dans Claude Chat — Herald envoie le travail à Claude Code sur votre machine.

```
  📱 Claude Chat (téléphone / web)
       │
       ▼ MCP over HTTPS
  🖥️  Herald (votre poste de travail)
       │
       ▼ lance & gère
  ⚡ Claude Code (exécute les tâches)
```

## Le Workflow

Vous êtes sur votre téléphone. Vous ouvrez Claude Chat et vous dites :

> « Refactore le middleware d'auth dans my-api pour utiliser du JWT au lieu des cookies de session. »

Voici ce qui se passe :

```
Vous (Claude Chat)         Herald                     Claude Code
──────────────────         ──────                     ───────────
"Refactore auth..."   ──►  start_task
                           → crée une branche
                           → lance Claude Code   ──►  lit le codebase
                                                      refactore l'auth
                                                      lance les tests
                                                      commit les changements
                      ◄──  task_id: herald-a1b2c3d4

"Où ça en est ?"      ──►  check_task
                      ◄──  ✅ Terminé (4m 12s)
                           4 fichiers modifiés (+127/-23)

"Montre-moi le diff"  ──►  get_diff
                      ◄──  auth/middleware.go
                           +func ValidateJWT(...)
                           -func CheckSession(...)
                           ...
```

Tout ça depuis votre téléphone. Votre machine a fait le gros du travail.

## Fonctionnalités

- **Pont MCP natif** — Utilise les Custom Connectors officiels d'Anthropic. Pas un hack, pas un wrapper.
- **Exécution asynchrone** — Lancez des tâches, suivez la progression, récupérez les résultats. Pas de long-polling, pas de timeout.
- **Isolation Git par branche** — Chaque tâche a sa propre branche. Votre branche principale reste propre.
- **Reprise de session** — Conversations Claude Code multi-tours. Reprenez là où vous vous êtes arrêté.
- **Multi-projets** — Configurez plusieurs projets avec des politiques de sécurité distinctes.
- **Outils autorisés par projet** — Contrôlez exactement quels outils Claude Code peut utiliser sur chaque projet.
- **OAuth 2.1 + PKCE** — Une vraie auth. Pas une clé API partagée.
- **Persistance SQLite** — Les tâches survivent aux redémarrages. L'historique est consultable.
- **Notifications push** — Soyez notifié via [ntfy](https://ntfy.sh) quand une tâche se termine ou échoue.
- **Binaire unique** — Un seul exécutable Go, ~15 Mo. Pas de Docker requis, pas de dépendances runtime.
- **Zéro CGO** — Cross-compilation vers toutes les plateformes supportées par Go.
- **6 dépendances** — chi, mcp-go, modernc/sqlite, uuid, yaml, testify. C'est tout.

## Démarrage rapide

### Prérequis

- **Go 1.26+**
- **Claude Code CLI** installé et authentifié (`claude --version`)
- **Compte Anthropic** avec accès aux Custom Connectors
- **Un domaine avec HTTPS** (Traefik, Caddy, ou tout reverse proxy pour le TLS)

### Compilation

```bash
git clone https://github.com/kolapsis/herald.git
cd herald
make build
```

Cela produit `bin/herald` — un binaire lié statiquement, zéro CGO.

### Configuration

```bash
mkdir -p ~/.config/herald
cp configs/herald.example.yaml ~/.config/herald/herald.yaml
```

Éditez `~/.config/herald/herald.yaml` :

```yaml
server:
  host: "127.0.0.1"
  port: 8420
  public_url: "https://herald.votredomaine.com"

auth:
  client_id: "herald-claude-chat"
  client_secret: "${HERALD_CLIENT_SECRET}"

projects:
  my-api:
    path: "/home/vous/projets/my-api"
    description: "API backend principale"
    default: true
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
      - "Bash(git *)"
      - "Bash(go *)"
      - "Bash(make *)"
    git:
      auto_branch: true
      auto_stash: true
      branch_prefix: "herald/"
```

Définissez le secret requis :

```bash
export HERALD_CLIENT_SECRET="$(openssl rand -hex 32)"
```

### Lancement

```bash
./bin/herald serve
# herald is ready addr=127.0.0.1:8420
```

### Connexion depuis Claude Chat

1. Allez dans **Claude Chat** → **Paramètres** → **Custom Connectors**
2. Ajoutez un nouveau connecteur MCP :
   - **URL** : `https://herald.votredomaine.com/mcp`
   - **Auth** : OAuth 2.1 (Herald gère le flow)
3. Claude Chat découvre automatiquement les 9 outils de Herald
4. Parlez à Claude — il peut maintenant envoyer des tâches à votre machine

## Référence de configuration

<details>
<summary>herald.yaml complet avec toutes les options</summary>

```yaml
server:
  host: "127.0.0.1"          # Toujours localhost — le reverse proxy gère l'externe
  port: 8420
  public_url: "https://herald.votredomaine.com"
  log_level: "info"           # debug, info, warn, error
  # log_file: "/var/log/herald.log"

auth:
  client_id: "herald-claude-chat"
  client_secret: "${HERALD_CLIENT_SECRET}"
  admin_password_hash: "${HERALD_ADMIN_PASSWORD_HASH}"
  access_token_ttl: 1h
  refresh_token_ttl: 720h    # 30 jours

  # Tokens API pour REST API / curl / automatisation
  # api_tokens:
  #   - name: "local"
  #     token_hash: "${HERALD_API_TOKEN_HASH}"
  #     scope: "*"

database:
  path: "~/.config/herald/herald.db"
  retention_days: 90

execution:
  claude_path: "claude"       # Chemin vers le binaire Claude Code
  default_timeout: 30m
  max_timeout: 2h
  work_dir: "~/.config/herald/work"
  max_concurrent: 3           # Max d'instances Claude Code en parallèle
  env:
    CLAUDE_CODE_ENTRYPOINT: "herald"
    CLAUDE_CODE_DISABLE_AUTO_UPDATE: "1"

notifications:
  ntfy:
    enabled: false
    server: "https://ntfy.sh"
    topic: "herald"
    # token: "${HERALD_NTFY_TOKEN}"
    events:
      - "task.completed"
      - "task.failed"

  # webhooks:
  #   - name: "n8n"
  #     url: "https://n8n.example.com/webhook/herald"
  #     secret: "${HERALD_WEBHOOK_SECRET}"
  #     events: ["task.completed", "task.failed"]

projects:
  my-api:
    path: "/home/vous/projets/my-api"
    description: "API backend principale"
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

rate_limit:
  requests_per_minute: 60
  burst: 10

dashboard:
  enabled: true
```

</details>

## Outils MCP

Herald expose 9 outils via le protocole MCP. Claude Chat les découvre et les utilise automatiquement.

| Outil | Description |
|---|---|
| `start_task` | Lance une tâche Claude Code. Retourne un ID immédiatement. Supporte priorité, timeout, dry run, reprise de session et options Git. |
| `check_task` | Vérifie le statut et la progression d'une tâche en cours. Peut inclure les dernières lignes de sortie. |
| `get_result` | Récupère le résultat complet d'une tâche terminée. Formats : `summary`, `full` ou `json`. |
| `list_tasks` | Liste les tâches avec filtres (statut, projet, période, limite). |
| `cancel_task` | Annule une tâche en cours ou en attente. Peut reverter les changements Git. |
| `get_diff` | Affiche le diff Git d'une branche de tâche ou des changements non commités d'un projet. |
| `list_projects` | Liste les projets configurés avec leur statut Git et description. |
| `read_file` | Lit un fichier d'un projet. Sécurisé — impossible de sortir de la racine du projet. |
| `get_logs` | Consulte les logs et l'historique d'activité. Filtrage par tâche, niveau ou nombre. |

## Architecture

```
Claude Chat (mobile/web)
  → HTTPS (MCP Streamable HTTP + OAuth 2.1)
  → Traefik / Caddy (reverse proxy, terminaison TLS)
  → Herald (binaire Go, port 8420)
    ├── Handler MCP (/mcp)
    ├── Serveur OAuth 2.1 (PKCE, rotation des tokens)
    ├── Gestionnaire de tâches (pool de goroutines, file de priorité)
    ├── Exécuteur Claude Code (os/exec, parsing stream-json)
    ├── SQLite (persistance tâches, tokens auth)
    └── Hub de notifications (ntfy, webhooks)
```

### Principes de conception

- **Binaire unique** — Tout est embarqué. Dashboard HTML via `go:embed`. Pas de runtime externe.
- **Async-first** — Chaque tâche est une goroutine. Pattern start/check/result par polling.
- **MCP stateless, backend stateful** — Les requêtes MCP sont indépendantes. L'état vit dans SQLite + mémoire.
- **Fail-safe** — Si Herald crashe, les processus Claude Code en cours continuent. Les résultats persistent sur disque.

### Stack technique

| Composant | Choix | Pourquoi |
|---|---|---|
| Langage | Go 1.26 | Binaire unique, cross-compilation, goroutines |
| MCP | [mcp-go](https://github.com/mark3labs/mcp-go) | Streamable HTTP, support protocole officiel |
| Routeur HTTP | [chi](https://github.com/go-chi/chi) | Léger, compatible stdlib |
| Base de données | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | SQLite pur Go, zéro CGO |
| Logging | `log/slog` | Stdlib Go, structuré, multi-handler |
| Config | `gopkg.in/yaml.v3` | Parsing YAML standard |

6 dépendances directes. Pas d'ORM, pas de framework de logging, pas de toolchain de build.

## Sécurité

Herald expose Claude Code sur le réseau. La sécurité n'est pas optionnelle.

- **Localhost uniquement** — Herald écoute sur `127.0.0.1`. Un reverse proxy (Traefik, Caddy) gère le TLS et l'accès externe.
- **OAuth 2.1 + PKCE** — Chaque requête MCP nécessite un Bearer token valide. Pas de clé partagée.
- **Tokens à durée courte** — Les access tokens expirent en 1 heure. Les refresh tokens tournent à chaque utilisation.
- **Protection path traversal** — `read_file` résout les chemins et vérifie qu'ils restent dans la racine du projet. Les échappements par symlink sont bloqués.
- **Restrictions d'outils par projet** — Chaque projet définit exactement quels outils Claude Code peut utiliser. Pas de permissions globales.
- **Rate limiting** — 60 requêtes/minute par token par défaut.
- **Timeouts de tâches** — Chaque tâche a une deadline (30 min par défaut). Pas de processus infini.
- **Pas d'injection de prompt** — Herald transmet les prompts à Claude Code sans modification. Pas d'enrichissement, pas de system prompt ajouté, pas de réécriture.
- **Piste d'audit** — Chaque action est loggée avec horodatage et identité.

## Déploiement avec Traefik

Herald est conçu pour fonctionner derrière un reverse proxy. Voici un `docker-compose.yml` minimal :

```yaml
services:
  traefik:
    image: traefik:v3
    command:
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.le.acme.email=vous@example.com"
      - "--certificatesresolvers.le.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.le.acme.httpchallenge.entrypoint=web"
    ports:
      - "443:443"
    volumes:
      - "./letsencrypt:/letsencrypt"

  herald:
    build: .
    network_mode: host     # Besoin d'accéder à Claude Code sur l'hôte
    volumes:
      - "~/.config/herald:/root/.config/herald"
      - "~/projets:/root/projets:ro"
    environment:
      - HERALD_CLIENT_SECRET
    labels:
      - "traefik.http.routers.herald.rule=Host(`herald.votredomaine.com`)"
      - "traefik.http.routers.herald.tls.certresolver=le"
      - "traefik.http.services.herald.loadbalancer.server.port=8420"
```

> **Note** : Faire tourner Herald en binaire natif (hors Docker) est recommandé pour la meilleure expérience, car il a besoin d'un accès direct à Claude Code et à vos fichiers projet.

## Feuille de route

| Version | Statut | Focus |
|---|---|---|
| **v0.1** | :white_check_mark: Terminé | Serveur MCP core, exécution async, intégration Git, OAuth 2.1, persistance SQLite |
| **v0.2** | :arrows_counterclockwise: En cours | Mémoire partagée — contexte bidirectionnel entre Claude Chat et Claude Code |
| **v0.3** | :clipboard: Prévu | Dashboard temps réel (UI web embarquée avec SSE) |
| **v1.0** | :rocket: Futur | API stable, hébergement managé, système de plugins |

## Contribuer

Herald est en alpha. Les contributions sont les bienvenues.

1. Forkez le dépôt
2. Créez une branche (`feat/votre-feature` ou `fix/votre-fix`)
3. Écrivez des tests pour les changements non triviaux
4. Lancez `make lint && make test`
5. Ouvrez une PR

Les messages de commit suivent [Conventional Commits](https://www.conventionalcommits.org/).

## Licence

[MIT](LICENSE) — Kolapsis
