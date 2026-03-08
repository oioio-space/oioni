package mount

import (
	"os"
	"testing"
)

func writeMagic(t *testing.T, size int, offset int, magic []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fake-dev-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.Write(make([]byte, size))
	f.WriteAt(magic, int64(offset))
	return f.Name()
}

func TestDetectFSType_ext4(t *testing.T) {
	// ext4 superblock magic: 0x53EF little-endian at offset 0x438 (1080)
	f := writeMagic(t, 2048, 0x438, []byte{0x53, 0xEF})
	got, err := DetectFSType(f)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ext4" {
		t.Errorf("got %q, want ext4", got)
	}
}

func TestDetectFSType_vfat(t *testing.T) {
	// FAT boot sector signature: 0x55AA at offset 510
	f := writeMagic(t, 2048, 0x1FE, []byte{0x55, 0xAA})
	got, err := DetectFSType(f)
	if err != nil {
		t.Fatal(err)
	}
	if got != "vfat" {
		t.Errorf("got %q, want vfat", got)
	}
}

func TestDetectFSType_exfat(t *testing.T) {
	// exFAT: "EXFAT   " (8 bytes) at offset 3
	f := writeMagic(t, 2048, 3, []byte("EXFAT   "))
	got, err := DetectFSType(f)
	if err != nil {
		t.Fatal(err)
	}
	if got != "exfat" {
		t.Errorf("got %q, want exfat", got)
	}
}

func TestDetectFSType_unknown(t *testing.T) {
	f := writeMagic(t, 2048, 0, []byte{})
	_, err := DetectFSType(f)
	if err == nil {
		t.Fatal("expected error for unknown FS")
	}
}
