// epaper/epd/hal.go
package epd

// SPIConn is a write-only SPI connection.
type SPIConn interface {
	Tx(w []byte) error
}

// OutputPin drives a GPIO signal (RST, DC, CS).
type OutputPin interface {
	Out(high bool) error
}

// InputPin reads a GPIO input (BUSY).
type InputPin interface {
	Read() bool
}
