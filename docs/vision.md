# Dew — Vision

> Previously named "Nidus". Renamed to "Dew" (2026-03-29).

## Concept

Complete rewrite of ezserv as **Dew**: a modular, self-hosted media streaming service.
Publishable on GitHub, installable by anyone with a single Docker Compose.

**Core philosophy:** transform a file library into a streaming service — nothing more.
Everything else is an addon.

---

## Language

- **Codebase:** English (code, comments, logs, API responses)
- **UI default:** English
- **Translations:** addon `i18n` — admin selects language (English / French, extensible)

---

## Stack

| Layer | Technology |
|---|---|
| **Backend** | Go — Echo framework |
| **Frontend** | TypeScript + Svelte |
| **Config** | TOML (`dew.toml`) |
| **Storage** | JSON — generic `JsonStore[T]` |
| **Deployment** | Docker Compose (1–2 containers) |

---

## Core — strict minimum

```
library walk → mediainfo cache → TMDB cache → FFmpeg streaming
```

No admin UI, no user management, no transcoding. A single token or open access.
Enough to stream a library from anywhere.

Anything beyond this is an addon.

---

## Addon system

Addons register on the router at startup if enabled in config.
The core only knows the addon interface, not the implementation.

```toml
# dew.toml
[addons]
enabled = ["admin", "transcoding", "subtitles", "download", "vlc", "requests", "seedbox", "streams", "logs", "i18n", "arr"]
```

| Addon | Description |
|---|---|
| `admin` | Web admin UI, config management, user/token management |
| `transcoding` | VAAPI / software transcoding, target resolution |
| `subtitles` | VTT extraction, /tmp cache |
| `download` | Direct download links |
| `vlc` | VLC M3U playlists |
| `requests` | User request interface + automation (Prowlarr) |
| `seedbox` | qBittorrent monitoring + Prowlarr + symlink management |
| `streams` | Active stream management, kill streams, per-user/IP limits |
| `logs` | Append-only action log, IP tracking, admin UI viewer |
| `i18n` | UI translations (English default, French available) |
| `arr` | Radarr/Sonarr webhook integration |
| `themes` | Complete UI replacement via alternate frontend build |
| `federation` | P2P library sharing between Dew instances + optional hub mode |

---

## Authentication

Abstract provider — swappable via config, rest of code is unaware of which is active.

```toml
[auth]
provider = "token"   # "token" | "password" | "both"
```

| Provider | Behavior |
|---|---|
| `token` | Hex token → signed cookie. No account needed. Shareable link. |
| `password` | Classic username/password, bcrypt, session cookie |
| `both` | Both coexist — some users have accounts, others use tokens |

---

## Permissions

Granular per-token/user. No fixed roles — compose freely.

```json
{
  "token": "abc123",
  "label": "Sophie",
  "permissions": ["watch", "download", "request", "request.auto_approve"],
  "max_streams": 2
}
```

| Permission | Description |
|---|---|
| `watch` | Access library + streaming |
| `download` | Direct download links |
| `request` | Submit a content request |
| `request.auto_approve` | Requests approved without admin validation |
| `request.choose_release` | Can select release from Prowlarr results |

Admin has all permissions implicitly.

---

## Player

Custom Svelte player, overlay fullscreen on the library page — no page navigation.
Same page: library grid + watchlist + continue watching sections.
Player state updates without reload.

---

## Addon: `streams`

Exposes the core's active stream map to admin.

```
GET    /api/admin/streams       → list active streams { token, label, ip, title, started_at, bitrate }
DELETE /api/admin/streams/:id   → kill a stream
```

Limits configurable globally or per-user:

```toml
[addons.streams]
max_per_ip   = 3
max_per_user = 2   # overridden per-user if set in their token config
```

---

## Addon: `logs`

Append-only JSON log. Every significant action recorded.

```json
{ "ts": "2026-03-29T21:00:00Z", "ip": "82.x.x.x", "user": "Sophie", "action": "stream.start", "detail": "Dune (4K)" }
```

Actions logged:
`auth.login` `auth.fail` `stream.start` `stream.stop` `stream.kill`
`download` `request.create` `request.approve` `token.create` `token.revoke` `admin.config_change`

Auto-rotation configurable (default 7 days). Filterable by IP / user / action / date in admin UI.

---

## Addon: `arr`

Radarr/Sonarr webhook integration. Dew exposes an endpoint; configure Radarr/Sonarr to call it on download complete.

```
POST /api/webhooks/arr
{ "eventType": "Download", "movie": { "filePath": "/srv/media/Films/..." } }
```

Dew receives path → runs mediainfo + TMDB fetch → adds to library instantly.
No polling, no coupling to Radarr/Sonarr internals.

---

## Addon: `i18n`

UI translations. Core and logs remain in English always.

```toml
[addons.i18n]
locale = "fr"   # "en" (default) | "fr"
```

Translation files: `web/src/locales/en.json` (reference, always complete), `fr.json`.
Svelte store `$t("key")` — lightweight in-house implementation.

---

## Addon: `federation`

Share libraries between Dew instances. Two modes, same addon.

### Mode `node` — personal instance

Exposes a signed catalog API to authorized peers. Validates federation stream tokens locally (no hub call needed at stream time).

```toml
[addons.federation]
mode       = "node"
node_name  = "max"
public_url = "https://dew.minipc.bly-net.com"

[[addons.federation.peers]]
name   = "ami"
url    = "https://dew.ami.com"
secret = "shared_secret_abc"

[[addons.federation.peers]]
name   = "hub"
url    = "https://dew.bly-net.com"
secret = "shared_secret_xyz"
```

### Mode `hub` — VPS aggregator

No local library. Aggregates catalogs from all nodes, manages shared users (tokens valid on all nodes), serves unified UI. **Never proxies video** — returns stream URL pointing to the source node. Client connects directly.

```toml
[addons.federation]
mode = "hub"

[[addons.federation.peers]]
name   = "max"
url    = "https://dew.minipc.bly-net.com"
secret = "shared_secret_xyz"

[[addons.federation.peers]]
name   = "ami"
url    = "https://dew.ami.com"
secret = "shared_secret_abc"
```

### Stream flow from hub

```
Client  → GET  hub/api/stream/films/dune
Hub     → 200  { stream_url: "https://dew.minipc.bly-net.com/api/stream/films/dune", fed_token: "xxx" }
Client  → streams directly from Mini PC — VPS carries zero video data
Mini PC → validates fed_token = HMAC(slug + user_id + exp, shared_secret) locally
```

### Bandwidth on VPS (1 core / 2 GB / 3 TB/month)

| Traffic type | Estimated volume |
|---|---|
| Catalog JSON (1000 films) | ~500 KB |
| API requests (100 active users/day) | ~50 MB/month |
| Static frontend | ~2 MB/visit |
| **Video** | **0 bytes** |
| **Total** | **< 10 GB/month** |

### Deduplication

Same film on multiple nodes → merged by TMDB ID in the hub catalog. UI shows a single entry with a source selector (quality / node).

---

## Addon: `themes`

Complete UI replacement via alternate frontend build. Theme selected in config.

```toml
[addons.themes]
theme = "default"   # "default" | "editorial" | "spatial" | "mood" | "timeline" | custom
```

Themes explored: default (Netflix-like), editorial (magazine asymmetric), spatial (2D pan/zoom map), mood-first (filter by feeling), timeline (horizontal scroll by year).

---

## Git strategy — private + public remote

```
repo: dew (private)
├── main   → pushed to github.com/xxx/dew (public)
└── perso  → private only
```

**Rule:** useful to others → `main` + public push. Personal features (bly-net, Matrix theme) → `perso` branch only.
`perso` rebases on `main` regularly.

### What belongs in `main` (public)
- Core streaming + library
- All addons listed above (generic, zero hardcode)
- Default theme
- English + French translations
- Docker Compose
- CLI: `dew serve`, `dew token`, `dew library`

### What belongs in `perso` only
- Matrix theme
- bly-net.com specific config
- Any hardcoded personal integrations

---

## Compatibility with ezserv

- Same JSON format (`movies.json`, `series.json`, `watch_history.json`) → direct migration
- Routes `/api/stream/`, `/api/watch/` preserved for existing clients
- Existing tokens importable from `tokens.json`
