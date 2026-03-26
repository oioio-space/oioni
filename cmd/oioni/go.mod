module github.com/oioio-space/oioni/cmd/oioni

go 1.26

require (
	github.com/oioio-space/oioni/drivers/epd v0.0.0
	github.com/oioio-space/oioni/drivers/touch v0.0.0
	github.com/oioio-space/oioni/drivers/usbgadget v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/imgvol v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/netconf v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/storage v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/wifi v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/tools v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/ui/canvas v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/ui/gui v0.0.0-00010101000000-000000000000
	github.com/spf13/afero v1.15.0
	github.com/vishvananda/netlink v1.3.1
)

require (
	github.com/insomniacslk/dhcp v0.0.0-20260220084031-5adc3eb26f91 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/mdlayher/packet v1.1.2 // indirect
	github.com/mdlayher/socket v0.4.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	github.com/u-root/uio v0.0.0-20230220225925-ffce2a382923 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	golang.org/x/image v0.37.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	periph.io/x/conn/v3 v3.7.2 // indirect
	periph.io/x/host/v3 v3.8.5 // indirect
	rsc.io/qr v0.2.0 // indirect
)

replace (
	github.com/oioio-space/oioni/drivers/epd => ../../drivers/epd
	github.com/oioio-space/oioni/drivers/touch => ../../drivers/touch
	github.com/oioio-space/oioni/drivers/usbgadget => ../../drivers/usbgadget
	github.com/oioio-space/oioni/system/imgvol => ../../system/imgvol
	github.com/oioio-space/oioni/system/netconf => ../../system/netconf
	github.com/oioio-space/oioni/system/storage => ../../system/storage
	github.com/oioio-space/oioni/system/wifi => ../../system/wifi
	github.com/oioio-space/oioni/tools => ../../tools
	github.com/oioio-space/oioni/ui/canvas => ../../ui/canvas
	github.com/oioio-space/oioni/ui/gui => ../../ui/gui
)
