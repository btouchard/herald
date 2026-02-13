<p align="center">
  <h1 align="center">Herald</h1>
  <p align="center">
    <strong>Codez depuis votre telephone. Pour de vrai.</strong>
    <br />
    <em>Le pont MCP self-hosted entre Claude Chat et Claude Code.</em>
  </p>
</p>

<p align="center">
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go 1.26+"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg" alt="AGPL-3.0 License"></a>
  <a href="https://github.com/kolapsis/herald/stargazers"><img src="https://img.shields.io/github/stars/kolapsis/herald?style=social" alt="GitHub Stars"></a>
</p>

<p align="center">
  <a href="#-demarrage-rapide">Demarrage rapide</a> &middot;
  <a href="#-comment-ca-marche">Comment ca marche</a> &middot;
  <a href="#%EF%B8%8F-fonctionnalites">Fonctionnalites</a> &middot;
  <a href="#-securite">Securite</a> &middot;
  <a href="#-feuille-de-route">Feuille de route</a>
  <br />
  :gb: <a href="README.md">English version</a>
</p>

---

Vous etes dans le canape. Sur votre telephone. Vous ouvrez Claude Chat et tapez :

> *"Refactore le middleware d'auth dans my-api pour utiliser du JWT au lieu des cookies de session. Lance les tests."*

Quatre minutes plus tard, c'est fait. Branche creee, code refactore, tests OK, changements commites. Votre machine a tout fait. Vous n'avez jamais ouvert votre laptop.

**Ca, c'est Herald.**

## Le Probleme

Claude Chat et Claude Code sont deux outils brillants qui vivent dans des mondes totalement separes.

| | Claude Chat | Claude Code |
|---|---|---|
| **Ou** | Navigateur, telephone, partout | Votre terminal |
| **Quoi** | Conversations, analyse, reflexion | Lit, ecrit et livre du vrai code |
| **Le trou** | Ne peut pas toucher votre code | Ne peut pas quitter votre bureau |

Vous faisiez du copier-coller entre les deux. Ou pire : vous attendiez d'etre de retour a votre bureau. C'est termine.

## La Solution

Herald est un serveur MCP self-hosted qui connecte Claude Chat a Claude Code via le protocole officiel [Custom Connectors](https://support.claude.com/en/articles/11503834-building-custom-connectors-via-remote-mcp-servers) d'Anthropic. Un binaire Go. Zero bidouille.

```
  Vous (telephone/tablette/navigateur)
       |
       |  "Ajoute du rate limiting a l'API"
       v
  Claude Chat ──── MCP over HTTPS ────> Herald (votre machine)
                                           |
                                           v
                                        Claude Code
                                           |-- lit votre codebase
                                           |-- ecrit le code
                                           |-- lance les tests
                                           '-- commit sur une branche

  Vous (terminal)
       |
       |  Claude Code appelle herald_push
       v
  Claude Code ──── MCP ────> Herald ────> Claude Chat reprend la main
                                           '-- contexte de session, resume,
                                               fichiers modifies, branche git
```

Le pont est **bidirectionnel**. Claude Chat envoie des taches a Claude Code, et Claude Code peut pousser son contexte de session vers Herald pour le suivi a distance et la reprise depuis un autre appareil.

Votre code ne quitte jamais votre machine. Herald ne fait qu'orchestrer.

## Comment ca marche

```
Vous (Claude Chat)         Herald                     Claude Code
──────────────────         ──────                     ───────────
"Refactore auth..."   ──>  start_task
                           -> cree une branche
                           -> lance Claude Code  ──>  lit le codebase
                                                      refactore le code
                                                      lance les tests
                                                      commit les changements
                      <──  task_id: herald-a1b2c3d4

"Ca en est ou ?"      ──>  check_task
                      <──  Termine (4m 12s)
                           4 fichiers modifies (+127/-23)

"Montre le diff"      ──>  get_diff
                      <──  auth/middleware.go
                           +func ValidateJWT(...)
                           -func CheckSession(...)
```

Trois outils. C'est la boucle principale. Lancer, verifier, recuperer les resultats — d'ou que vous soyez.

### Flux inverse : Claude Code → Herald

Vous travaillez dans votre terminal et voulez continuer depuis votre telephone ? Claude Code pousse sa session vers Herald :

```
Vous (terminal)            Claude Code                Herald
──────────────             ───────────                ──────
"Pousse ca vers Herald" ──> herald_push
                             -> session_id, resume,
                                fichiers, branche  ──> tache liee creee
                                                        visible dans list_tasks

Vous (telephone, apres)    Claude Chat                Herald
──────────────────         ───────────                ──────
"Quelles sessions         list_tasks
 m'attendent ?"       ──> (status: linked)       ──> herald-a1b2c3d4
                                                        my-api / feat/auth

"Reprends cette session" ──> start_task
                              (session_id)        ──> reprend la ou vous en etiez
```

## Fonctionnalites

### Coeur

- **Pont MCP natif** — Utilise le protocole officiel Custom Connectors d'Anthropic. Pas un hack, pas un wrapper, pas un proxy.
- **Execution asynchrone** — Lancez des taches, suivez la progression, recuperez les resultats. Claude Code tourne en arriere-plan pendant que vous faites autre chose.
- **Isolation Git** — Chaque tache tourne sur sa propre branche. Votre branche principale reste intacte.
- **Reprise de session** — Conversations Claude Code multi-tours. Reprenez la ou vous en etiez.
- **Pont bidirectionnel** — Claude Code peut pousser son contexte de session vers Herald via `herald_push` pour le suivi a distance et la reprise depuis un autre appareil.

### Multi-Projet

- **Plusieurs projets** — Configurez autant de projets que necessaire, chacun avec ses propres parametres.
- **Restrictions d'outils par projet** — Controlez exactement quels outils Claude Code peut utiliser. Sandboxing complet par projet.

### Operations

- **Notifications MCP push** — Herald pousse les mises a jour directement vers Claude Chat via les notifications serveur MCP. Pas de polling necessaire.
- **Persistance SQLite** — Les taches survivent aux redemarrages. Historique complet et consultable.

### Ingenierie

- **Binaire unique** — Un executable Go, ~15 Mo. Pas de Docker, pas de runtime, pas de node_modules.
- **Zero CGO** — Go pur. Cross-compile vers Linux, macOS, Windows, ARM.
- **6 dependances** — chi, mcp-go, modernc/sqlite, uuid, yaml, testify. C'est tout l'arbre de dependances.

## Demarrage rapide

**Prerequis** : Go 1.26+, [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) installe, un domaine avec HTTPS.

```bash
# Compiler
git clone https://github.com/kolapsis/herald.git
cd herald && make build

# Configurer
mkdir -p ~/.config/herald
cp configs/herald.example.yaml ~/.config/herald/herald.yaml

# Lancer (le secret client est auto-genere au premier demarrage)
./bin/herald serve
```

Editez `~/.config/herald/herald.yaml` avec votre domaine et vos projets :

```yaml
server:
  host: "127.0.0.1"
  port: 8420
  public_url: "https://herald.votredomaine.com"

auth:
  client_id: "herald-claude-chat"

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
      branch_prefix: "herald/"
```

Puis connectez-vous depuis Claude Chat :

1. **Claude Chat** -> **Parametres** -> **Custom Connectors**
2. Ajoutez un connecteur : `https://herald.votredomaine.com/mcp`
3. Authentifiez-vous via OAuth
4. C'est fait — Claude Chat dispose maintenant de 10 outils pour piloter votre machine

<details>
<summary><strong>Reference de configuration complete</strong></summary>

```yaml
server:
  host: "127.0.0.1"          # Toujours localhost — le reverse proxy gere l'externe
  port: 8420
  public_url: "https://herald.votredomaine.com"
  log_level: "info"           # debug, info, warn, error
  log_file: ""                # Chemin optionnel pour la sortie des logs

auth:
  client_id: "herald-claude-chat"
  # client_secret est auto-genere — redefinir avec la var env HERALD_CLIENT_SECRET si besoin
  access_token_ttl: 1h
  refresh_token_ttl: 720h    # 30 jours
  redirect_uris:
    - "https://claude.ai/oauth/callback"
    - "https://claude.ai/api/oauth/callback"

database:
  path: "~/.config/herald/herald.db"
  retention_days: 90

execution:
  claude_path: "claude"
  model: "claude-sonnet-4-5-20250929"  # Modele par defaut pour les taches
  default_timeout: 30m
  max_timeout: 2h
  work_dir: "~/.config/herald/work"
  max_concurrent: 3
  max_prompt_size: 102400    # 100KB
  max_output_size: 1048576   # 1MB
  env:
    CLAUDE_CODE_ENTRYPOINT: "herald"
    CLAUDE_CODE_DISABLE_AUTO_UPDATE: "1"

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
  requests_per_minute: 200
  burst: 100

```

</details>

## Outils MCP

Herald expose 10 outils que Claude Chat decouvre automatiquement via le protocole MCP :

| Outil | Ce qu'il fait |
|---|---|
| `start_task` | Lance une tache Claude Code. Retourne un ID immediatement. Priorite, timeout, reprise de session, options Git. |
| `check_task` | Verifie le statut et la progression. Peut inclure la sortie recente. |
| `get_result` | Resultat complet d'une tache terminee (`summary`, `full` ou `json`). |
| `list_tasks` | Liste les taches avec filtres — statut, projet, periode. |
| `cancel_task` | Annule une tache en cours ou en file. Peut reverter les changements Git. |
| `get_diff` | Diff Git de la branche d'une tache ou des changements non commites. |
| `list_projects` | Projets configures avec statut Git. |
| `read_file` | Lire un fichier d'un projet (securise — ne peut pas sortir de la racine projet). |
| `herald_push` | Pousse une session Claude Code vers Herald pour le suivi a distance et la reprise depuis un autre appareil. |
| `get_logs` | Logs et historique d'activite. |

## Securite

Herald expose Claude Code sur le reseau. On prend ca au serieux.

| Couche | Protection |
|---|---|
| **Reseau** | Ecoute sur `127.0.0.1` uniquement. Reverse proxy (Traefik/Caddy) gere le TLS. |
| **Auth** | OAuth 2.1 avec PKCE. Chaque requete necessite un Bearer token valide. |
| **Tokens** | Access tokens : 1h. Refresh tokens : 30j, rotation a chaque utilisation. |
| **Filesystem** | Protection path traversal sur toutes les operations fichier. Echappement symlink bloque. |
| **Execution** | Restrictions d'outils par projet. Pas de `--dangerously-skip-permissions`. |
| **Rate limiting** | 200 req/min par token (configurable). |
| **Timeouts** | Chaque tache a une deadline (defaut : 30min). Pas de processus zombie. |
| **Prompts** | Transmis a Claude Code sans modification. Pas d'injection, pas d'enrichissement. |
| **Audit** | Chaque action logguee avec horodatage et identite. |

## Architecture

```
Claude Chat (mobile/web)
  -> HTTPS (MCP Streamable HTTP + OAuth 2.1)
  -> Traefik / Caddy (terminaison TLS)
  -> Herald (binaire Go, port 8420)
    |-- Handler MCP (/mcp)
    |-- Serveur OAuth 2.1 (PKCE, rotation des tokens)
    |-- Gestionnaire de taches (pool de goroutines, file de priorite)
    |-- Executeur Claude Code (os/exec, parsing stream-json)
    |-- SQLite (persistance)
    '-- Notifications MCP (push serveur via SSE)
```

**Principes** : binaire unique (tout compile dans un seul executable Go), async-first (chaque tache est une goroutine), MCP stateless avec backend stateful, fail-safe (un crash de Herald ne tue pas les processus Claude Code en cours).

<details>
<summary><strong>Stack technique</strong></summary>

| Composant | Choix | Pourquoi |
|---|---|---|
| Langage | Go 1.26 | Binaire unique, cross-compilation, goroutines |
| MCP | [mcp-go](https://github.com/mark3labs/mcp-go) | Streamable HTTP, protocole officiel |
| Routeur | [chi](https://github.com/go-chi/chi) | Leger, compatible stdlib |
| BDD | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | Go pur, zero CGO |
| Logging | `log/slog` | Stdlib Go, structure |
| Config | `gopkg.in/yaml.v3` | YAML standard |

6 dependances directes. Pas d'ORM. Pas de framework de logging. Pas de toolchain de build.

</details>

## Deploiement

Herald tourne idealement en binaire natif (acces direct a Claude Code et vos fichiers). Docker est disponible en option.

<details>
<summary><strong>Docker Compose avec Traefik</strong></summary>

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
    network_mode: host
    volumes:
      - "~/.config/herald:/root/.config/herald"
      - "~/projets:/root/projets:ro"
    labels:
      - "traefik.http.routers.herald.rule=Host(`herald.votredomaine.com`)"
      - "traefik.http.routers.herald.tls.certresolver=le"
      - "traefik.http.services.herald.loadbalancer.server.port=8420"
```

</details>

## Feuille de route

| Version | Statut | Focus |
|---|---|---|
| **v0.1** | :white_check_mark: Termine | Serveur MCP core, taches async, integration Git, OAuth 2.1, SQLite |
| **v0.2** | :construction: En cours | Memoire partagee — contexte bidirectionnel entre Claude Chat et Claude Code |
| **v0.3** | :clipboard: Prevu | Monitoring temps reel (UI web — long terme) |
| **v1.0** | :rocket: Futur | API stable, systeme de plugins |

Une idee ? [Ouvrez une issue](https://github.com/kolapsis/herald/issues). On construit ce dont les utilisateurs ont besoin.

## Contribuer

Herald est en alpha — le meilleur moment pour influencer un projet.

```bash
# Demarrer
git clone https://github.com/kolapsis/herald.git
cd herald
make build && make test

# Creer votre branche
git checkout -b feat/votre-feature

# Coder, tester, linter
make lint && make test

# Ouvrir une PR
```

Les messages de commit suivent [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `refactor:`, `docs:`).

Que ce soit un fix, un nouveau backend de notification, ou une amelioration de la doc — toutes les contributions sont les bienvenues.

## Pourquoi Herald ?

| | Herald | Copier-coller | Autres outils |
|---|---|---|---|
| **Protocole officiel** | MCP Custom Connectors | N/A | APIs custom, fragile |
| **Code reste local** | Toujours | Oui | Ca depend |
| **Marche depuis le tel** | Natif | Non | Rarement |
| **Self-hosted** | 100% | N/A | Souvent SaaS |
| **Dependances** | 6 | N/A | 50-200+ |
| **Temps de setup** | ~5 minutes | N/A | 30min+ |
| **CGO requis** | Non | N/A | Souvent |

Herald utilise le meme protocole qu'Anthropic a construit pour ses propres integrations. Pas de reverse engineering, pas d'APIs non-officielles, pas de bidouilles qui cassent a la prochaine mise a jour.

---

<p align="center">
  <a href="LICENSE"><strong>AGPL-3.0 License</strong></a> — Fait par <a href="https://github.com/kolapsis"><strong>Kolapsis</strong></a>
  <br /><br />
  Si Herald vous fait gagner du temps, <a href="https://github.com/kolapsis/herald">laissez une etoile</a>. Ca aide les autres a decouvrir le projet.
</p>
