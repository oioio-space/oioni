module github.com/oioio-space/oioni/cmd/oioni

go 1.26

require (
	github.com/oioio-space/oioni/drivers/epd v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/drivers/touch v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/drivers/usbgadget v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/imgvol v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/storage v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/ui/gui v0.0.0-00010101000000-000000000000
	github.com/spf13/afero v1.15.0
)

require (
	github.com/oioio-space/oioni/ui/canvas v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/image v0.37.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace (
	github.com/oioio-space/oioni/drivers/epd => ../../drivers/epd
	github.com/oioio-space/oioni/drivers/touch => ../../drivers/touch
	github.com/oioio-space/oioni/drivers/usbgadget => ../../drivers/usbgadget
	github.com/oioio-space/oioni/system/imgvol => ../../system/imgvol
	github.com/oioio-space/oioni/system/storage => ../../system/storage
	github.com/oioio-space/oioni/ui/canvas => ../../ui/canvas
	github.com/oioio-space/oioni/ui/gui => ../../ui/gui
)
