---
name: ship
description: >-
  Builds, pushes, and deploys Docker images or Go binaries with the ship CLI
  (ship.toml schema 2). Use when the user mentions ship, ship.toml, image build
  and push, compose deploy, rollback, release tags, matrix profiles, or git-tag
  based releases.
version: dev
---

# Ship

CLI for **build → tag → push → deploy → verify**. Config: `ship.toml` at the project root (`schema = 2`).

`ship skill` stamps frontmatter `version` with the current ship binary version. If `build` / `run` / `doctor` warns that the skill is outdated, run `ship skill -f`.

## Default workflow

```bash
ship plan -v v1.0.0
ship doctor -v v1.0.0
ship run -v v1.0.0 -y
```

Or step by step: `build` → `tag` → `push` → `deploy`. Prefer `run` for real releases.

## Command cheat sheet

| Command | Purpose | Key flags |
|---------|---------|-----------|
| `init` | Generate `ship.toml` | `-f` |
| `ai` / `ai init` | Thin release advisor (LLM; optional) | `-p`, `--dry-run`, `-y` |
| `plan` | Show release plan (no side effects) | `-v`, `-p`, `--json` |
| `doctor` | Check release readiness | `-v`, `-p` |
| `build` / `tag` / `push` | Build, tag, publish | `-v`, `-p`, `--promote-latest` (push) |
| `deploy` / `rollback` / `history` | Deploy / rollback / history | `-v`, `-y`, `-n` |
| `run` | Full pipeline | `-v`, `-p`, `--skip-deploy`, `--promote-latest` |
| `current` / `version` / `skill` | Current git tag / ship version / install this skill | `-f` (skill) |

## Hard rules

- When an agent or CI runs a command that may prompt for confirmation, **always pass `-y`**.
- Default `version.source = "git-tag"`: `-v` / `SHIP_VERSION` must be a **real local Git tag**. Builds use the tag source snapshot (worktree); uncommitted changes are not included.
- Standalone `push` / `deploy` / `rollback` consume a **release manifest** under `.ship/releases/`; versions that were never published fail (no silent rebuild from the working tree).
- Default deploy pin is by **digest** (`APP_IMAGE_DIGEST`). Compose image lines must use `@${APP_IMAGE_DIGEST}` (otherwise pin degrades to `tag`). `deploy`/`rollback` do **not** move registry `:latest`; use `--promote-latest` when needed.
- Docker: `build.docker.load = true`, and **single platform** only (e.g. `linux/amd64`).
- Unknown keys in `ship.toml` error by default; relax with `[config] unknown_keys = "warn"` or `SHIP_UNKNOWN_KEYS=warn`.
- `.ship/` is runtime state (runs / releases / history); add it to `.gitignore`.
- `ship ai` is an optional advisor harness (not a substitute for `plan` / `run` / `deploy`). Prefer `ship ai init --dry-run` before writing config; never ask it to run deploy/push for you.

## Dig deeper when needed

- Config fields and drivers: [REFERENCE.md](REFERENCE.md)
- Copy-paste recipes: [EXAMPLES.md](EXAMPLES.md)
- v2.7.1 digest pin hotfix retrospective: `docs/changes/completed/hotfix-v2.7.1-digest-pin.md`
