# AGENTS.md — plugin-cardwire

Standalone out-of-tree plugin repo serving the `charly cardwire` GPU-manager
surface (`command:cardwire`). The plugin is a Go module at
`candy/plugin-cardwire/` (module path
`github.com/opencharly/plugin-cardwire/candy/plugin-cardwire`); the root
`charly.yml` declares `discover: candy` (so the repo is a project and its candy
is scanned) and the `check-cardwire-local` R10 witness bed.

Canonical files:

- `candy/plugin-cardwire/charly.yml` — the `plugin-cardwire:` candy entity and
  the embedded `cardwire-skill:` skill entity.
- `candy/plugin-cardwire/command.go` — the `command:cardwire` kong CLI tree.
- `candy/plugin-cardwire/plugin.go` / `provider.go` — `NewProvider()` /
  `NewMeta()` / `CliMain`.
- `candy/plugin-cardwire/schema/cardwire.cue` — the self-contained plugin schema.
- `candy/cardwire-install/charly.yml` — the CachyOS/Arch install plan candy (the
  only install surface).
- `charly.yml` — the root manifest (`discover: candy`) + the
  `check-cardwire-local` witness bed.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-cardwire:cardwire` — the `charly cardwire` command reference
  (projected from the embedded `cardwire-skill:` entity). Load before changing
  the command tree.
- `/charly-bpf:bpf` — the canonical BPF-LSM readiness surface this plugin's
  status reads.
- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the `command` provider class, the per-plugin CUE-schema contract.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-cardwire/` — compile the plugin module.
- `go test ./...` in `candy/plugin-cardwire/` — the plugin's hermetic tests (the
  two-GPU `list --json` fixture, TOML roundtrips, block/unblock argv + verify).
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- The R10 witness is the `check-cardwire-local` bed in the root `charly.yml` — a
  host-side run of the deterministic graceful-N/A exit-code contracts.

## Modify this repo

- Edit the `plugin-cardwire:` candy entity, the Go source, and
  `schema/cardwire.cue` **together** — the schema is the single source for the
  plugin's served declaration surface.
- Keep the `cardwire-skill:` entity in step with any command-tree change — it is
  the projected source for `/charly-cardwire:cardwire`.
- The plugin candy carries NO install content; install changes belong in
  `candy/cardwire-install/`.

## Landing

- PR-only. Every change lands through a pull request; the org-required
  `charly/pr-validator` validates the diff and body and arms native auto-merge on
  PASS. Direct pushes to `main` are blocked.
- History lives in `CHANGELOG/` (written by `tag-on-merge` at merge time); the PR
  body IS the changelog.
- The authoritative rulebook is the umbrella `AGENTS.md` in
  `opencharly/opencharly` and `charly/AGENTS.md` in the charly repo. Do not
  restate its rules here.
