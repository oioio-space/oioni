package imgvol

import (
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed bin/mkfs.fat
var binFAT []byte

//go:embed bin/mkfs.exfat
var binExFAT []byte

//go:embed bin/mkfs.ext4
var binExt4 []byte

var (
	extractOnceDir  = "/tmp/imgvol-bin"
	extractInitOnce sync.Once
	extractErr      error
)

// extractBinaries writes embedded binaries to extractOnceDir exactly once.
func extractBinaries() error {
	extractInitOnce.Do(func() {
		if err := os.MkdirAll(extractOnceDir, 0755); err != nil {
			extractErr = err
			return
		}
		bins := []struct {
			name string
			data []byte
		}{
			{"mkfs.fat", binFAT},
			{"mkfs.exfat", binExFAT},
			{"mkfs.ext4", binExt4},
		}
		for _, b := range bins {
			name, data := b.name, b.data
			dest := filepath.Join(extractOnceDir, name)
			if err := os.WriteFile(dest, data, fs.FileMode(0755)); err != nil {
				extractErr = fmt.Errorf("extract %s: %w", name, err)
				return
			}
		}
	})
	return extractErr
}

// binPath returns the path to the extracted binary for fstype.
func binPath(fstype FSType) string {
	return filepath.Join(extractOnceDir, "mkfs."+binSuffix(fstype))
}

func binSuffix(fstype FSType) string {
	switch fstype {
	case FAT:
		return "fat"
	case ExFAT:
		return "exfat"
	default: // Ext4
		return "ext4"
	}
}

// mkfsArgs returns the arguments to pass to the mkfs binary for the given fstype.
// The device/image path is always the last argument.
func mkfsArgs(fstype FSType, path string) []string {
	switch fstype {
	case FAT:
		return []string{"-F", "32", path}
	case ExFAT:
		return []string{path}
	default: // Ext4
		return []string{"-t", "ext4", "-F", path}
	}
}

// runMkfs executes the mkfs binary with the given target path.
func runMkfs(fstype FSType, target string) error {
	bin := binPath(fstype)
	args := mkfsArgs(fstype, target)
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("format %s (%s): %w\n%s", fstype, bin, err, out)
	}
	return nil
}

// format runs the appropriate mkfs binary on path.
func format(path string, fstype FSType) error {
	if err := extractBinaries(); err != nil {
		return fmt.Errorf("format extract: %w", err)
	}
	return runMkfs(fstype, path)
}
