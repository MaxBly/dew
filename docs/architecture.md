# Dew — Architecture

## Finalized decisions

| Topic | Decision |
|---|---|
| Storage | JSON everywhere — generic `JsonStore[T]`, atomic write (`rename`), `sync.RWMutex` |
| HTTP framework | **Echo** (stdlib-compatible, stable, sufficient perf) |
| Frontend | TypeScript + Svelte |
| Config | **TOML** (`dew.toml`), partial hot-reload |
| Metadata | TMDB optional — fallback to filename parsing if no API key |
| Auth | Abstract provider: `token` / `password` / `both` — swappable via config |
| Permissions | Granular per-token/user (`watch`, `download`, `request`, `request.auto_approve`, `request.choose_release`) |
| Seedbox | `seedbox` addon only — core = streaming + library |
| Library | Films + series in one lib, one token = one user with their own history |
| Player | Integrated fullscreen overlay on main page, no separate page |
| URLs | Readable slugs: `/?play=films/mulholland-drive-2001-12345`, `/?play=series/breaking-bad-1396/s03e08` |
| Slug format | `{normalized-title}-{year}-{tmdb_id}` — always includes TMDB ID to guarantee uniqueness |
| Multiple versions | Grouped by slug, quality selector in UI — `files[]` array per entry |
| Transcoding | `transcoding` addon, toggleable, target resolution configurable (720p/1080p/4k/source) |
| CLI | cobra — `dew serve`, `dew token create/list`, `dew library scan` |
| Admin | `admin` addon — web UI for config + token management |
| i18n | `i18n` addon — English default, French available. Core always in English. |
| Language | Codebase in English (code, comments, logs, API) |

---

## Directory structure

```
dew/
├── cmd/dew/
│   └── main.go                  # cobra: serve | token | library | config
│
├── internal/
│   ├── config/                  # dew.toml (TOML, partial hot-reload)
│   ├── store/                   # JsonStore[T] generic, atomic write, RWMutex
│   ├── library/                 # filesystem walk, slugs, mtime cache, version grouping
│   ├── streaming/               # FFmpeg pipe, fragmented MP4, backpressure
│   ├── auth/                    # abstract AuthProvider, token + password implementations
│   ├── watch/                   # watch history per token (position, audio, sub)
│   ├── events/                  # global SSE broker
│   └── api/                     # Echo handlers
│       ├── stream.go
│       ├── library.go
│       ├── watch.go
│       ├── auth.go
│       └── admin.go
│
├── addons/
│   ├── addon.go                 # Addon interface { Name() string; Register(*echo.Echo) }
│   ├── transcoding/             # VAAPI/QSV/software, target resolution
│   ├── subtitles/               # VTT extraction, /tmp cache
│   ├── download/                # direct download links
│   ├── vlc/                     # VLC M3U playlists
│   ├── requests/                # user request system + Prowlarr automation
│   ├── seedbox/                 # qBittorrent monitoring + symlink management
│   ├── streams/                 # active stream list, kill, per-user/IP limits
│   ├── logs/                    # append-only JSON log, rotation, admin viewer
│   ├── arr/                     # Radarr/Sonarr webhook endpoint
│   ├── i18n/                    # translation loader, locale selector
│   ├── admin/                   # admin web UI
│   └── federation/              # P2P library sharing, hub mode, federated tokens
│
├── web/src/
│   ├── lib/
│   │   ├── Library.svelte       # browsing + integrated player overlay
│   │   ├── Player.svelte        # fullscreen overlay, no-reload navigation
│   │   └── Admin.svelte         # config UI, tokens, library (admin addon)
│   ├── locales/
│   │   ├── en.json              # reference — always complete
│   │   └── fr.json
│   └── themes/
│       ├── default/             # main (public) — clean, Netflix-like
│       └── ...                  # editorial, spatial, mood, timeline (future)
│
├── data/
│   ├── movies.json              # keyed by slug
│   ├── series.json              # keyed by slug
│   ├── seasons.json             # keyed by series slug → season → episodes (no mediainfo)
│   ├── tokens.json              # token, label, permissions, max_streams, expires_at
│   ├── watch_history.json       # keyed by token → slug → { file, position, … }
│   ├── watchlist.json           # keyed by token → [slug, …]
│   ├── cache/
│   │   └── mediainfo.json       # keyed by absolute file path
│   └── addons/                  # each addon owns its namespace
│       ├── logs/
│       ├── requests/
│       └── seedbox/
│
├── dew.toml
├── Dockerfile
└── docker-compose.yml
```

---

## Addon interface

```go
type Addon interface {
    Name() string
    Register(e *echo.Echo, cfg *config.Config) error
}
```

Core loads enabled addons from `dew.toml` at startup, calls `Register` on each.
Addons can add routes, middleware, background goroutines.
Core never imports addon packages directly — registration is side-effect driven.

---

## Auth abstraction

```go
type AuthProvider interface {
    Authenticate(c echo.Context) (*User, error)  // validate request → user
    IssueSession(c echo.Context, user *User) error // set cookie
    Middleware() echo.MiddlewareFunc
}

type User struct {
    ID          string
    Label       string
    IsAdmin     bool
    Permissions []string
    MaxStreams   int
}
```

`TokenProvider` and `PasswordProvider` both implement `AuthProvider`.
`BothProvider` wraps both, tries token first then password.

---

## API routes — core

```
GET  /api/films                        # film list (slugs, metadata, versions)
GET  /api/series                       # series list
GET  /api/stream/films/:slug           # FFmpeg stream (?q=4k&audio=1&start=3600)
GET  /api/stream/series/:slug/:ep      # episode stream
GET  /api/watch/continue               # continue-watching list
POST /api/watch/progress               # save position
POST /api/auth/login                   # login → signed cookie
POST /api/auth/logout
```

## API routes — addons (when enabled)

```
GET  /api/media/info/:type/:slug       # mediainfo (codecs, tracks, versions)
GET  /api/stream/subs/:type/:slug      # VTT subtitle extraction    [subtitles]
GET  /api/download/:type/:slug         # direct download link       [download]
GET  /api/vlc/:type/:slug              # VLC M3U playlist           [vlc]
GET  /api/requests                     # list requests              [requests]
POST /api/requests                     # submit request             [requests]
POST /api/webhooks/arr                 # Radarr/Sonarr webhook      [arr]
GET  /api/federation/catalog           # expose signed catalog to peers  [federation/node]
GET  /api/federation/info              # node name, version, public URL  [federation/node]
GET  /api/federation/nodes             # list connected nodes            [federation/hub]
GET  /api/federation/catalog/merged    # aggregated catalog (hub only)   [federation/hub]
GET  /api/admin/streams                # active streams             [streams]
DELETE /api/admin/streams/:id          # kill stream                [streams]
GET  /api/admin/logs                   # query logs                 [logs]
GET  /api/admin/config                 # read config                [admin]
PUT  /api/admin/config                 # update config              [admin]
POST /api/admin/library/scan           # rescan library             [admin]
GET  /api/admin/tokens                 # list tokens                [admin]
POST /api/admin/tokens                 # create token               [admin]
DELETE /api/admin/tokens/:token        # revoke token               [admin]
```

---

## Federation architecture

### Topology

```
[Hub — VPS dew.bly-net.com]        mode: hub
  No local library
  Aggregates catalogs from nodes
  Manages shared users (tokens valid on all nodes)
  Returns stream URLs — never proxies video
        ↑  HMAC-signed API          ↑  HMAC-signed API
[Node — Mini PC]                [Node — Friend's server]
  mode: node                      mode: node
  Own library                     Own library
  Own users                       Own users
  Exposes /api/federation/*       Exposes /api/federation/*
```

### Federated stream token

Hub issues a short-lived HMAC token when a user requests a stream:

```
fed_token = HMAC-SHA256(slug + ":" + user_id + ":" + exp_unix, shared_secret)
```

Node validates locally — no round-trip to hub at stream time.
Token expires in 60s (enough for client to initiate the stream).

### Node-to-node authentication

Every inter-node request includes:
```
X-Dew-Node: max
X-Dew-Sig:  HMAC-SHA256(method + path + timestamp, shared_secret)
X-Dew-Time: 1743300000
```

Node rejects requests older than 30s (replay protection).

### Catalog sync

- Hub polls each node `GET /api/federation/catalog` every 60s (or on push notification)
- Delta sync via `?since=<unix_timestamp>` — only changed entries
- Merged in memory, cached to `data/federation_cache.json`
- Deduplication by TMDB ID: same film on multiple nodes → single entry, source selector in UI

### Hub resource usage (VPS: 1 core / 2 GB / 3 TB/month)

| Traffic | Estimate |
|---|---|
| Catalog JSON | ~500 KB total |
| API + auth requests | ~50 MB/month |
| Static frontend | ~2 MB/visit |
| **Video (redirected to nodes)** | **0 bytes** |
| **Total** | **< 10 GB/month** |

---

## FFmpeg transcoding — VAAPI (N95)

N95 Intel UHD GPU supports VAAPI/QuickSync.

```bash
# H.265 → H.264 via VAAPI
ffmpeg -hwaccel vaapi -hwaccel_device /dev/dri/renderD128 \
  -i input.mkv \
  -map 0:v:0 -map 0:a:N \
  -c:v h264_vaapi -b:v 8M -maxrate 12M \
  -c:a aac -b:a 192k \
  -movflags frag_keyframe+empty_moov -f mp4 pipe:1
```

Logic: detect `video_codec == "H.265"` from mediainfo cache → VAAPI pipeline, else `-c:v copy`.
Docker: pass `/dev/dri` as volume if running in container.

---

## Compatibility with ezserv

- JSON files (`movies.json`, `series.json`, `watch_history.json`) — same format, direct copy
- Routes `/api/stream/`, `/api/watch/` preserved
- `tokens.json` importable as-is
