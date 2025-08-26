//go:build !windows
// +build !windows

package discovery

import (
	"golang.org/x/sys/unix"
)

// setBroadcastSockopt ustawia opcję SO_BROADCAST na sockecie
// dla systemów Unix-like (Linux, macOS)
func setBroadcastSockopt(fd int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
}
