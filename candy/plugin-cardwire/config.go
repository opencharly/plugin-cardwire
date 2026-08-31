package cardwire

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// config.go owns `cardwire config get|set <key> [value]`: reads/writes
// /etc/cardwire/cardwire.toml with an ATOMIC tmp+rename write (mirroring cardwire's
// own approach — a torn config file would strand the daemon's persisted state). Keys
// are whitelisted (ConfigKeys). Missing root/file → graceful N/A, exit 0.

// configGet prints the current value of a whitelisted key (or N/A).
func configGet(key string) (string, error) {
	if !isConfigKey(key) {
		return fmt.Sprintf("cardwire config: unknown key %q (whitelist: %s)\n", key, strings.Join(ConfigKeys, ", ")), fmt.Errorf("cardwire config: unknown key %q", key)
	}
	content, err := os.ReadFile(tomlPath)
	if err != nil {
		return fmt.Sprintf("cardwire config %s: N/A (config file %s not found)\n", key, tomlPath), nil
	}
	return fmt.Sprintf("cardwire config %s: %s\n", key, tomlKeyString(content, key)), nil
}

// configSet writes a whitelisted key (atomic tmp+rename). A nil value prints the
// current value (set-without-value). A missing root/file is a graceful N/A, exit 0.
func configSet(key string, value *string) (string, error) {
	if !isConfigKey(key) {
		return fmt.Sprintf("cardwire config: unknown key %q (whitelist: %s)\n", key, strings.Join(ConfigKeys, ", ")), fmt.Errorf("cardwire config: unknown key %q", key)
	}
	if value == nil {
		return configGet(key)
	}
	val := *value
	if !validConfigValue(key, val) {
		return fmt.Sprintf("cardwire config: invalid value %q for %s\n", val, key), fmt.Errorf("cardwire config: invalid value %q for %s", val, key)
	}
	if _, err := os.Stat(filepath.Dir(tomlPath)); err != nil {
		return fmt.Sprintf("cardwire config %s: N/A (config dir %s not found)\n", key, filepath.Dir(tomlPath)), nil
	}
	// Load the existing doc (an absent file starts empty); set; marshal; atomic write.
	doc := map[string]any{}
	if content, err := os.ReadFile(tomlPath); err == nil {
		_ = toml.Unmarshal(content, &doc) // a partially-corrupt file is replaced by the write below
	}
	if strings.EqualFold(val, "true") || strings.EqualFold(val, "false") {
		doc[key] = strings.EqualFold(val, "true")
	} else {
		doc[key] = val
	}
	out, err := toml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("cardwire config: marshal failed: %v", err)
	}
	if err := atomicWrite(tomlPath, out); err != nil {
		return "", fmt.Errorf("cardwire config: write failed: %v", err)
	}
	return fmt.Sprintf("cardwire config %s: %s (written)\n", key, val), nil
}

// isConfigKey reports whether key is on the whitelist.
func isConfigKey(key string) bool {
	for _, k := range ConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

// validConfigValue validates a value against the key's type: bool keys accept
// true/false; battery_auto_switch_mode accepts a cardwire mode name.
func validConfigValue(key, val string) bool {
	switch key {
	case "battery_auto_switch_mode":
		switch strings.ToLower(val) {
		case "integrated", "hybrid", "manual", "smart":
			return true
		}
		return false
	default:
		return strings.EqualFold(val, "true") || strings.EqualFold(val, "false")
	}
}

// atomicWrite writes content to path via a temp file in the same dir + rename.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cardwire-toml-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
