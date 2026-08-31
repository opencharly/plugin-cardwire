package cardwire

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// status.go owns the deterministic host-report collectors + renderer for
// `cardwire status`. Every fact has an explicit N/A path — an unreadable file or
// unavailable systemctl is a reported N/A line, never a hard failure. The BPF-LSM
// facts come from /sys/kernel/security/lsm DIRECTLY (the same canonical read
// plugin-bpf's status performs — R3: one canonical surface, no ad-hoc copies, and no
// shelling out to charly bpf; the plugin stays self-contained).

// Overridable paths (hermetic tests point these at temp fixtures / PATH fakes).
var (
	cardwireBin  = "/usr/bin/cardwire" // cardwireBin overridable: a bare name resolves via PATH (tests inject a fake)
	systemctlBin = "systemctl"
	tomlPath     = "/etc/cardwire/cardwire.toml"
	lsmPath      = "/sys/kernel/security/lsm"
)

// ConfigKeys is the whitelisted config surface (the keys cardwire's daemon persists
// in /etc/cardwire/cardwire.toml).
var ConfigKeys = []string{
	"auto_apply_gpu_state",
	"experimental_nvidia_block",
	"battery_auto_switch",
	"battery_auto_switch_mode",
	"external_display_auto_switch",
}

// Status is the collected host report (stable JSON keys via tags).
type Status struct {
	Cardwired     string            `json:"cardwired"`      // active | inactive | N/A
	ConfigPresent bool              `json:"config_present"` // /etc/cardwire/cardwire.toml exists
	ConfigKeys    map[string]string `json:"config_keys"`    // the whitelisted keys → value strings
	LSMList       []string          `json:"lsm_list"`       // parsed /sys/kernel/security/lsm
	LSMHasBPF     bool              `json:"lsm_bpf"`        // bpf present in the active LSM list
}

// collectStatus gathers the report: systemd state of cardwired, the config file keys,
// and the BPF-LSM facts. Every read is defensive (missing file → zero value + N/A).
func collectStatus() Status {
	s := Status{ConfigKeys: map[string]string{}}

	// (a) cardwired systemd state — systemctl is-active prints the state and exits
	// nonzero for non-active; we take the printed state regardless of exit code.
	if out, err := exec.Command(systemctlBin, "is-active", "cardwired").Output(); err == nil || len(out) > 0 {
		st := strings.TrimSpace(string(out))
		switch st {
		case "active", "inactive", "failed", "activating", "deactivating":
			s.Cardwired = st
		default:
			s.Cardwired = "N/A"
		}
	} else {
		s.Cardwired = "N/A"
	}

	// (b) config file + whitelisted keys.
	content, err := os.ReadFile(tomlPath)
	if err != nil {
		s.ConfigPresent = false
	} else {
		s.ConfigPresent = true
		for _, k := range ConfigKeys {
			s.ConfigKeys[k] = tomlKeyString(content, k)
		}
	}

	// (c) BPF-LSM facts — the DIRECT canonical read (same as plugin-bpf's status).
	if content, err := os.ReadFile(lsmPath); err == nil {
		s.LSMList = parseLsmList(string(content))
		s.LSMHasBPF = hasBPF(s.LSMList)
	}

	return s
}

// tomlKeyString returns the string rendering of a key from a toml document, or "N/A".
func tomlKeyString(content []byte, key string) string {
	var doc map[string]any
	if err := toml.Unmarshal(content, &doc); err != nil {
		return "N/A"
	}
	v, ok := doc[key]
	if !ok {
		return "N/A"
	}
	switch t := v.(type) {
	case bool:
		return fmt.Sprintf("%v", t)
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// parseLsmList splits the raw /sys/kernel/security/lsm content into the LSM names.
func parseLsmList(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(content, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// hasBPF reports whether bpf is in an LSM list.
func hasBPF(list []string) bool {
	for _, l := range list {
		if l == "bpf" {
			return true
		}
	}
	return false
}

// renderStatusText renders the host report (human or --json).
func renderStatusText(s Status, jsonOut bool) string {
	if jsonOut {
		b, _ := json.MarshalIndent(s, "", "  ")
		return string(b) + "\n"
	}
	var b strings.Builder
	b.WriteString("cardwire status report\n")
	b.WriteString(fmt.Sprintf("  cardwired: %s\n", s.Cardwired))
	if s.ConfigPresent {
		b.WriteString(fmt.Sprintf("  config: %s (present)\n", tomlPath))
		for _, k := range ConfigKeys {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, s.ConfigKeys[k]))
		}
	} else {
		b.WriteString(fmt.Sprintf("  config: %s (absent)\n", tomlPath))
	}
	if len(s.LSMList) > 0 {
		b.WriteString(fmt.Sprintf("  lsm: %s\n", strings.Join(s.LSMList, ",")))
	} else {
		b.WriteString("  lsm: N/A\n")
	}
	if s.LSMHasBPF {
		b.WriteString("  lsm_bpf: yes\n")
	} else if len(s.LSMList) > 0 {
		b.WriteString("  lsm_bpf: no\n")
	} else {
		b.WriteString("  lsm_bpf: N/A\n")
	}
	return b.String()
}
