package cardwire

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Hermetic config tests: TOML read/set roundtrips against t.TempDir files
// (tomlPath override); graceful N/A paths (missing root/file → exit 0).

func TestConfigGetSet_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	toml := filepath.Join(dir, "cardwire.toml")
	old := tomlPath
	tomlPath = toml
	defer func() { tomlPath = old }()

	// Start from the install-plan default file.
	if err := os.WriteFile(toml, []byte(
		"auto_apply_gpu_state = true\nexperimental_nvidia_block = true\n"+
			"battery_auto_switch = false\nbattery_auto_switch_mode = \"hybrid\"\n"+
			"external_display_auto_switch = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	text, err := configGet("auto_apply_gpu_state")
	if err != nil {
		t.Fatalf("get must exit 0: %v", err)
	}
	if !strings.Contains(text, "auto_apply_gpu_state: true") {
		t.Fatalf("get output wrong: %s", text)
	}

	// Set a bool key.
	text, err = configSet("experimental_nvidia_block", strptr("false"))
	if err != nil {
		t.Fatalf("set must exit 0: %v", err)
	}
	if !strings.Contains(text, "experimental_nvidia_block: false (written)") {
		t.Fatalf("set output wrong: %s", text)
	}
	// Roundtrip: the file must now carry false.
	content, _ := os.ReadFile(toml)
	if !strings.Contains(string(content), "experimental_nvidia_block = false") {
		t.Fatalf("toml not persisted: %s", content)
	}
	// The OTHER keys must survive the atomic rewrite (load-merge, not clobber).
	if !strings.Contains(string(content), "auto_apply_gpu_state = true") ||
		!strings.Contains(string(content), "hybrid") {
		t.Fatalf("atomic write clobbered sibling keys: %s", content)
	}

	// Set the mode key (string-typed).
	text, err = configSet("battery_auto_switch_mode", strptr("manual"))
	if err != nil {
		t.Fatalf("mode set must exit 0: %v", err)
	}
	content, _ = os.ReadFile(toml)
	if !strings.Contains(string(content), "manual") {
		t.Fatalf("mode not persisted: %s", content)
	}
}

func TestConfigGet_MissingFile_GracefulNA(t *testing.T) {
	dir := t.TempDir()
	toml := filepath.Join(dir, "nope", "cardwire.toml") // dir does not exist either
	old := tomlPath
	tomlPath = toml
	defer func() { tomlPath = old }()

	text, err := configGet("auto_apply_gpu_state")
	if err != nil {
		t.Fatalf("missing config file must exit 0: %v", err)
	}
	if !strings.Contains(text, "N/A") {
		t.Fatalf("missing file must report N/A: %s", text)
	}
}

func TestConfigSet_WithoutValue_PrintsCurrent(t *testing.T) {
	dir := t.TempDir()
	toml := filepath.Join(dir, "cardwire.toml")
	old := tomlPath
	tomlPath = toml
	defer func() { tomlPath = old }()
	if err := os.WriteFile(toml, []byte("auto_apply_gpu_state = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := configSet("auto_apply_gpu_state", nil)
	if err != nil {
		t.Fatalf("set-without-value must exit 0: %v", err)
	}
	if !strings.Contains(text, "auto_apply_gpu_state: true") {
		t.Fatalf("set-without-value must print the current value: %s", text)
	}
}

func TestConfigSet_MissingDir_GracefulNA(t *testing.T) {
	dir := t.TempDir()
	old := tomlPath
	tomlPath = filepath.Join(dir, "no-such-dir", "cardwire.toml")
	defer func() { tomlPath = old }()
	text, err := configSet("experimental_nvidia_block", strptr("true"))
	if err != nil {
		t.Fatalf("missing config dir must exit 0: %v", err)
	}
	if !strings.Contains(text, "N/A") {
		t.Fatalf("missing dir must report N/A: %s", text)
	}
}

func TestConfigSet_InvalidValue_Exit1(t *testing.T) {
	dir := t.TempDir()
	old := tomlPath
	tomlPath = filepath.Join(dir, "cardwire.toml")
	defer func() { tomlPath = old }()
	if _, err := configSet("experimental_nvidia_block", strptr("banana")); err == nil {
		t.Fatal("a non-bool value for a bool key must exit 1")
	}
	if _, err := configSet("battery_auto_switch_mode", strptr("turbo")); err == nil {
		t.Fatal("an unknown mode name must exit 1")
	}
}

func TestConfigSet_UnknownKey_Exit1(t *testing.T) {
	old := tomlPath
	tomlPath = filepath.Join(t.TempDir(), "cardwire.toml")
	defer func() { tomlPath = old }()
	if _, err := configGet("not_a_key"); err == nil {
		t.Fatal("a non-whitelisted key must exit 1 on get")
	}
	if _, err := configSet("not_a_key", strptr("true")); err == nil {
		t.Fatal("a non-whitelisted key must exit 1 on set")
	}
}

func TestTomlKeyString(t *testing.T) {
	content := []byte("auto_apply_gpu_state = true\nbattery_auto_switch_mode = \"hybrid\"\n")
	if v := tomlKeyString(content, "auto_apply_gpu_state"); v != "true" {
		t.Fatalf("expected true, got %q", v)
	}
	if v := tomlKeyString(content, "battery_auto_switch_mode"); v != "hybrid" {
		t.Fatalf("expected hybrid, got %q", v)
	}
	if v := tomlKeyString(content, "experimental_nvidia_block"); v != "N/A" {
		t.Fatalf("expected N/A for missing key, got %q", v)
	}
	if v := tomlKeyString([]byte("not = toml [[["), "auto_apply_gpu_state"); v != "N/A" {
		t.Fatalf("expected N/A for corrupt toml, got %q", v)
	}
}

func strptr(s string) *string { return &s }
