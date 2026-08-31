package cardwire

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Hermetic gpu tests: block/unblock argv + verify logic against a FAKE /usr/bin/cardwire
// injected on PATH (records argv, maintains a blocked-state file, emits the spike list
// JSON with the live blocked flags). No real cardwire is executed anywhere.

// fakeCardwireScript is a stateful fake: `gpu <id> --block|--unblock` flips the
// blocked flag in $CARDWIRE_STATE_DIR/blocked-<id>, `list --json` emits the spike
// two-GPU JSON with the live flags, and every argv is appended to $CARDWIRE_ARGV_LOG.
const fakeCardwireScript = `#!/bin/sh
LOG="${CARDWIRE_ARGV_LOG:-/dev/null}"
STATE="${CARDWIRE_STATE_DIR:-/tmp}"
echo "cardwire $*" >> "$LOG"
cmd="$1"
case "$cmd" in
  gpu)
    id="$2"; flag="$3"
    if [ "$flag" = "--block" ]; then v=true; else v=false; fi
    echo "$v" > "$STATE/blocked-$id"
    if [ "$flag" = "--block" ]; then echo "GPU $id has been blocked"; else echo "GPU $id has been unblocked"; fi
    exit 0
    ;;
  manager)
    echo "Daemon is alive"
    exit 0
    ;;
  list)
    b0=$(cat "$STATE/blocked-0" 2>/dev/null || echo false)
    b1=$(cat "$STATE/blocked-1" 2>/dev/null || echo false)
    cat <<EOF
{
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
    "blocked": $b0,
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
    "blocked": $b1,
    "launchable": true,
    "nvidia": true,
    "nvidia_minor": "0"
  }
}
EOF
    exit 0
    ;;
  *)
    echo "error: unknown cardwire command: $*" >&2
    exit 1
    ;;
esac
`

// noopFakeScript is a fake whose gpu command reports success but does NOT flip the
// daemon state (a silent refusal) — the verify leg must catch the mismatch.
const noopFakeScript = `#!/bin/sh
echo "cardwire $*" >> "${CARDWIRE_ARGV_LOG:-/dev/null}"
case "$1" in
  gpu) echo "GPU $2 has been blocked"; exit 0 ;;
  list) cat "${CARDWIRE_LIST_OUT}" ;;
  *) exit 1 ;;
esac
`

// installFakeCardwire writes the fake script into a fresh temp dir and points
// cardwireBin at the bare name with that dir on PATH; returns the state dir + argv log.
func installFakeCardwire(t *testing.T) (binDir, stateDir, argvLog string) {
	t.Helper()
	binDir = t.TempDir()
	fake := filepath.Join(binDir, "cardwire")
	if err := os.WriteFile(fake, []byte(fakeCardwireScript), 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir = t.TempDir()
	argvLog = filepath.Join(t.TempDir(), "argv.log")

	oldBin, oldPath := cardwireBin, os.Getenv("PATH")
	cardwireBin = "cardwire"
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	t.Setenv("CARDWIRE_STATE_DIR", stateDir)
	t.Setenv("CARDWIRE_ARGV_LOG", argvLog)
	t.Cleanup(func() { cardwireBin = oldBin })
	return binDir, stateDir, argvLog
}

func TestGpuToggle_Block_ArgvAndVerify(t *testing.T) {
	_, stateDir, argvLog := installFakeCardwire(t)

	text, err := gpuToggle(1, "block")
	if err != nil {
		t.Fatalf("block must succeed: %v", err)
	}
	if !strings.Contains(text, "GPU 1 BLOCKED (verified: blocked=true)") {
		t.Fatalf("block marker+verify wrong: %s", text)
	}
	// The argv must reach the real cardwire surface: gpu <id> --block, then the verify list.
	log, _ := os.ReadFile(argvLog)
	argv := string(log)
	if !strings.Contains(argv, "cardwire gpu 1 --block") {
		t.Fatalf("block argv wrong: %s", argv)
	}
	if !strings.Contains(argv, "cardwire list --json") {
		t.Fatalf("verify argv missing: %s", argv)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "blocked-1")); err != nil {
		t.Fatalf("fake state not flipped: %v", err)
	}
}

func TestGpuToggle_Unblock_ArgvAndVerify(t *testing.T) {
	_, _, argvLog := installFakeCardwire(t)
	// Seed a blocked state via the fake itself, then unblock.
	if _, err := gpuToggle(1, "block"); err != nil {
		t.Fatalf("seed block failed: %v", err)
	}
	text, err := gpuToggle(1, "unblock")
	if err != nil {
		t.Fatalf("unblock must succeed: %v", err)
	}
	if !strings.Contains(text, "GPU 1 UNBLOCKED (verified: blocked=false)") {
		t.Fatalf("unblock marker+verify wrong: %s", text)
	}
	log, _ := os.ReadFile(argvLog)
	if !strings.Contains(string(log), "cardwire gpu 1 --unblock") {
		t.Fatalf("unblock argv wrong: %s", log)
	}
}

func TestGpuToggle_NotInstalled_Refusal(t *testing.T) {
	// No fake on PATH, bare name → exec.ErrNotFound → "cardwire UNAVAILABLE", exit 1
	// (this IS the mutating surface — refusal is correct).
	oldBin := cardwireBin
	cardwireBin = "cardwire"
	t.Setenv("PATH", t.TempDir())
	t.Cleanup(func() { cardwireBin = oldBin })

	text, err := gpuToggle(1, "block")
	if err == nil {
		t.Fatalf("block with no cardwire must exit 1; got: %s", text)
	}
	if !strings.Contains(err.Error(), "cardwire UNAVAILABLE") {
		t.Fatalf("refusal prose must name the unavailability: %v", err)
	}
}

func TestGpuToggle_VerifyFail(t *testing.T) {
	// The noop fake's gpu command does NOT flip the state (a daemon refusal not
	// surfaced by the CLI exit code) → the verify leg must catch it and exit 1.
	dir := t.TempDir()
	fake := filepath.Join(dir, "cardwire")
	if err := os.WriteFile(fake, []byte(noopFakeScript), 0o755); err != nil {
		t.Fatal(err)
	}
	// The list output always reports blocked=false — the block did NOT stick.
	listOut := filepath.Join(t.TempDir(), "list.json")
	if err := os.WriteFile(listOut, []byte(`{"0": {"id": 0, "name": "Virtio 1.0 GPU", "pci": "0000:00:01.0", "render": 128, "card": 1, "default": true, "discrete": false, "virtual_gpu": true, "available": true, "vendor": "Unknown Vendor", "driver": "virtio_gpu", "blocked": false, "launchable": true, "nvidia": false, "nvidia_minor": "none"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	oldBin, oldPath := cardwireBin, os.Getenv("PATH")
	cardwireBin = "cardwire"
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	t.Setenv("CARDWIRE_LIST_OUT", listOut)
	t.Cleanup(func() { cardwireBin = oldBin })

	text, err := gpuToggle(0, "block")
	if err == nil {
		t.Fatalf("a block that did not stick must exit 1; got: %s", text)
	}
	if !strings.Contains(err.Error(), "VERIFY FAILED") {
		t.Fatalf("verify failure prose missing: %v", err)
	}
}

func TestManagerStatus_Passthrough(t *testing.T) {
	_, _, argvLog := installFakeCardwire(t)
	text, err := managerStatus()
	if err != nil {
		t.Fatalf("manager status must exit 0: %v", err)
	}
	if !strings.Contains(text, "Daemon is alive") {
		t.Fatalf("passthrough output wrong: %s", text)
	}
	log, _ := os.ReadFile(argvLog)
	if !strings.Contains(string(log), "cardwire manager status") {
		t.Fatalf("manager argv wrong: %s", log)
	}
}

func TestManagerStatus_Absent_Graceful(t *testing.T) {
	oldBin := cardwireBin
	cardwireBin = "cardwire"
	t.Setenv("PATH", t.TempDir())
	t.Cleanup(func() { cardwireBin = oldBin })
	text, err := managerStatus()
	if err != nil {
		t.Fatalf("absent cardwire manager status must exit 0: %v", err)
	}
	if !strings.Contains(text, "UNAVAILABLE") {
		t.Fatalf("unavailable report wrong: %s", text)
	}
}
