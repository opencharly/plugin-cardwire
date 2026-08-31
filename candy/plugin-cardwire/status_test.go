package cardwire

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Hermetic status tests: pure collectors/renderers on fixtures (real host + guest LSM
// lists from the Phase-0 spike) + a PATH-injected fake systemctl; the graceful N/A
// path is exercised with no readable config/lsm and no systemctl at all.

func TestCollectStatus_FakeSystemctlAndFixtures(t *testing.T) {
	dir := t.TempDir()
	// Fake systemctl on PATH: cardwired is-active → active.
	fakeSys := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(fakeSys, []byte("#!/bin/sh\nif [ \"$1\" = \"is-active\" ]; then echo active; exit 0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Config file with the install-plan defaults.
	toml := filepath.Join(dir, "cardwire.toml")
	if err := os.WriteFile(toml, []byte(
		"auto_apply_gpu_state = true\nexperimental_nvidia_block = true\n"+
			"battery_auto_switch = false\nbattery_auto_switch_mode = \"hybrid\"\n"+
			"external_display_auto_switch = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// LSM fixture: the reference host's active list (spike: capability,landlock,lockdown,yama,bpf).
	lsm := filepath.Join(dir, "lsm")
	if err := os.WriteFile(lsm, []byte("capability,landlock,lockdown,yama,bpf"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldBin, oldSys, oldToml, oldLsm, oldPath := cardwireBin, systemctlBin, tomlPath, lsmPath, os.Getenv("PATH")
	cardwireBin, systemctlBin = "cardwire", "systemctl"
	tomlPath, lsmPath = toml, lsm
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	defer func() {
		cardwireBin, systemctlBin, tomlPath, lsmPath = oldBin, oldSys, oldToml, oldLsm
	}()

	s := collectStatus()
	if s.Cardwired != "active" {
		t.Fatalf("cardwired must be active, got %q", s.Cardwired)
	}
	if !s.ConfigPresent {
		t.Fatal("config must be present")
	}
	if s.ConfigKeys["experimental_nvidia_block"] != "true" ||
		s.ConfigKeys["battery_auto_switch_mode"] != "hybrid" {
		t.Fatalf("config keys wrong: %+v", s.ConfigKeys)
	}
	if !s.LSMHasBPF || len(s.LSMList) != 5 || s.LSMList[4] != "bpf" {
		t.Fatalf("lsm facts wrong: %+v", s.LSMList)
	}
}

func TestCollectStatus_GracefulNA(t *testing.T) {
	// No systemctl fake, no config, no lsm file — every fact must degrade to N/A
	// without erroring (the R10 host without cardwire is the same path).
	dir := t.TempDir()
	oldBin, oldSys, oldToml, oldLsm := cardwireBin, systemctlBin, tomlPath, lsmPath
	cardwireBin, systemctlBin = "cardwire", "systemctl-definitely-absent"
	tomlPath = filepath.Join(dir, "no", "cardwire.toml")
	lsmPath = filepath.Join(dir, "no", "lsm")
	t.Setenv("PATH", dir)
	defer func() {
		cardwireBin, systemctlBin, tomlPath, lsmPath = oldBin, oldSys, oldToml, oldLsm
	}()

	// The full CLI surface must still exit 0 (graceful N/A lines).
	text := renderStatusText(collectStatus(), false)
	if !strings.Contains(text, "cardwire status report") {
		t.Fatalf("report header missing: %s", text)
	}
	if !strings.Contains(text, "N/A") {
		t.Fatalf("N/A lines required: %s", text)
	}
	// The Run() contract: exit 0.
	var c StatusCmd
	if err := c.Run(); err != nil {
		t.Fatalf("status must exit 0 on a GPU-less host: %v", err)
	}
}

func TestRenderStatusText_JSONStableKeys(t *testing.T) {
	s := Status{
		Cardwired:     "inactive",
		ConfigPresent: true,
		ConfigKeys:    map[string]string{"auto_apply_gpu_state": "true"},
		LSMList:       []string{"capability", "landlock", "lockdown", "yama", "bpf"},
		LSMHasBPF:     true,
	}
	out := renderStatusText(s, true)
	for _, k := range []string{"cardwired", "config_present", "config_keys", "lsm_list", "lsm_bpf"} {
		if !strings.Contains(out, "\""+k+"\"") {
			t.Fatalf("status --json missing key %q: %s", k, out)
		}
	}
	if !strings.Contains(out, "\"lsm_bpf\": true") {
		t.Fatalf("expected lsm_bpf true: %s", out)
	}
}

func TestParseLsmList(t *testing.T) {
	got := parseLsmList("capability,landlock,lockdown,yama,bpf")
	if !hasBPF(got) || len(got) != 5 {
		t.Fatalf("bpf active parse failed: %v", got)
	}
	got2 := parseLsmList("capability,landlock,lockdown,yama")
	if hasBPF(got2) {
		t.Fatalf("bpf must NOT be reported when absent: %v", got2)
	}
	if got3 := parseLsmList(""); got3 != nil {
		t.Fatalf("empty lsm must yield nil, got %v", got3)
	}
}
