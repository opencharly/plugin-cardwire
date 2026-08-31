# plugin-cardwire

The charly COMMAND-class plugin for the **cardwire** GPU-manager surface —
[ogc/cardwire](https://github.com/opengamingcollective/cardwire), the Open Gaming
Collective's eBPF/LSM GPU-blocking daemon for CachyOS/Arch. It serves the externalized
`charly cardwire …` CLI (command:cardwire) as a standalone Go module, mirroring the
plugin-udev / plugin-example-command command-only precedents.

## Surfaces

- **`charly cardwire status [--json]`** — deterministic host report, GPU-less graceful
  (exit 0 ALWAYS): the cardwired systemd state (systemctl is-active cardwired →
  active/inactive/N/A), the /etc/cardwire/cardwire.toml whitelisted keys
  (auto_apply_gpu_state, experimental_nvidia_block, battery_auto_switch,
  battery_auto_switch_mode, external_display_auto_switch), and the BPF-LSM facts read
  DIRECTLY from /sys/kernel/security/lsm (bpf present?) — the same canonical reads
  plugin-bpf's status performs (R3: no ad-hoc copies, no shelling out to charly bpf;
  the plugin is self-contained).
- **`charly cardwire list [--json]`** — lists the GPUs cardwired manages: fork/execs
  `/usr/bin/cardwire list --json`, parses the REAL cardwire shape (a map keyed
  "0".."N" with id/name/pci/render/card/default/discrete/virtual_gpu/available/
  vendor/driver/blocked/launchable/nvidia/nvidia_minor — the two-GPU fixture captured
  in the Phase-0 spike: Virtio 1.0 GPU id 0 + NVIDIA GeForce RTX 4080 SUPER id 1) and
  normalizes it into stable keys. A MISSING cardwire install is graceful: explicit
  `CARDWIRE UNAVAILABLE` N/A lines, exit 0 (the host-independent bed assertion); a
  REAL failure of an installed cardwire (daemon down, unparsable output) exits 1 with
  prose.
- **`charly cardwire config get|set <key> [value]`** — read/write
  /etc/cardwire/cardwire.toml with an ATOMIC tmp+rename write (mirroring cardwire's
  own approach). Keys are whitelisted; `set` without a value prints the current value;
  a missing root/file is a graceful N/A, exit 0.
- **`charly cardwire gpu <id> block|unblock`** — THE enable/disable surface: forks
  `/usr/bin/cardwire gpu <id> --block/--unblock`, then VERIFIES via
  `cardwire list --json` that the GPU's blocked flag actually flipped (the daemon
  enforces mode-dependent semantics; verification is the honest contract). Prints
  `GPU <id> BLOCKED`/`GPU <id> UNBLOCKED` + the verify result; exit 0 on success, 1
  with prose on failure (cardwire not installed → `cardwire UNAVAILABLE` — a mutating
  surface refuses, never pretends).
- **`charly cardwire manager status`** — passthrough to `cardwire manager status`
  (read-only; absent cardwire → graceful N/A, exit 0).

## Install plan (CachyOS/Arch deploy scope)

The CachyOS-first install lives in the sibling **`candy/cardwire-install/`** candy (the
ONLY install surface): append the `[ogc]` pacman repo to /etc/pacman.conf, trust the OGC
signing key F79100EF8C802DAB81C323BB8EEA5962FE510E19, `pacman -Syu ogc/cardwire`, write
the /etc/cardwire/cardwire.toml defaults (experimental_nvidia_block=true), enable+start
cardwired, and add the primary user to the video,render groups. Plan checks verify the
binary, the active service, the config default and the bpf LSM gate. The plugin candy
itself carries NO install content, so the check-cardwire-local R10 bed's host deploy is
NON-MUTATING; the check-cardwire-vm bed composes both candies.

## Layout

- `candy/plugin-cardwire/` — the plugin module (command-only: `command:cardwire`; no install content).
- `candy/cardwire-install/` — the CachyOS/Arch install plan candy (the only install surface).
- `cmd/serve/main.go` — the dual-mode sdk.Main entrypoint.

## Verification

- `go test ./...` + `go build ./...` + `go vet ./...` in `candy/plugin-cardwire/`
  (hermetic tests: the real two-GPU list --json fixture, TOML roundtrips on temp
  files, block/unblock argv + verify against a fake /usr/bin/cardwire on PATH, and the
  graceful N/A paths).
- `check-cardwire-local` (opencharly/charly, goal task 4) — host-side R10 witness for
  the `charly cardwire` dispatch with the deterministic graceful-N/A exit-code
  contracts on a cardwire-less host.
