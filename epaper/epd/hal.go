// epaper/epd/hal.go
package epd

import (
	"fmt"
	"unsafe"
	"golang.org/x/sys/unix"
)

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

// spiIOCTransfer mirrors the kernel struct spi_ioc_transfer (64-bit ARM, 32 bytes).
// The anonymous [5]byte field intentionally zeroes cs_change, tx_nbits, rx_nbits,
// word_delay_usecs and pad — all unused for this write-only e-ink use case.
type spiIOCTransfer struct {
	txBuf       uint64
	rxBuf       uint64
	length      uint32
	speedHz     uint32
	delayUsecs  uint16
	bitsPerWord uint8
	_           [5]byte // cs_change, tx_nbits, rx_nbits, word_delay_usecs, pad
}

// Compile-time assertion: struct must be exactly 32 bytes on ARM64.
var _ [32]byte = [unsafe.Sizeof(spiIOCTransfer{})]byte{}

const (
	// spiIOCMessage1 = _IOW('k', 0, spi_ioc_transfer) = 0x40206b00 (ARM64, sizeof=32).
	spiIOCMessage1 = 0x40206b00
	// spiIOCWrMode = _IOW('k', 1, uint8) = SPI_IOC_WR_MODE.
	spiIOCWrMode = 0x40016b01
	// spiIOCWrBitsPerWord = _IOW('k', 3, uint8) = SPI_IOC_WR_BITS_PER_WORD.
	spiIOCWrBitsPerWord = 0x40016b03
)

type linuxSPI struct {
	fd    int
	speed uint32
}

func openSPI(device string, speed uint32) (*linuxSPI, error) {
	fd, err := unix.Open(device, unix.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", device, err)
	}
	mode := uint8(0) // SPI_MODE_0
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), spiIOCWrMode, uintptr(unsafe.Pointer(&mode))); errno != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("set SPI mode: %w", errno)
	}
	bits := uint8(8)
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), spiIOCWrBitsPerWord, uintptr(unsafe.Pointer(&bits))); errno != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("set SPI bits: %w", errno)
	}
	return &linuxSPI{fd: fd, speed: speed}, nil
}

func (s *linuxSPI) Tx(w []byte) error {
	if len(w) == 0 {
		return nil
	}
	t := spiIOCTransfer{
		txBuf:       uint64(uintptr(unsafe.Pointer(&w[0]))),
		length:      uint32(len(w)),
		speedHz:     s.speed,
		bitsPerWord: 8,
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(s.fd), uintptr(spiIOCMessage1), uintptr(unsafe.Pointer(&t)))
	if errno != 0 {
		return fmt.Errorf("SPI Tx: %w", errno)
	}
	return nil
}

func (s *linuxSPI) Close() error { return unix.Close(s.fd) }
