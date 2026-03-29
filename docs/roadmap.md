# Dew — Roadmap

## Current status

ezserv is **maintenance only** — blocking bugs only. Dew development starts now.
ezserv stays in production until Dew is operational. PM2 switchover: `server.js` → `dew serve`.

---

## Phase 1 — Core (`main`)

Scaffolding + core modules. No addons yet.

- [ ] Scaffolding: `go.mod`, directory structure, cobra CLI skeleton
- [ ] `internal/store` — `JsonStore[T]` generic, atomic write, RWMutex
- [ ] `internal/config` — TOML parser, partial hot-reload
- [ ] `internal/library` — filesystem walk, slugs, mtime cache, version grouping
- [ ] `internal/streaming` — FFmpeg pipe, fragmented MP4, backpressure
- [ ] `internal/auth` — abstract `AuthProvider`, `TokenProvider` implementation
- [ ] `internal/watch` — watch history per token
- [ ] `internal/events` — global SSE broker
- [ ] `internal/api` — Echo handlers (stream, library, watch, auth)
- [ ] Frontend Svelte — Library.svelte + Player.svelte overlay (watchlist + continue)
- [ ] Default theme (Netflix-like, clean)
- [ ] Docker Compose minimal
- [ ] CLI: `dew serve`, `dew token`, `dew library scan`

---

## Phase 2 — Addons (`main`)

Each addon is independent — implement in any order.

- [ ] `admin` — web UI config + token management
- [ ] `transcoding` — VAAPI/software, target resolution
- [ ] `subtitles` — VTT extraction, /tmp cache
- [ ] `download` — direct download links
- [ ] `vlc` — VLC M3U playlists
- [ ] `streams` — active stream list, kill, per-user/IP limits
- [ ] `logs` — append-only log, rotation, admin UI viewer
- [ ] `auth/password` — username/password provider (`both` mode)
- [ ] `requests` — user request interface + Prowlarr automation
- [ ] `seedbox` — qBittorrent monitoring + symlink management
- [ ] `arr` — Radarr/Sonarr webhook endpoint
- [ ] `i18n` — translation loader, `fr.json`
- [ ] `federation` — node mode: catalog API + fed_token validation
- [ ] `federation` — hub mode: catalog aggregation, shared users, stream redirect

---

## Phase 3 — `perso` branch

```bash
git checkout -b perso
```

- [ ] Matrix theme (`web/src/themes/matrix/`)
- [ ] bly-net.com specific config
- [ ] Pre-link automation (port from ezserv)
- [ ] Any hardcoded personal integrations

---

## Phase 4 — Themes (future)

Alternate UI themes as separate frontend builds:

- [ ] `editorial` — asymmetric magazine layout (mockup done)
- [ ] `spatial` — 2D pan/zoom map of the library
- [ ] `mood` — mood-first navigation
- [ ] `timeline` — horizontal scroll by release year

---

## Migration ezserv → Dew

JSON files are compatible — no data migration needed.

1. Copy `data/*.json` to new `data/`
2. Write `dew.toml` (equiv. of `.env`)
3. Verify filesystem paths
4. Switch PM2: `server.js` → `dew serve`

---

## Next immediate step

1. `mkdir ~/dew && cd ~/dew`
2. `go mod init github.com/xxx/dew`
3. Directory structure (`cmd/`, `internal/`, `addons/`, `web/`)
4. `internal/store` + `internal/config` — everything depends on these
5. First Echo handler: `GET /api/films`
