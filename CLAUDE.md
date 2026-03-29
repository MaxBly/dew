# CLAUDE.md — dew

> Réécriture d'ezserv. Remplaçant en Go + Svelte, modulaire, publiable sur GitHub.
> Contexte machine → `~/CLAUDE.md`

---

## Stack

Go + Echo · TypeScript + Svelte · TOML config · JSON storage · Docker Compose

**Codebase en anglais** — code, commentaires, logs, API. L'i18n est un addon.

---

## Chemins

```
~/dew/           racine projet
~/dew/docs/      documentation Dew
~/dew/data/      JSON stores (runtime)
```

---

## Documentation

| Fichier | Contenu |
|---|---|
| `docs/vision.md` | Concept, addons, auth, permissions, fédération |
| `docs/architecture.md` | Décisions techniques, structure, routes, fédération |
| `docs/roadmap.md` | Phases de développement |
| `docs/rules.md` | Règles public/perso, compatibilité ezserv |

---

## Règles non-négociables

- **Core = strict minimum** : library walk + mediainfo + TMDB + streaming FFmpeg. Rien d'autre.
- **Tout le reste = addon** activable via `dew.toml`
- **Zéro hardcode** dans `main` : aucun domaine, path ou token dans le code
- Chaque addon expose `Register(e *echo.Echo, cfg *config.Config) error` — pas de dépendances croisées
- Seedbox, Matrix theme, bly-net config → branche `perso` uniquement

---

## Git

```
main   → github.com/MaxBly/dew (public)
perso  → privé uniquement (bly-net, Matrix theme)
```

`perso` rebase sur `main` régulièrement.

---

## Prochaine étape immédiate

1. `go mod init github.com/MaxBly/dew`
2. Structure dossiers (`cmd/`, `internal/`, `addons/`, `web/`)
3. `internal/store` + `internal/config`
4. Premier handler Echo : `GET /api/films`
