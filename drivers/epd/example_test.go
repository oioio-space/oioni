package epd_test

import (
	"fmt"

	"github.com/oioio-space/oioni/drivers/epd"
)

func ExampleNew_missingDevice() {
	_, err := epd.New(epd.Config{
		SPIDevice: "/dev/spidev_nonexistent",
		SPISpeed:  4_000_000,
		PinRST:    17, PinDC: 25, PinCS: 8, PinBUSY: 24,
	})
	fmt.Println(err != nil)
	// Output: true
}
