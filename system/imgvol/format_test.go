package imgvol

import (
	"testing"
)

func TestMkfsArgs(t *testing.T) {
	tests := []struct {
		fstype FSType
		path   string
		want   []string
	}{
		{FAT, "/tmp/test.img", []string{"-F", "32", "/tmp/test.img"}},
		{ExFAT, "/tmp/test.img", []string{"/tmp/test.img"}},
		{Ext4, "/tmp/test.img", []string{"-t", "ext4", "-F", "/tmp/test.img"}},
	}
	for _, tt := range tests {
		got := mkfsArgs(tt.fstype, tt.path)
		if len(got) != len(tt.want) {
			t.Errorf("%s: args %v, want %v", tt.fstype, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: arg[%d] = %q, want %q", tt.fstype, i, got[i], tt.want[i])
			}
		}
	}
}
