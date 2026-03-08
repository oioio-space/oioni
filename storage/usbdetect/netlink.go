package usbdetect

import (
	"context"
	"fmt"
	"strings"
	"syscall"
)

// listenNetlink opens an AF_NETLINK KOBJECT_UEVENT socket and returns
// a channel that emits parsed Events. Closes when ctx is cancelled.
func listenNetlink(ctx context.Context) (<-chan Event, error) {
	fd, err := syscall.Socket(
		syscall.AF_NETLINK,
		syscall.SOCK_RAW|syscall.SOCK_CLOEXEC,
		syscall.NETLINK_KOBJECT_UEVENT,
	)
	if err != nil {
		return nil, fmt.Errorf("netlink socket: %w", err)
	}

	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: 1,
	}
	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("netlink bind: %w", err)
	}

	ch := make(chan Event, 16)
	go func() {
		defer syscall.Close(fd)
		defer close(ch)
		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, _, err := syscall.Recvfrom(fd, buf, 0)
			if err != nil {
				return
			}
			if ev, ok := parseUevent(buf[:n]); ok {
				select {
				case ch <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		syscall.Close(fd)
	}()

	return ch, nil
}

// parseUevent parses a raw kernel uevent message (null-separated key=value pairs).
// Returns the Event and true if it's a block partition add/remove event.
func parseUevent(msg []byte) (Event, bool) {
	parts := strings.Split(string(msg), "\x00")
	if len(parts) < 2 {
		return Event{}, false
	}

	kv := make(map[string]string, len(parts))
	for _, p := range parts[1:] {
		if idx := strings.IndexByte(p, '='); idx > 0 {
			kv[p[:idx]] = p[idx+1:]
		}
	}

	if kv["SUBSYSTEM"] != "block" {
		return Event{}, false
	}
	if kv["DEVTYPE"] != "partition" {
		return Event{}, false
	}
	action := kv["ACTION"]
	if action != "add" && action != "remove" {
		return Event{}, false
	}
	devName := kv["DEVNAME"]
	if devName == "" {
		return Event{}, false
	}

	return Event{
		Action: action,
		Device: "/dev/" + devName,
	}, true
}
