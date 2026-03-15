//go:build ignore

package imgvol_test

import (
	"fmt"
	"log"

	"github.com/oioio-space/oioni/system/imgvol"
	"github.com/spf13/afero"
)

// ExampleCreate demonstrates creating a FAT image, writing a file, and reading it back.
// Requires Linux root and ARM64 — run on the target device, not in CI.
func ExampleCreate() {
	const path = "/tmp/example.img"

	if err := imgvol.Create(path, 32<<20, imgvol.FAT); err != nil {
		log.Fatal(err)
	}

	vol, err := imgvol.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer vol.Close()

	if err := afero.WriteFile(vol.FS, "hello.txt", []byte("hello imgvol"), 0644); err != nil {
		log.Fatal(err)
	}

	data, err := afero.ReadFile(vol.FS, "hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
	// Output: hello imgvol
}
