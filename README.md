# plugin-cardwire

The `charly cardwire` GPU-manager surface for OpenCharly — served as a charly
`command:cardwire` plugin.

[ogc/cardwire](https://github.com/opengamingcollective/cardwire) is the Open
Gaming Collective's eBPF/LSM GPU-blocking daemon (CachyOS/Arch first). This
plugin drives it from the charly CLI. Confirm BPF-LSM readiness first with
`charly bpf lsm`.

## What it provides

| Capability | Surface |
|---|---|
| `command:cardwire` | the `charly cardwire` CLI — `status`, `list`, `config`, `gpu block|unblock`, `manager` |

## The command

- **`charly cardwire status [--json]`** — a deterministic, GPU-less-graceful host
  report (exit 0 ALWAYS): the `cardwired` systemd state, the
  `/etc/cardwire/cardwire.toml` whitelisted keys, and the BPF-LSM facts read
  directly from `/sys/kernel/security/lsm`.
- **`charly cardwire list [--json]`** — the GPUs cardwired manages, normalized
  from `cardwire list --json`. A missing cardwire install is graceful
  (`CARDWIRE UNAVAILABLE` N/A lines, exit 0); a real failure of an installed
  cardwire exits 1.
- **`charly cardwire config get|set <key> [value]`** — read/write
  `/etc/cardwire/cardwire.toml` with an atomic tmp+rename write; missing
  root/file is a graceful N/A, exit 0.
- **`charly cardwire gpu <id> block|unblock`** — the enable/disable surface:
  forks `/usr/bin/cardwire`, then verifies via `cardwire list --json` that the
  blocked flag actually flipped. A missing cardwire refuses (exit 1) — a
  mutating surface never pretends.
- **`charly cardwire manager status`** — passthrough to `cardwire manager status`
  (read-only; absent cardwire → graceful N/A, exit 0).

## How to use it

Compose the plugin candy in a box's `candy:` list:

```yaml
- '@github.com/opencharly/plugin-cardwire/candy/plugin-cardwire:<tag>'
```

The CachyOS/Arch install plan lives in the sibling `candy/cardwire-install/`
candy (the only install surface); the plugin candy itself carries no install
content.

## Layout

- `candy/plugin-cardwire/` — the plugin module: `plugin.go`, `provider.go`,
  `command.go`, `status.go`, `list.go`, `config.go`, `gpu.go`,
  `schema/cardwire.cue`, `cmd/serve/main.go`.
- `candy/cardwire-install/` — the CachyOS/Arch install plan candy.
- `charly.yml` — the root project manifest (`discover: candy`) + the
  `check-cardwire-local` R10 witness bed.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.

## Related

- Owning skill: `/charly-cardwire:cardwire` (projected from the embedded
  `cardwire-skill:` entity).
- `/charly-bpf:bpf` — the canonical BPF-LSM readiness surface.
- [`opencharly/charly`](https://github.com/opencharly/charly) — the charly CLI.
