//go:build windows
// +build windows

package discovery

import (
	"golang.org/x/sys/windows"
)

// setBroadcastSockopt ustawia opcję SO_BROADCAST na sockecie
// dla systemu Windows
func setBroadcastSockopt(fd int) error {
	return windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_BROADCAST, 1)
}
