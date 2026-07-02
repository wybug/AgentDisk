package feature

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// persistFlags writes the features.* keys back to config.yaml without
// disturbing other sections. Viper's WriteConfig does not preserve comments
// in YAML, so the file loses its inline commentary on first PATCH; this is
// a known trade-off. The features: block is always rewritten from the live
// snapshot so the file reflects exactly what's running.
//
// Approach:
//  1. Read the existing file into a fresh viper instance (so we have all
//     keys, not just features).
//  2. Overwrite the features.* keys with the live values.
//  3. Write back to a temp file in the same dir, then atomic rename over the
//     original. The temp file uses a .yaml suffix so viper picks the right
//     encoder; we explicitly set ConfigType as a belt-and-braces measure.
//
// This is best-effort: a failure to persist is logged + returned, but the
// in-memory state was already updated by Set. The next restart reads
// whatever is on disk.
func persistFlags(cfgPath string, flags *Flags) error {
	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")
	// Read existing config; if it doesn't parse we abort — we don't want to
	// rewrite a file we can't understand, since that could drop keys.
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}
	v.Set("features.okfReader", flags.OkfReader)
	v.Set("features.okfWriter", flags.OkfWriter)
	v.Set("features.okfGraphBFS", flags.OkfGraphBFS)

	// viper.WriteConfig uses the registered file's extension to pick the
	// serializer. We write to a sibling temp file with .yaml suffix so the
	// serializer is unambiguous, then atomic-rename over the original. The
	// rename guarantees a reader never sees a half-written file.
	dir := filepath.Dir(cfgPath)
	tmp, err := os.CreateTemp(dir, "config-*.yaml")
	if err != nil {
		return fmt.Errorf("create tmp config: %w", err)
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	if err := v.WriteConfigAs(tmpName); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("write tmp config: %w", err)
	}
	if err := os.Rename(tmpName, cfgPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename tmp config: %w", err)
	}
	return nil
}
