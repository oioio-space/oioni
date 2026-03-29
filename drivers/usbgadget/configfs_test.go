// usbgadget/configfs_test.go — internal tests for configfs teardown
package usbgadget

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTeardownConfigfs_LogsIntermediateErrors verifies that when an
// intermediate os.Remove fails (e.g. because a directory is non-empty),
// the error is logged rather than silently swallowed. Without the fix all
// errors except the final os.Remove(dir) are discarded.
func TestTeardownConfigfs_LogsIntermediateErrors(t *testing.T) {
	dir := t.TempDir()
	origRoot := configfsRoot
	configfsRoot = dir
	t.Cleanup(func() { configfsRoot = origRoot })

	g := &Gadget{name: "testgadget", langID: "0x409"}

	// Build a minimal configfs tree.
	gadgetPath := g.gadgetDir()
	cfgStringsDir := filepath.Join(gadgetPath, "configs", "c.1", "strings", "0x409")
	if err := os.MkdirAll(cfgStringsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Place an extra file inside configs/c.1/strings so that
	// removing configs/c.1/strings/0x409 succeeds but removing
	// configs/c.1 fails (non-empty: still contains "strings").
	if err := os.WriteFile(filepath.Join(gadgetPath, "configs", "c.1", "strings", "extra"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gadgetPath, "functions"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gadgetPath, "strings", "0x409"), 0755); err != nil {
		t.Fatal(err)
	}

	// Capture log output.
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	_ = g.teardownConfigfs()

	// Without fix: no log output for intermediate errors.
	// With fix: the failed removal of configs/c.1 (non-empty) is logged.
	logged := buf.String()
	if !strings.Contains(logged, "configs") && !strings.Contains(logged, "usbgadget") {
		// Only an error if the configs/c.1 directory still exists
		// (meaning the remove actually failed but was silently ignored).
		if _, err := os.Stat(filepath.Join(gadgetPath, "configs", "c.1")); err == nil {
			t.Error("teardownConfigfs failed to remove configs/c.1 but logged nothing")
		}
	}
}
