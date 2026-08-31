package cardwire

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

// command.go is the command:cardwire leg of this plugin — the `charly cardwire …` CLI
// surface (status / list / config / gpu / manager), parsed with kong and dispatched by
// cliMain in CLI mode (sdk.Main → cliMain). Exit-code contracts: status/list/config
// always exit 0 (a missing cardwire install prints explicit N/A lines — the graceful,
// host-independent path the check-cardwire-local bed asserts); gpu block|unblock is the
// MUTATING surface and exits 1 with prose when cardwire is unavailable or the
// operation/verification fails; manager status passes through.

// CardwireCmd is the kong command tree.
type CardwireCmd struct {
	Status  StatusCmd  `cmd:"" help:"cardwire host report (cardwired state, config keys, BPF-LSM gate)"`
	List    ListCmd    `cmd:"" help:"List GPUs known to cardwired (normalized from cardwire list --json)"`
	Config  ConfigCmd  `cmd:"" help:"Read/write the /etc/cardwire/cardwire.toml config"`
	Gpu     GpuCmd     `cmd:"" help:"Block/unblock a GPU by id (the enable/disable surface)"`
	Manager ManagerCmd `cmd:"" help:"cardwire manager operations"`
}

// StatusCmd prints the deterministic host report (human or --json). Reads cardwired
// systemd state, the /etc/cardwire/cardwire.toml config keys and the BPF-LSM facts via
// /sys/kernel/security/lsm DIRECTLY — every unreadable fact prints an explicit N/A
// line, exit 0 ALWAYS.
type StatusCmd struct {
	JSON bool `long:"json" help:"Emit a stable JSON report"`
}

func (c StatusCmd) Run() error {
	fmt.Print(renderStatusText(collectStatus(), c.JSON))
	return nil
}

// ListCmd lists the GPUs cardwired manages. --json emits the normalized stable report.
// A MISSING cardwire install is graceful: explicit N/A lines, exit 0. A REAL failure
// of an installed cardwire (daemon down, parse failure) exits 1 with prose.
type ListCmd struct {
	JSON bool `long:"json" help:"Emit the normalized GPU list as stable JSON"`
}

func (c ListCmd) Run() error {
	text, err := listReport(c.JSON)
	fmt.Print(text)
	return err
}

// ConfigCmd reads/writes /etc/cardwire/cardwire.toml (atomic tmp+rename, mirroring
// cardwire's own approach). Keys are whitelisted: auto_apply_gpu_state,
// experimental_nvidia_block, battery_auto_switch, battery_auto_switch_mode,
// external_display_auto_switch. Missing root/file → graceful N/A, exit 0.
type ConfigCmd struct {
	Get ConfigGetCmd `cmd:"" help:"Print the current value of a config key"`
	Set ConfigSetCmd `cmd:"" help:"Set a config key (no value prints the current value)"`
}

type ConfigGetCmd struct {
	Key string `arg:"" required:"" help:"config key (whitelisted)"`
}

func (c ConfigGetCmd) Run() error {
	text, err := configGet(c.Key)
	fmt.Print(text)
	return err
}

type ConfigSetCmd struct {
	Key   string  `arg:"" required:"" help:"config key (whitelisted)"`
	Value *string `arg:"" optional:"" help:"value (bool or a mode name); omitted prints the current value"`
}

func (c ConfigSetCmd) Run() error {
	text, err := configSet(c.Key, c.Value)
	fmt.Print(text)
	return err
}

// GpuCmd is the enable/disable surface: block/unblock a GPU by id via
// /usr/bin/cardwire, then VERIFY via cardwire list --json that the blocked flag
// flipped. exit 0 on success; 1 with prose on failure (cardwire not installed →
// "cardwire UNAVAILABLE" — this IS the mutating surface, refusal is correct).
type GpuCmd struct {
	ID  uint32 `arg:"" required:"" help:"GPU id to operate on"`
	Act string `arg:"" required:"" enum:"block,unblock" help:"block | unblock"`
}

func (c GpuCmd) Run() error {
	text, err := gpuToggle(c.ID, c.Act)
	fmt.Print(text)
	return err
}

// ManagerCmd passes through cardwire manager operations (status).
type ManagerCmd struct {
	Status ManagerStatusCmd `cmd:"" help:"cardwire manager status (passthrough)"`
}

type ManagerStatusCmd struct{}

func (c ManagerStatusCmd) Run() error {
	text, err := managerStatus()
	fmt.Print(text)
	return err
}

// cliMain is the CLI-mode entry point (sdk.Main calls it when charly fork/exec'd this
// plugin as a command passthrough). It parses the pass-through tokens against
// CardwireCmd and dispatches via kong Run. Returns the process exit code.
func cliMain(args []string) int {
	var grp CardwireCmd
	parser, err := kong.New(&grp,
		kong.Name("cardwire"),
		kong.Description("charly cardwire — the cardwire GPU-manager surface (status / list / config / gpu block|unblock / manager)"),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		parser.FatalIfErrorf(err)
		return 1
	}
	if err := ctx.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
