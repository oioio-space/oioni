// Package usbdetect detects USB mass storage partitions using a combination
// of an initial sysfs scan (for drives already connected at boot) and a
// kernel AF_NETLINK KOBJECT_UEVENT socket for hot-plug events.
package usbdetect
