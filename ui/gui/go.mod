module github.com/oioio-space/oioni/ui/gui

go 1.26

require (
	github.com/oioio-space/oioni/drivers/epd v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/drivers/touch v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/ui/canvas v0.0.0-00010101000000-000000000000
	rsc.io/qr v0.2.0
)

require (
	golang.org/x/image v0.37.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace (
	github.com/oioio-space/oioni/drivers/epd => ../../drivers/epd
	github.com/oioio-space/oioni/drivers/touch => ../../drivers/touch
	github.com/oioio-space/oioni/ui/canvas => ../../ui/canvas
)
