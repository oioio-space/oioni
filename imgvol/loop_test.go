package imgvol

import (
	"os"
	"testing"
)

func TestDetectFSType(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(f *os.File)
		want    FSType
		wantErr bool
	}{
		{
			name: "exfat",
			setup: func(f *os.File) {
				buf := make([]byte, 1082)
				copy(buf[3:], "EXFAT   ")
				f.Write(buf)
			},
			want: ExFAT,
		},
		{
			name: "ntfs",
			setup: func(f *os.File) {
				buf := make([]byte, 1082)
				copy(buf[3:], "NTFS    ")
				f.Write(buf)
			},
			want: NTFS,
		},
		{
			name: "fat",
			setup: func(f *os.File) {
				buf := make([]byte, 1082)
				buf[510] = 0x55
				buf[511] = 0xAA
				f.Write(buf)
			},
			want: FAT,
		},
		{
			name: "ext4",
			setup: func(f *os.File) {
				buf := make([]byte, 1082)
				buf[0x438] = 0x53
				buf[0x439] = 0xEF
				f.Write(buf)
			},
			want: Ext4,
		},
		{
			name: "unknown",
			setup: func(f *os.File) {
				f.Write(make([]byte, 1082))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp(t.TempDir(), "img-*")
			if err != nil {
				t.Fatal(err)
			}
			tt.setup(f)
			f.Close()
			got, err := detectFSType(f.Name())
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
