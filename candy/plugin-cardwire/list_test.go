package cardwire

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Hermetic list tests: the fixture is the REAL cardwire list --json shape captured in
// the Phase-0 spike (two GPUs — Virtio 1.0 GPU id 0 + NVIDIA GeForce RTX 4080 SUPER
// id 1 — exactly as the spike's GPU table records, with the remaining fields filled
// per cardwire's own CLI-side serialization contract in display.rs). The unavailable
// path is exercised with cardwireBin = "cardwire" and NO fake on PATH (graceful N/A,
// exit 0); the real-failure path with a fake that exits nonzero.

// spikeListJSON is the reconstructed cardwire list --json output for the Phase-0
// spike host: both GPUs unblocked (the pre-T2 state).
const spikeListJSON = `{
  "0": {
    "id": 0,
    "name": "Virtio 1.0 GPU",
    "pci": "0000:00:01.0",
    "render": 128,
    "card": 1,
    "default": true,
    "discrete": false,
    "virtual_gpu": true,
    "available": true,
    "vendor": "Unknown Vendor",
    "driver": "virtio_gpu",
    "blocked": false,
    "launchable": true,
    "nvidia": false,
    "nvidia_minor": "none"
  },
  "1": {
    "id": 1,
    "name": "NVIDIA GeForce RTX 4080 SUPER",
    "pci": "0000:05:00.0",
    "render": 129,
    "card": 0,
    "default": false,
    "discrete": false,
    "virtual_gpu": false,
    "available": true,
    "vendor": "Nvidia",
    "driver": "nvidia",
    "blocked": false,
    "launchable": true,
    "nvidia": true,
    "nvidia_minor": "0"
  }
}
`

func TestParseCardwireList_SpikeFixture(t *testing.T) {
	gpus, err := parseCardwireList(spikeListJSON)
	if err != nil {
		t.Fatalf("spike fixture must parse: %v", err)
	}
	if len(gpus) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(gpus))
	}
	g0, g1 := gpus[0], gpus[1]
	if g0.ID != 0 || g0.Name != "Virtio 1.0 GPU" || g0.PCI != "0000:00:01.0" {
		t.Fatalf("GPU 0 normalized wrong: %+v", g0)
	}
	if g0.Render != 128 || g0.Card != 1 || !g0.Default || g0.Discrete || !g0.VirtualGPU {
		t.Fatalf("GPU 0 flags wrong: %+v", g0)
	}
	if g0.Blocked || !g0.Launchable || g0.Nvidia {
		t.Fatalf("GPU 0 block/launch/nvidia wrong: %+v", g0)
	}
	if g1.ID != 1 || g1.Name != "NVIDIA GeForce RTX 4080 SUPER" || g1.PCI != "0000:05:00.0" {
		t.Fatalf("GPU 1 normalized wrong: %+v", g1)
	}
	if g1.Render != 129 || g1.Card != 0 || g1.Default || g1.Discrete || g1.VirtualGPU {
		t.Fatalf("GPU 1 flags wrong: %+v", g1)
	}
	if !g1.Nvidia || g1.NvidiaMinor != "0" || g1.Vendor != "Nvidia" || g1.Driver != "nvidia" {
		t.Fatalf("GPU 1 nvidia facts wrong: %+v", g1)
	}
	// Keys are authoritative for the id: ids must come back sorted.
	if gpus[0].ID >= gpus[1].ID {
		t.Fatalf("gpus must be id-sorted: %+v", gpus)
	}
}

func TestParseCardwireList_BlockedFlip(t *testing.T) {
	flipped := strings.Replace(spikeListJSON, "\"blocked\": false",
		"\"blocked\": true", 1) // flips GPU 0's blocked field only
	gpus, err := parseCardwireList(flipped)
	if err != nil {
		t.Fatalf("flipped fixture must parse: %v", err)
	}
	if gpus[0].Blocked != true {
		t.Fatalf("GPU 0 must be blocked: %+v", gpus[0])
	}
	if gpus[1].Blocked {
		t.Fatalf("GPU 1 must stay unblocked: %+v", gpus[1])
	}
}

func TestParseCardwireList_NotACardwireReport(t *testing.T) {
	if _, err := parseCardwireList("{\"foo\": \"bar\"}"); err == nil {
		t.Fatal("a non-GPU-map JSON object must be rejected")
	}
	if _, err := parseCardwireList("not json"); err == nil {
		t.Fatal("non-JSON must be rejected")
	}
}

func TestListReport_AbsentCardwire_GracefulExit0(t *testing.T) {
	// cardwireBin = "cardwire" (bare) with NO fake on PATH → exec.ErrNotFound →
	// graceful N/A report, exit 0.
	old := cardwireBin
	cardwireBin = "cardwire"
	defer func() { cardwireBin = old }()
	text, err := listReport(false)
	if err != nil {
		t.Fatalf("absent cardwire must be graceful (exit 0): %v", err)
	}
	if !strings.Contains(text, "CARDWIRE UNAVAILABLE") || !strings.Contains(text, "N/A") {
		t.Fatalf("unavailable report malformed: %s", text)
	}
	// --json variant: stable report keys, cardwire=unavailable.
	jtext, err := listReport(true)
	if err != nil {
		t.Fatalf("absent cardwire --json must be graceful: %v", err)
	}
	if !strings.Contains(jtext, "\"cardwire\": \"unavailable\"") || !strings.Contains(jtext, "\"count\": 0") {
		t.Fatalf("unavailable --json report malformed: %s", jtext)
	}
}

func TestListReport_RealFailure_Exit1(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "cardwire")
	// A fake that FAILS (daemon-down simulation: nonzero exit + prose on stderr).
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'error: cardwired daemon is not running. Is the service up?' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldBin, oldPath := cardwireBin, os.Getenv("PATH")
	cardwireBin = "cardwire"
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	defer func() { cardwireBin = oldBin }()

	text, err := listReport(false)
	if err == nil {
		t.Fatalf("an installed-but-failing cardwire must exit 1; got text: %s", text)
	}
	if !strings.Contains(err.Error(), "daemon is not running") {
		t.Fatalf("failure prose must carry the cardwire stderr: %v", err)
	}
}

func TestRunCardwire_ArgvLookup(t *testing.T) {
	// The exec helper must resolve a bare name through PATH (the fake-injection seam).
	dir := t.TempDir()
	fake := filepath.Join(dir, "cardwire")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"args:$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldBin, oldPath := cardwireBin, os.Getenv("PATH")
	cardwireBin = "cardwire"
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	defer func() { cardwireBin = oldBin }()

	out, err := runCardwire("list", "--json")
	if err != nil {
		t.Fatalf("fake cardwire must run: %v", err)
	}
	if !strings.Contains(out, "args:list --json") {
		t.Fatalf("argv not passed through: %q", out)
	}
	// Bare name with nothing on PATH → ErrNotFound.
	cardwireBin = "definitely-not-a-real-cardwire-binary"
	if _, err := runCardwire("list"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("expected exec.ErrNotFound, got %v", err)
	}
}

func TestListReport_AbsentCardwire_AbsolutePath_GracefulExit0(t *testing.T) {
	// The R10/bed path: cardwireBin stays the ABSOLUTE /usr/bin/cardwire (no PATH
	// fake). A missing binary at an absolute path surfaces as a PathError wrapping
	// os.ErrNotExist (NOT exec.ErrNotFound — LookPath is skipped for absolute names);
	// runCardwire must normalize it so the graceful N/A path still exits 0.
	old := cardwireBin
	cardwireBin = "/usr/bin/definitely-not-a-real-cardwire-binary"
	defer func() { cardwireBin = old }()

	text, err := listReport(false)
	if err != nil {
		t.Fatalf("absent cardwire at an absolute path must be graceful (exit 0): %v", err)
	}
	if !strings.Contains(text, "CARDWIRE UNAVAILABLE") {
		t.Fatalf("unavailable report malformed: %s", text)
	}
	// status/config share the same absent-path contract through the CLI Run()s.
	var sc StatusCmd
	if err := sc.Run(); err != nil {
		t.Fatalf("status must exit 0 with an absent cardwire: %v", err)
	}
	var cc ConfigGetCmd
	cc.Key = "auto_apply_gpu_state"
	if err := cc.Run(); err != nil {
		t.Fatalf("config get must exit 0 with an absent cardwire: %v", err)
	}
}
