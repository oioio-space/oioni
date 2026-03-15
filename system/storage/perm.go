package storage

import (
	"os"

	"github.com/spf13/afero"
)

// permVolume returns a Volume representing the gokrazy /perm partition.
// /perm is already mounted by gokrazy at boot — we just wrap it with afero.
func permVolume(permPath string) *Volume {
	os.MkdirAll(permPath, 0755)
	return &Volume{
		Name:       "perm",
		Device:     "",
		MountPath:  permPath,
		FSType:     "perm",
		Persistent: true,
		FS:         afero.NewBasePathFs(afero.NewOsFs(), permPath),
	}
}
