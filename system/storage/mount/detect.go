// Package mount provides filesystem type detection and syscall-level
// mount/unmount operations for use on gokrazy (no udev, no blkid).
package mount

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// DetectFSType reads magic bytes from device to identify the filesystem type.
// Supported: ext4, vfat (FAT16/FAT32), exfat, ntfs.
func DetectFSType(device string) (string, error) {
	f, err := os.Open(device)
	if err != nil {
		return "", fmt.Errorf("detect fstype: %w", err)
	}
	defer f.Close()

	// Read enough bytes to cover all magic offsets.
	// ext4 superblock magic @ 0x438 (1080) needs 1082 bytes minimum.
	const readSize = 1082
	buf := make([]byte, readSize)
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", fmt.Errorf("detect fstype read: %w", err)
	}

	// exFAT: "EXFAT   " (8 bytes) at offset 3 — check before FAT (FAT sig may also match)
	if len(buf) >= 11 && string(buf[3:11]) == "EXFAT   " {
		return "exfat", nil
	}

	// NTFS: OEM ID "NTFS    " (8 bytes, 4 trailing spaces) at offset 3
	if len(buf) >= 11 && string(buf[3:11]) == "NTFS    " {
		return "ntfs", nil
	}

	// ext4: superblock magic 0xEF53 (little-endian) at offset 0x438
	if len(buf) >= 0x43A {
		magic := binary.LittleEndian.Uint16(buf[0x438:0x43A])
		if magic == 0xEF53 {
			return "ext4", nil
		}
	}

	// FAT: boot sector signature 0x55AA at offset 510
	if len(buf) >= 512 && buf[510] == 0x55 && buf[511] == 0xAA {
		return "vfat", nil
	}

	return "", fmt.Errorf("detect fstype %s: unrecognized filesystem", device)
}
