// Package cardwire is the charly COMMAND-class plugin serving the externalized
// `charly cardwire …` CLI — the cardwire GPU-manager surface (ogc/cardwire — the
// Open Gaming Collective's eBPF/LSM GPU-blocking daemon for CachyOS/Arch). It is an
// importable dual-placement command plugin: the SAME NewProvider()/NewMeta()/CliMain
// compile INTO charly in-process when listed in compiled_plugins, or cmd/serve serves
// them OUT-OF-PROCESS (charly fork/execs the binary for command:cardwire dispatch)
// when they are not — placement is invisible above the registry. It is a PURE
// command-only plugin (no gRPC verb), mirroring candy/plugin-udev and
// candy/plugin-example-command, NOT the verb+command plugin-bpf / plugin-mcp
// precedents.
//
// CLI dispatch contract (charly/provider_command_external.go dispatchExternalCommand):
// on `charly cardwire <args…>`, charly RESOLVES this plugin's binary (host-built from
// source, or baked into /usr/lib/charly/plugins by the native package) and
// syscall.Exec's it with the pass-through tokens after the `cardwire` word, in CLI
// mode (the go-plugin handshake cookie is stripped, so sdk.Main runs cliMain instead
// of serving gRPC). The plugin therefore owns real terminal stdio/TTY — mutating
// surfaces (`gpu <id> block|unblock`) fork/exec the real /usr/bin/cardwire against
// the running cardwired daemon.
//
// A command is NOT a gRPC-registry capability (charly fork/execs the binary; it never
// connects over gRPC for a command), so this plugin advertises NO Describe capability —
// its serve half (sdk.Serve, never reached for a command-only plugin) exists only to
// satisfy the dual-mode sdk.Main signature. The candy's plugin.providers declaration
// still lists command:cardwire (that drives the CLI-grammar prescan + the baked
// `.providers` manifest).
//
// Everything is self-contained: status reads the cardwired systemd state, the
// /etc/cardwire/cardwire.toml config and the BPF-LSM facts via /sys/kernel/security/lsm
// DIRECTLY (the same direct reads plugin-bpf's status performs — R3: identical
// canonical reads, no ad-hoc copies, no shelling out to charly bpf).
package cardwire

import (
	"github.com/opencharly/sdk"
	pb "github.com/opencharly/spec/proto"
)

// NewProvider returns the cardwire provider (inert — command-only plugin).
func NewProvider() pb.ProviderServer { return &provider{} }

// NewMeta advertises NO gRPC capability — command:cardwire is CLI-dispatched, not
// resolved through the gRPC provider registry (mirrors plugin-udev; nil schema since
// there is no verb input to validate).
func NewMeta() pb.PluginMetaServer {
	return sdk.NewMeta("2026.243.0001",
		[]sdk.ProvidedCapability{},
		nil)
}

// CliMain is the plugin's CLI entrypoint (command:cardwire dispatch).
func CliMain(args []string) int { return cliMain(args) }
