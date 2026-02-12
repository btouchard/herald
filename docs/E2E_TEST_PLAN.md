# Plan de tests E2E — Herald sur homelab

> Stack : `*.home.kolapsis.com` → Traefik (DMZ, Let's Encrypt) → Herald (workstation)

---

## 1. Architecture cible

```
Internet / Claude Chat (claude.ai)
  │
  │  HTTPS (TLS 1.3, Let's Encrypt)
  ▼
┌──────────────────────────────────┐
│  herald.home.kolapsis.com        │
│  Traefik (DMZ)                   │
│  - TLS termination               │
│  - Certificat LE wildcard        │
│  - Forward headers (X-Real-IP)   │
│  - Proxy vers workstation:8420   │
└──────────┬───────────────────────┘
           │  HTTP (réseau interne)
           ▼
┌──────────────────────────────────┐
│  Herald (workstation:8420)       │
│  - Écoute 127.0.0.1:8420        │
│  - OAuth 2.1 + PKCE             │
│  - MCP Streamable HTTP (/mcp)   │
│  - SQLite (~/.config/herald/)   │
│  - Claude Code CLI (os/exec)    │
└──────────────────────────────────┘
```

### Prérequis

| Composant       | Version                            | Vérification                                |
| --------------- | ---------------------------------- | ------------------------------------------- |
| Go              | 1.26+                              | `go version`                                |
| Claude Code CLI | 2.x                                | `claude --version`                          |
| Traefik         | 3.x                                | `traefik version`                           |
| Herald          | dev                                | `herald version`                            |
| DNS             | `*.home.kolapsis.com` → IP Traefik | `dig herald.home.kolapsis.com`              |
| Certificat      | Let's Encrypt wildcard valide      | `curl -vI https://herald.home.kolapsis.com` |

---

## 2. Configuration Herald pour les tests

```yaml
# ~/.config/herald/herald.yaml
server:
  host: "127.0.0.1"
  port: 8420
  public_url: "https://herald.home.kolapsis.com"
  log_level: "debug"

auth:
  client_id: "herald-claude-chat"
  client_secret: "${HERALD_CLIENT_SECRET}"

database:
  path: "~/.config/herald/herald.db"

execution:
  claude_path: "claude"
  max_concurrent: 2
  work_dir: "~/.config/herald/work"

projects:
  herald:
    path: "/home/benjamin/Documents/kOlapsis/herald"
    description: "Herald — MCP bridge"
    default: true
    allowed_tools:
      - "Read"
      - "Glob"
      - "Grep"
    max_concurrent_tasks: 1
```

### Configuration Traefik (extrait)

```yaml
# Route dynamique Traefik → Herald
http:
  routers:
    herald:
      rule: "Host(`herald.home.kolapsis.com`)"
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
      service: herald

  services:
    herald:
      loadBalancer:
        servers:
          - url: "http://<workstation-ip>:8420"
```

> **Note :** Herald écoute sur `127.0.0.1`. Si Traefik est sur une autre machine, il faut soit un tunnel SSH, soit changer le bind en `<ip-locale>:8420` (mais jamais `0.0.0.0` en production).

---

## 3. Tests infrastructure

### T-INFRA-01 — Résolution DNS

```bash
dig +short herald.home.kolapsis.com
# Attendu : IP du serveur Traefik
```

### T-INFRA-02 — Certificat TLS valide

```bash
curl -sI https://herald.home.kolapsis.com/health | head -5
# Attendu :
#   HTTP/2 200
#   content-type: application/json
```

```bash
openssl s_client -connect herald.home.kolapsis.com:443 -servername herald.home.kolapsis.com </dev/null 2>/dev/null | openssl x509 -noout -dates -issuer
# Attendu :
#   issuer= /C=US/O=Let's Encrypt/...
#   notBefore=...
#   notAfter=... (> date du jour)
```

### T-INFRA-03 — Health check

```bash
curl -s https://herald.home.kolapsis.com/health | jq .
# Attendu : {"status":"ok"}
```

### T-INFRA-04 — Herald refuse les connexions directes sur 0.0.0.0

```bash
curl -s http://<workstation-ip>:8420/health
# Attendu : connexion refusée (ECONNREFUSED)
# Herald n'écoute que sur 127.0.0.1
```

---

## 4. Tests OAuth 2.1

### T-AUTH-01 — Discovery metadata (RFC 8414)

```bash
curl -s https://herald.home.kolapsis.com/.well-known/oauth-authorization-server | jq .
```

**Attendu :**

```json
{
  "issuer": "https://herald.home.kolapsis.com",
  "authorization_endpoint": "https://herald.home.kolapsis.com/oauth/authorize",
  "token_endpoint": "https://herald.home.kolapsis.com/oauth/token",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "code_challenge_methods_supported": ["S256"],
  "token_endpoint_auth_methods_supported": ["client_secret_post"]
}
```

**Vérifications :**

- [x] `issuer` = `public_url` exact (pas de trailing slash)
- [x] Tous les endpoints pointent vers `https://herald.home.kolapsis.com`
- [x] `S256` présent dans `code_challenge_methods_supported`

### T-AUTH-02 — Authorization code flow (sans PKCE)

```bash
# Étape 1 : GET /oauth/authorize → redirige avec code
REDIRECT=$(curl -s -o /dev/null -w '%{redirect_url}' \
  "https://herald.home.kolapsis.com/oauth/authorize?response_type=code&client_id=herald-claude-chat&redirect_uri=https://callback.test/cb&state=test123")

printf '%s\n' "$REDIRECT"
# Attendu : https://callback.test/cb?code=<64hex>&state=test123

CODE=$(printf '%s\n' "$REDIRECT" | grep -oP 'code=\K[^&]+')
echo "Code: $CODE"
```

```bash
# Étape 2 : POST /oauth/token → échange code contre tokens
curl -s -X POST https://herald.home.kolapsis.com/oauth/token \
  -d "grant_type=authorization_code" \
  -d "code=$CODE" \
  -d "client_id=herald-claude-chat" \
  -d "client_secret=$HERALD_CLIENT_SECRET" | jq .
```

**Attendu :**

```json
{
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "<jwt>",
  "scope": ""
}
```

**Vérifications :**

- [x] `token_type` = `"Bearer"`
- [x] `expires_in` = `3600` (1h)
- [x] `access_token` est un JWT valide (3 parties base64url séparées par `.`)
- [x] `refresh_token` présent

### T-AUTH-03 — Authorization code flow avec PKCE (S256)

```bash
# Générer un code_verifier (43-128 chars, unreserved URI chars)
CODE_VERIFIER=$(openssl rand -base64 32 | tr -d '=/+' | head -c 43)

# Calculer le code_challenge S256
CODE_CHALLENGE=$(echo -n "$CODE_VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')

echo "Verifier:  $CODE_VERIFIER"
echo "Challenge: $CODE_CHALLENGE"
```

```bash
# Authorize avec PKCE
REDIRECT=$(curl -s -o /dev/null -w '%{redirect_url}' \
  "https://herald.home.kolapsis.com/oauth/authorize?response_type=code&client_id=herald-claude-chat&redirect_uri=https://callback.test/cb&code_challenge=$CODE_CHALLENGE&code_challenge_method=S256")

CODE=$(printf '%s\n' "$REDIRECT" | grep -oP 'code=\K[^&]+')
```

```bash
# Token exchange avec code_verifier
curl -s -X POST https://herald.home.kolapsis.com/oauth/token \
  -d "grant_type=authorization_code" \
  -d "code=$CODE" \
  -d "client_id=herald-claude-chat" \
  -d "client_secret=$HERALD_CLIENT_SECRET" \
  -d "code_verifier=$CODE_VERIFIER" | jq .
# Attendu : 200 avec access_token + refresh_token
```

```bash
# Même code mais MAUVAIS verifier → doit échouer
curl -s -X POST https://herald.home.kolapsis.com/oauth/token \
  -d "grant_type=authorization_code" \
  -d "code=$CODE" \
  -d "client_id=herald-claude-chat" \
  -d "client_secret=$HERALD_CLIENT_SECRET" \
  -d "code_verifier=wrong-verifier-value" | jq .
# Attendu : 400 {"error":"invalid_grant","error_description":"PKCE verification failed"}
```

### T-AUTH-04 — Refresh token + rotation

```bash
# Obtenir les tokens initiaux (réutiliser T-AUTH-02)
# ...
REFRESH_TOKEN="<refresh_token from above>"

# Refresh
RESP=$(curl -s -X POST https://herald.home.kolapsis.com/oauth/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=$REFRESH_TOKEN" \
  -d "client_id=herald-claude-chat" \
  -d "client_secret=$HERALD_CLIENT_SECRET")

printf '%s\n' "$RESP" | jq .
NEW_REFRESH=$(printf '%s\n' "$RESP" | jq -r .refresh_token)
# Attendu : 200 avec nouveaux access_token + refresh_token
```

```bash
# Réutiliser l'ANCIEN refresh token → doit échouer (rotation)
curl -s -X POST https://herald.home.kolapsis.com/oauth/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=$REFRESH_TOKEN" \
  -d "client_id=herald-claude-chat" \
  -d "client_secret=$HERALD_CLIENT_SECRET" | jq .
# Attendu : 400 {"error":"invalid_grant","error_description":"token revoked"}
```

### T-AUTH-05 — Rejection des mauvais credentials

```bash
# Mauvais client_id
curl -s -X POST https://herald.home.kolapsis.com/oauth/token \
  -d "grant_type=authorization_code&code=whatever&client_id=wrong&client_secret=wrong" | jq .
# Attendu : 401 {"error":"invalid_client"}
```

### T-AUTH-06 — Code usage unique

```bash
# Obtenir un code, l'utiliser une fois (succès), le réutiliser (échec)
# Première utilisation : 200
# Deuxième utilisation : 400 {"error":"invalid_grant","error_description":"authorization code already used"}
```

---

## 5. Tests MCP — Protocole

> Tous les appels MCP nécessitent un Bearer token valide.
> Variable : `TOKEN` = access_token obtenu via T-AUTH-02.
> 
> **Important :** Le protocole MCP Streamable HTTP utilise des sessions. L'appel `initialize`
> retourne un header `Mcp-Session-Id` qui doit être transmis dans toutes les requêtes suivantes.

### T-MCP-01 — MCP sans auth → 401

```bash
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
  -w "\nHTTP: %{http_code}\n"
# Attendu : HTTP 401, "missing Authorization header"
```

### T-MCP-02 — Initialize

```bash
# Initialize ET capturer le header Mcp-Session-Id pour les requêtes suivantes
INIT_RESPONSE=$(curl -s -D - -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"e2e-test","version":"1.0"}}}')

SESSION_ID=$(printf '%s\n' "$INIT_RESPONSE" | grep -i 'Mcp-Session-Id' | tr -d '\r' | awk '{print $2}')
echo "Session ID: $SESSION_ID"

# Extraire le body JSON (dernière ligne de la réponse)
printf '%s\n' "$INIT_RESPONSE" | tail -1 | jq .
```

**Attendu :**

- Header `Mcp-Session-Id` présent dans la réponse
- Body JSON :

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "capabilities": { "tools": { "listChanged": false } },
    "serverInfo": { "name": "Herald", "version": "dev" }
  }
}
```

### T-MCP-03 — tools/list (9 outils)

```bash
# Nécessite $SESSION_ID obtenu via T-MCP-02
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | jq '.result.tools[].name'
```

**Attendu (9 outils) :**

```
"list_projects"
"start_task"
"check_task"
"get_result"
"list_tasks"
"cancel_task"
"get_diff"
"read_file"
"get_logs"
```

---

## 6. Tests MCP — Outils

> Tous les appels ci-dessous nécessitent `$TOKEN` et `$SESSION_ID` obtenus via T-AUTH-02 et T-MCP-02.

### T-TOOL-01 — list_projects

```bash
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}' | jq '.result.content[0].text'
```

**Vérifications :**

- [ ] Le projet `herald` apparaît
- [ ] Le chemin est correct
- [ ] Le statut Git (branch, clean/dirty) est affiché

### T-TOOL-02 — start_task + check_task + get_result (flow complet)

```bash
# Lancer une tâche simple (read-only, pas de modification)
RESP=$(curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"start_task","arguments":{"prompt":"List the files in the project root directory and count them. Do not modify anything.","project":"herald","timeout_minutes":5}}}')

printf '%s\n' "$RESP" | jq '.result.content[0].text'
TASK_ID=$(printf '%s\n' "$RESP" | jq -r '.result.content[0].text' | grep -oPm1 'herald-[a-f0-9]+')
echo "Task ID: $TASK_ID"
```

```bash
# Polling : check_task toutes les 5s
while true; do
  STATUS=$(curl -s -X POST https://herald.home.kolapsis.com/mcp \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":5,\"method\":\"tools/call\",\"params\":{\"name\":\"check_task\",\"arguments\":{\"task_id\":\"$TASK_ID\",\"include_output\":true}}}" | jq -r '.result.content[0].text')

  printf '%s\n' "$STATUS"

  printf '%s\n' "$STATUS" | grep -q "running" || break
  sleep 5
done
```

```bash
# Récupérer le résultat complet
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d "{\"jsonrpc\":\"2.0\",\"id\":6,\"method\":\"tools/call\",\"params\":{\"name\":\"get_result\",\"arguments\":{\"task_id\":\"$TASK_ID\",\"format\":\"summary\"}}}" | jq '.result.content[0].text'
```

**Vérifications :**

- [ ] `start_task` retourne un task_id au format `herald-[a-f0-9]{8}`
- [ ] `check_task` montre la progression (running → completed)
- [ ] `get_result` contient la sortie de Claude Code
- [ ] Le cost USD est > 0
- [ ] La durée est cohérente

### T-TOOL-03 — list_tasks

```bash
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_tasks","arguments":{"status":"all","limit":5}}}' | jq '.result.content[0].text'
# Attendu : la tâche de T-TOOL-02 apparaît dans la liste
```

### T-TOOL-04 — cancel_task

```bash
# Lancer une tâche longue puis l'annuler
RESP=$(curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"start_task","arguments":{"prompt":"Write a very detailed 5000 word essay about the history of computing. Take your time.","project":"herald","timeout_minutes":10}}}')

TASK_ID=$(printf '%s\n' "$RESP" | jq -r '.result.content[0].text' | grep -oPm1 'herald-[a-f0-9]+')
sleep 5

curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d "{\"jsonrpc\":\"2.0\",\"id\":9,\"method\":\"tools/call\",\"params\":{\"name\":\"cancel_task\",\"arguments\":{\"task_id\":\"$TASK_ID\"}}}" | jq '.result.content[0].text'
# Attendu : "Task ... has been cancelled"
```

### T-TOOL-05 — read_file (valide)

```bash
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"read_file","arguments":{"project":"herald","path":"go.mod"}}}' | jq '.result.content[0].text'
# Attendu : contenu du fichier go.mod avec "module github.com/kolapsis/herald"
```

### T-TOOL-06 — read_file (path traversal → refusé)

```bash
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"read_file","arguments":{"project":"herald","path":"../../../etc/passwd"}}}' | jq '.result.content[0].text'
# Attendu : "Access denied: path traversal detected..."
```

```bash
# Absolute path
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"read_file","arguments":{"project":"herald","path":"/etc/passwd"}}}' | jq '.result.content[0].text'
# Attendu : "Access denied: absolute paths are not allowed..."
```

### T-TOOL-07 — get_diff

```bash
# Par projet (diff uncommitted contre HEAD)
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"get_diff","arguments":{"project":"herald"}}}' | jq '.result.content[0].text'
# Attendu : diff ou "No changes detected"

# Par task_id (diff de la branche tâche contre la branche courante)
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d "{\"jsonrpc\":\"2.0\",\"id\":14,\"method\":\"tools/call\",\"params\":{\"name\":\"get_diff\",\"arguments\":{\"task_id\":\"$TASK_ID\"}}}" | jq '.result.content[0].text'
# Attendu : diff ou "No changes detected" (requiert que la tâche soit encore en mémoire)
```

### T-TOOL-08 — get_logs

```bash
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"get_logs","arguments":{"limit":10}}}' | jq '.result.content[0].text'
# Attendu : activité récente avec les tâches créées pendant les tests
```

---

## 7. Tests sécurité

### T-SEC-01 — Bearer token expiré → 401

```bash
# Créer un token, attendre qu'il expire (ou utiliser un token forgé avec exp passé)
curl -s -X POST https://herald.home.kolapsis.com/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.INVALID.SIGNATURE" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
  -w "\nHTTP: %{http_code}\n"
# Attendu : 401
```

### T-SEC-02 — Refresh token invalide

```bash
curl -s -X POST https://herald.home.kolapsis.com/oauth/token \
  -d "grant_type=refresh_token&refresh_token=fake-token&client_id=herald-claude-chat&client_secret=$HERALD_CLIENT_SECRET" | jq .
# Attendu : 400 {"error":"invalid_grant"}
```

### T-SEC-03 — Path traversal via read_file

Couvert par T-TOOL-06 ci-dessus.

### T-SEC-04 — Headers Traefik (X-Forwarded-*, X-Real-IP)

```bash
# Vérifier que Traefik transmet les headers de proxy
curl -s -D- https://herald.home.kolapsis.com/health 2>&1 | grep -i "x-"
# Vérifier qu'aucun header sensible ne fuite
```

### T-SEC-05 — CORS / méthodes HTTP

```bash
# OPTIONS sur /mcp → pas d'erreur 405
curl -s -X OPTIONS https://herald.home.kolapsis.com/mcp -w "HTTP: %{http_code}\n" -o /dev/null

# GET sur /mcp → pas supporté (MCP = POST only pour tools/call)
curl -s -X GET https://herald.home.kolapsis.com/mcp -w "HTTP: %{http_code}\n" -o /dev/null
```

---

## 8. Test E2E complet — Claude Chat Custom Connector

> Ce test valide le flow réel utilisé par Claude Chat.

### Pré-requis

1. Herald tourne sur la workstation (`herald serve`)
2. Traefik route `herald.home.kolapsis.com` vers Herald
3. Claude Chat configuré avec un Custom Connector :
   - **Server URL** : `https://herald.home.kolapsis.com`
   - **Client ID** : `herald-claude-chat`
   - **Client Secret** : `$HERALD_CLIENT_SECRET`

### Scénario

| #   | Action (dans Claude Chat)                                                                             | Vérification côté Herald                                           |
| --- | ----------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| 1   | Ajouter le Custom Connector dans Claude Chat                                                          | OAuth discovery OK, pas d'erreur                                   |
| 2   | Demander : *"Utilise l'outil list_projects"*                                                          | Logs : `tools/call list_projects`, réponse avec le projet `herald` |
| 3   | Demander : *"Lance une tâche sur herald : lis le fichier CLAUDE.md et fais-en un résumé en 3 points"* | Logs : `start_task` → task_id créé, Claude Code lancé              |
| 4   | Demander : *"Vérifie l'avancement"*                                                                   | Logs : `check_task` → status running/completed                     |
| 5   | Demander : *"Donne-moi le résultat"*                                                                  | Logs : `get_result` → résumé affiché dans Claude Chat              |
| 6   | Demander : *"Lis le fichier go.mod du projet"*                                                        | Logs : `read_file` → contenu affiché                               |
| 7   | Demander : *"Montre-moi les dernières tâches"*                                                        | Logs : `list_tasks` → historique affiché                           |

### Points de vérification

- [ ] OAuth flow transparent (pas de prompt de login visible pour l'utilisateur)
- [ ] Claude Chat affiche le nom des outils quand il les appelle
- [ ] Les réponses sont formatées lisiblement (emojis, puces, blocs de code)
- [ ] Le refresh token fonctionne silencieusement (session > 1h)
- [ ] Pas de latence excessive (< 2s pour les outils non-task, < 5s pour start_task)
- [ ] Les logs Herald montrent chaque requête avec client_id et scope

---

## 9. Matrice de couverture

| Zone                | Test                   | Statut     |
| ------------------- | ---------------------- | ---------- |
| **Infra**           | DNS                    | T-INFRA-01 |
|                     | TLS/Let's Encrypt      | T-INFRA-02 |
|                     | Health check           | T-INFRA-03 |
|                     | Bind localhost only    | T-INFRA-04 |
| **OAuth**           | Discovery metadata     | T-AUTH-01  |
|                     | Auth code flow         | T-AUTH-02  |
|                     | PKCE S256              | T-AUTH-03  |
|                     | Refresh + rotation     | T-AUTH-04  |
|                     | Bad credentials        | T-AUTH-05  |
|                     | Code usage unique      | T-AUTH-06  |
| **MCP**             | Sans auth → 401        | T-MCP-01   |
|                     | Initialize             | T-MCP-02   |
|                     | tools/list (9 outils)  | T-MCP-03   |
| **Outils**          | list_projects          | T-TOOL-01  |
|                     | start/check/get_result | T-TOOL-02  |
|                     | list_tasks             | T-TOOL-03  |
|                     | cancel_task            | T-TOOL-04  |
|                     | read_file (valide)     | T-TOOL-05  |
|                     | read_file (traversal)  | T-TOOL-06  |
|                     | get_diff               | T-TOOL-07  |
|                     | get_logs               | T-TOOL-08  |
| **Sécurité**        | Token expiré           | T-SEC-01   |
|                     | Refresh invalide       | T-SEC-02   |
|                     | Path traversal         | T-SEC-03   |
|                     | Headers proxy          | T-SEC-04   |
|                     | Méthodes HTTP          | T-SEC-05   |
| **E2E Claude Chat** | Flow complet 7 étapes  | Section 8  |

---

## 10. Script d'exécution automatisé

```bash
#!/bin/bash
# scripts/e2e-test.sh — Exécute les tests E2E Herald
set -euo pipefail

BASE="https://herald.home.kolapsis.com"
CLIENT_ID="herald-claude-chat"
CLIENT_SECRET="${HERALD_CLIENT_SECRET:?missing}"

pass() { echo "  ✅ $1"; }
fail() { echo "  ❌ $1"; FAILURES=$((FAILURES+1)); }
FAILURES=0

echo "=== Herald E2E Tests ==="
echo "Base: $BASE"
echo ""

# --- INFRA ---
echo "🔧 Infrastructure"
HTTP=$(curl -so /dev/null -w '%{http_code}' "$BASE/health")
[ "$HTTP" = "200" ] && pass "T-INFRA-03 Health check" || fail "T-INFRA-03 Health check (got $HTTP)"

# --- OAUTH ---
echo "🔐 OAuth 2.1"

META=$(curl -sf "$BASE/.well-known/oauth-authorization-server")
printf '%s\n' "$META" | jq -e '.issuer' >/dev/null && pass "T-AUTH-01 Discovery metadata" || fail "T-AUTH-01 Discovery metadata"

REDIRECT=$(curl -s -o /dev/null -w '%{redirect_url}' \
  "$BASE/oauth/authorize?response_type=code&client_id=$CLIENT_ID&redirect_uri=https://callback.test/cb&state=e2e")
CODE=$(printf '%s\n' "$REDIRECT" | grep -oP 'code=\K[^&]+' || true)
[ -n "$CODE" ] && pass "T-AUTH-02 Authorize → code" || fail "T-AUTH-02 Authorize → code"

TOKEN_RESP=$(curl -sf -X POST "$BASE/oauth/token" \
  -d "grant_type=authorization_code&code=$CODE&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET")
TOKEN=$(printf '%s\n' "$TOKEN_RESP" | jq -r '.access_token // empty')
[ -n "$TOKEN" ] && pass "T-AUTH-02 Token exchange" || fail "T-AUTH-02 Token exchange"

# --- MCP ---
echo "📡 MCP Protocol"

HTTP=$(curl -so /dev/null -w '%{http_code}' -X POST "$BASE/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
[ "$HTTP" = "401" ] && pass "T-MCP-01 No auth → 401" || fail "T-MCP-01 No auth → 401 (got $HTTP)"

# Initialize : capturer le Mcp-Session-Id
INIT_RESP=$(curl -s -D - -X POST "$BASE/mcp" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"e2e-test","version":"1.0"}}}')
SESSION_ID=$(printf '%s\n' "$INIT_RESP" | grep -i 'Mcp-Session-Id' | tr -d '\r' | awk '{print $2}')
[ -n "$SESSION_ID" ] && pass "T-MCP-02 Initialize + session" || fail "T-MCP-02 Initialize + session"

TOOLS=$(curl -sf -X POST "$BASE/mcp" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')
COUNT=$(printf '%s\n' "$TOOLS" | jq '.result.tools | length')
[ "$COUNT" = "9" ] && pass "T-MCP-03 tools/list (9 tools)" || fail "T-MCP-03 tools/list ($COUNT tools)"

# --- TOOLS ---
echo "🛠️  MCP Tools"

mcp_call() {
  curl -sf -X POST "$BASE/mcp" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":99,\"method\":\"tools/call\",\"params\":{\"name\":\"$1\",\"arguments\":$2}}"
}

PROJ=$(mcp_call "list_projects" '{}')
printf '%s\n' "$PROJ" | jq -e '.result.content[0].text' >/dev/null && pass "T-TOOL-01 list_projects" || fail "T-TOOL-01 list_projects"

READF=$(mcp_call "read_file" '{"project":"herald","path":"go.mod"}')
printf '%s\n' "$READF" | jq -r '.result.content[0].text' | grep -q "kolapsis/herald" && pass "T-TOOL-05 read_file" || fail "T-TOOL-05 read_file"

TRAV=$(mcp_call "read_file" '{"project":"herald","path":"../../../etc/passwd"}')
printf '%s\n' "$TRAV" | jq -r '.result.content[0].text' | grep -qi "denied\|traversal" && pass "T-SEC-03 Path traversal" || fail "T-SEC-03 Path traversal"

echo ""
echo "=== Résultat : $FAILURES échec(s) ==="
exit $FAILURES
```

> Rendre exécutable : `chmod +x scripts/e2e-test.sh`
> Lancer : `HERALD_CLIENT_SECRET=xxx ./scripts/e2e-test.sh`

---

## 11. Critères de validation

Le déploiement est validé quand **tous** les tests passent :

1. **Infrastructure** : TLS valide, health OK, bind localhost vérifié
2. **OAuth** : flow complet (authorize → token → refresh → rotation) sans erreur
3. **MCP** : 9 outils listés, appels authentifiés fonctionnels
4. **Sécurité** : path traversal bloqué, tokens invalides rejetés, auth obligatoire
5. **E2E Claude Chat** : conversation fluide avec au moins 1 tâche complétée
