// Package storage manages persistent and ephemeral storage on gokrazy.
//
// It wraps the gokrazy /perm partition and auto-detects USB mass storage
// drives via kernel netlink events, exposing each volume as an afero.Fs.
//
// # Quick Start
//
//	m := storage.New(
//	    storage.WithOnMount(func(v *storage.Volume) {
//	        log.Printf("mounted: %s (%s) at %s", v.Name, v.FSType, v.MountPath)
//	        afero.WriteFile(v.FS, "hello.txt", []byte("hi"), 0644)
//	    }),
//	    storage.WithOnUnmount(func(v *storage.Volume) {
//	        log.Printf("unmounted: %s", v.Name)
//	    }),
//	)
//	if err := m.Start(ctx); err != nil {
//	    log.Printf("storage: %v", err)
//	}
package storage
