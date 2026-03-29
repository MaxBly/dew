# Rules — Dew

> Read when planning or writing Dew code.

## public / perso separation
- Everything going into `main` (public): zero hardcoded domain, path, or credential
- Anything specific to bly-net: `perso` branch only
- Before implementing a feature, decide explicitly: main or perso?

## Compatibility ezserv → Dew
- JSON data files (`movies.json`, `series.json`, `watch_history.json`, `tokens.json`) are directly compatible
- Routes `/api/stream/` and `/api/watch/` must be preserved to avoid breaking existing clients

## Addons
- Each addon exposes `Register(e *echo.Echo, cfg *config.Config) error` — no cross-addon dependencies
- The seedbox addon (qBittorrent, linking, requests) is `perso` only

## Language
- All code, comments, logs, and API responses in English
- i18n is an addon — core UI strings are English keys
