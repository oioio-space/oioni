// usbgadget/modules/embed.go
package modules

import "embed"

//go:embed 6.12.47-v8/*.ko
var koFS embed.FS
