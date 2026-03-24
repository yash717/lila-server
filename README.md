# Nebula Strike — Backend Server

Multiplayer Tic-Tac-Toe backend powered by **Nakama** game server with a **Go** runtime module.

Configuration at runtime comes from **environment variables** only. The image renders `config/config.template.yml` via `scripts/docker-entrypoint.sh` and `scripts/render-config.pl` (no secrets committed).

## Production (AWS EC2 + Neon Postgres)

1. In **Neon**, create a database (or use the default `neondb`) and copy the connection details.
2. Convert Neon’s `postgresql://…` URL into Nakama’s **`DATABASE_ADDRESS`** form (not a full URL with scheme):

   `username:password@host:port/database?sslmode=require`

   Example:

   `neondb_owner:YOUR_PASSWORD@ep-xxxx.us-east-1.aws.neon.tech:5432/neondb?sslmode=require`

   If `nakama migrate` fails with SSL errors, try removing `channel_binding=require` from Neon’s query string and keep `sslmode=require` only. If the password contains `@` or other reserved characters, change it in Neon or use URL-encoded credentials per your driver docs.

3. Copy `.env.example` to `.env` on the server, set `DATABASE_ADDRESS`, `NAKAMA_SERVER_KEY`, and console credentials. **Never commit `.env`.**
4. Build and run the production stack (no local Postgres container):

   ```bash
   make up-prod
   # or: docker compose -f docker-compose.prod.yml up -d --build
   ```

5. Open the **AWS security group** for TCP **80** (if nginx proxies to Nakama), **7350** (direct Nakama), and **7351** (console; restrict by IP in production).
6. Point the **nebula-strike** build at the same host, port, and **`VITE_NAKAMA_SERVER_KEY`** as `NAKAMA_SERVER_KEY` (if nginx serves Nakama on port 80, use `VITE_NAKAMA_PORT=80`).

**EC2 bootstrap:** `deploy/ec2-server-setup.sh` (Amazon Linux 2023) installs git, Docker, Docker Compose v2, nginx (`:80` → `127.0.0.1:7350`), clones `yash717/lila-server`, and creates `~/lila-server/.env` if missing. Set **`DATABASE_ADDRESS`** for Neon, then run `sudo docker compose -f docker-compose.prod.yml up -d --build` in `~/lila-server`.

**Security:** If any database URL or password was ever pasted into chat or committed, **rotate the Neon password** and keys immediately.

## Architecture

```
nebula-server/
├── main.go                 # Plugin entry (match, leaderboard, RPCs)
├── match/                  # Authoritative match + history
├── rpc/                    # create_match, get_leaderboard, get_match_history
├── scripts/
│   ├── docker-entrypoint.sh
│   ├── render-config.pl    # Safe templating from env
│   └── verify-apis.sh      # HTTP smoke test
├── config/
│   ├── config.template.yml # Rendered at container start (from env)
│   └── local.yml           # Reference only (same defaults as dev)
├── docker-compose.yml      # Local: Postgres + Nakama
├── docker-compose.prod.yml # Prod: Nakama only (external DB)
├── Dockerfile
├── go.mod
└── README.md
```

## Prerequisites

- Docker & Docker Compose
- **Local dev:** **Postgres is included** in `docker-compose.yml` (`postgres:15-alpine`). Host port **`5433`** maps to the container’s `5432`. Credentials: `postgres` / `password`, database `nakama`.
- **Production:** Postgres is **external** (e.g. Neon); set `DATABASE_ADDRESS` in `.env`.

### Go plugin versions

Build the module with **`github.com/heroiclabs/nakama-common`** and **`google.golang.org/protobuf`** versions aligned to the Nakama image (see `go.mod`). A mismatch surfaces in logs as:  
`plugin was built with a different version of package google.golang.org/protobuf/internal/pragma`.

### HTTP server key

Clients must use the same key as **`NAKAMA_SERVER_KEY`** (rendered into `socket.server_key`). If the browser shows **`Server key invalid`**, align `VITE_NAKAMA_SERVER_KEY` with the server and rebuild the client if needed.

## Formatting (Go module)

Format all `.go` files (uses local `go` + `gofmt -s` + `goimports` if installed; otherwise Docker):

```bash
make fmt
# or
make format
```

If you have no Go toolchain, the Makefile runs `go fmt` inside `golang:1.21-bookworm`.

**Docker build** expects `go.sum` present (run `go mod tidy` once, or use `docker run --rm -v "$PWD":/src -w /src golang:1.21-bookworm go mod tidy`).

## Quick Start

```bash
# Build and run Postgres + Nakama (creates DB on first start)
docker compose up --build -d

# Logs
docker compose logs -f nakama

# Dashboard (NAKAMA_CONSOLE_* in .env or defaults admin/password)
open http://localhost:7351
```

Variables in `docker-compose.yml` can be overridden by a project `.env` file (Compose loads it automatically for `${VAR}` substitution).

## Server Endpoints

| Port  | Protocol    | Usage                      |
|-------|-------------|----------------------------|
| 7349  | gRPC        | Server-to-server API       |
| 7350  | HTTP + WS   | Client API (frontend uses) |
| 7351  | HTTP        | Admin Console / Dashboard  |

## Server key

Default dev key: `nebula-strike-dev-key` (override with `NAKAMA_SERVER_KEY` / `VITE_NAKAMA_SERVER_KEY` in production).

## GitHub

This directory is intended to be the root of the **`lila-server`** repository. The UI lives in **`nebula-strike`** (`lila-ui`).

## RPCs

| ID | Description |
|----|-------------|
| `create_match` | Create a match room; JSON body `{ "mode": "classic" \| "timed" }` |
| `get_leaderboard` | Top 20 leaderboard rows |
| `get_match_history` | Authenticated user’s combat history (storage-backed) |

### Frontend usage (`nebula-strike`)

| RPC | UI surface | Client |
|-----|------------|--------|
| `create_match` | Lobby → **CREATE ROOM** (passes mode; server adds **host** display name from account) | `nakamaClient.createMatch` → `session.rpc` |
| `get_leaderboard` | **Rankings** screen | `nakamaClient.getLeaderboard` |
| `get_match_history` | Lobby **Combat History** panel (refreshes when returning from a match) | `nakamaClient.getMatchHistory` |

**Open rooms:** Lobby calls `client.listMatches` (authoritative, filters by match label `open: true` and one player) so other players can join without a pasted code. Direct Access still joins by full match id.

### Database (Postgres via Nakama)

- **Leaderboard scores**: `LeaderboardRecordWrite` in the match handler updates `leaderboard_record` (and related rows) when a game ends.
- **Combat history**: `StorageWrite` in `match/history_storage.go` stores JSON under collection `nebula_strike`, key `combat_history` per user (`storage` table in Postgres).

### End-to-end HTTP check

With Nakama listening on `7350` (e.g. after `docker compose up -d`), or after **`docker compose -f docker-compose.prod.yml up -d`** with a project **`.env`** (the script loads `.env` for `NAKAMA_SERVER_KEY` if unset):

```bash
make verify-apis
# or
./scripts/verify-apis.sh
```

If nginx proxies **port 80** to Nakama (as in `deploy/ec2-server-setup.sh`):

```bash
make verify-apis-nginx
```

This exercises `/v2/account/authenticate/device` and all three RPCs with the same encoding as `@heroiclabs/nakama-js` (double JSON-stringified payload body).

## Matchmaking

The module registers `RegisterMatchmakerMatched` so Nakama can create an authoritative match when two tickets match. Clients should use socket `addMatchmaker` with string property `mode` (`classic` or `timed`) and a query such as `+properties.mode:classic`.

## WebSocket (realtime, port 7350)

The client opens a **WebSocket** after `authenticateDevice` + `socket.connect(session)` (`nebula-strike` → `GameContext`). Used for:

| Feature | Socket API |
|---------|------------|
| Matchmaking | `addMatchmaker` / `removeMatchmaker` (Lobby **FIND MATCH**) |
| Join match | `joinMatch` (after matchmaker, or after room code / RPC join) |
| Moves | `sendMatchState` opcode **1** (`sendMove` in `nakamaClient.ts`) |
| Incoming | `onmatchdata` opcodes **2** (state), **3** (game over); `onmatchmakermatched` |

RPCs stay on **HTTP**; gameplay is **websocket + authoritative match**.

## OpCodes

| Code | Direction        | Description          |
|------|------------------|----------------------|
| 1    | Client → Server  | Player move          |
| 2    | Server → Client  | Game state update    |
| 3    | Server → Client  | Game over result     |
