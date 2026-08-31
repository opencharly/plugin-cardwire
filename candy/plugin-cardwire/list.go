package cardwire

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// list.go owns the `cardwire list` surface: fork/exec /usr/bin/cardwire list --json,
// parse the real cardwire JSON shape (a map keyed "0".."N" whose values carry the
// stable fields below — the shape captured in the Phase-0 spike), and normalize it
// into a stable, id-sorted report. A MISSING cardwire install is graceful: explicit
// N/A lines, exit 0 (host-independent bed). A REAL failure of an installed cardwire
// (daemon down, non-JSON output, nonzero exit) exits 1 with prose.

// CardwireGpu mirrors the CLI-side GpuDevice struct cardwire serializes (its
// display.rs / get_gpu_list): the canonical stable keys.
type CardwireGpu struct {
	ID          uint32 `json:"id"`
	Name        string `json:"name"`
	PCI         string `json:"pci"`
	Render      uint32 `json:"render"`
	Card        uint32 `json:"card"`
	Default     bool   `json:"default"`
	Discrete    bool   `json:"discrete"`
	VirtualGPU  bool   `json:"virtual_gpu"`
	Available   bool   `json:"available"`
	Vendor      string `json:"vendor"`
	Driver      string `json:"driver"`
	Blocked     bool   `json:"blocked"`
	Launchable  bool   `json:"launchable"`
	Nvidia      bool   `json:"nvidia"`
	NvidiaMinor string `json:"nvidia_minor"`
}

// ListReport is the normalized report (stable JSON keys).
type ListReport struct {
	Cardwire string        `json:"cardwire"` // "present" | "unavailable"
	Count    int           `json:"count"`
	GPUs     []CardwireGpu `json:"gpus"`
}

// runCardwire executes the cardwire binary with args, returning stdout. An
// exec.ErrNotFound (binary absent) is returned as-is so callers can branch on the
// graceful N/A path; any other failure carries the captured stderr.
func runCardwire(args ...string) (string, error) {
	cmd := exec.Command(cardwireBin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			// Both the bare-name LookPath miss (exec.ErrNotFound) and an absolute-path
			// miss (PathError wrapping os.ErrNotExist) mean "cardwire not installed":
			// normalize to the sentinel so every caller branches on the graceful N/A
			// path uniformly.
			return "", exec.ErrNotFound
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("cardwire %s failed: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

// parseCardwireList parses the REAL cardwire list --json output: a JSON object keyed
// "0".."N" with the CardwireGpu fields.
func parseCardwireList(raw string) ([]CardwireGpu, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, fmt.Errorf("cardwire list --json: not a cardwire JSON report: %v", err)
	}
	// Numeric-keyed object (BTreeMap<usize, GpuDevice>): tolerate a top-level wrapper
	// key (some cardwire builds nest under "gpus") by unwrapping defensively.
	if gpus, ok := obj["gpus"]; ok && len(obj) == 1 {
		if err := json.Unmarshal(gpus, &obj); err != nil {
			return nil, fmt.Errorf("cardwire list --json: bad gpus wrapper: %v", err)
		}
	}
	var out []CardwireGpu
	for k, v := range obj {
		id, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			// Not a "0".."N" key — the output is not the cardwire GPU map.
			return nil, fmt.Errorf("cardwire list --json: unexpected key %q (not a cardwire GPU map)", k)
		}
		var g CardwireGpu
		if err := json.Unmarshal(v, &g); err != nil {
			return nil, fmt.Errorf("cardwire list --json: bad GPU entry %q: %v", k, err)
		}
		// The map key is authoritative for the id (the embedded id field is
		// cardwire's own echo; keep it consistent with the key).
		g.ID = uint32(id)
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// listReport produces the list output: exec cardwire list --json; parse + normalize.
// Missing cardwire → graceful N/A, exit 0. Real failure → error (exit 1).
func listReport(jsonOut bool) (string, error) {
	raw, err := runCardwire("list", "--json")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return renderListUnavailable(jsonOut), nil
		}
		return renderListUnavailable(jsonOut) + "\n" + err.Error() + "\n", err
	}
	gpus, err := parseCardwireList(raw)
	if err != nil {
		return "cardwire list: FAILED to parse the cardwire report\n" + err.Error() + "\n", err
	}
	return renderListReport(ListReport{Cardwire: "present", Count: len(gpus), GPUs: gpus}, jsonOut), nil
}

// renderListUnavailable is the graceful N/A report (absent cardwire) — exit 0.
func renderListUnavailable(jsonOut bool) string {
	if jsonOut {
		r := ListReport{Cardwire: "unavailable", Count: 0, GPUs: []CardwireGpu{}}
		b, _ := json.MarshalIndent(r, "", "  ")
		return string(b) + "\n"
	}
	return "cardwire list: CARDWIRE UNAVAILABLE (daemon not installed/running)\n" +
		"  GPUs: N/A\n"
}

// renderListReport renders the normalized report (human table or --json).
func renderListReport(r ListReport, jsonOut bool) string {
	if jsonOut {
		b, _ := json.MarshalIndent(r, "", "  ")
		return string(b) + "\n"
	}
	if r.Count == 0 {
		return "cardwire list: no GPUs managed by cardwired\n"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-2s  %-30s  %-12s  %-10s  %-6s  %-8s  %-8s  %s\n",
		"ID", "NAME", "PCI", "RENDER", "CARD", "DEFAULT", "DISCRETE", "BLOCKED"))
	b.WriteString(fmt.Sprintf("%-2s  %-30s  %-12s  %-10s  %-6s  %-8s  %-8s  %s\n",
		"--", strings.Repeat("-", 30), strings.Repeat("-", 12), strings.Repeat("-", 10),
		strings.Repeat("-", 6), strings.Repeat("-", 8), strings.Repeat("-", 8), strings.Repeat("-", 7)))
	for _, g := range r.GPUs {
		def, dis := "( )", "( )"
		if g.Default {
			def = "(*)"
		}
		if g.Discrete {
			dis = "(*)"
		}
		b.WriteString(fmt.Sprintf("%-2d  %-30s  %-12s  renderD%-5d  card%-3d  %-8s  %-8s  %v\n",
			g.ID, g.Name, g.PCI, g.Render, g.Card, def, dis, g.Blocked))
	}
	return b.String()
}
