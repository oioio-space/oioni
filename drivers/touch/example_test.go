package touch_test

import (
	"fmt"

	"github.com/oioio-space/oioni/drivers/touch"
)

func ExampleNew_missingDevice() {
	_, err := touch.New(touch.Config{
		I2CDevice: "/dev/i2c-nonexistent",
		I2CAddr:   0x14,
		PinTRST:   22,
		PinINT:    27,
	})
	fmt.Println(err != nil)
	// Output: true
}
