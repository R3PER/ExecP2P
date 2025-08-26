package discovery

import (
	"execp2p/internal/platform"
	"net"
)

// getBroadcastAddresses returns platform-specific broadcast addresses
func getBroadcastAddresses() []string {
	return platform.GetNetworkBroadcastAddresses()
}

// setBroadcastSocket configures a UDP socket for broadcast
func setBroadcastSocket(conn *net.UDPConn) error {
	// Ustawiamy socket jako broadcast dla wszystkich platform
	// Używamy metody SetSockopt z pakietu syscall
	fileDescriptor, err := conn.File()
	if err != nil {
		return err
	}
	defer fileDescriptor.Close()

	fd := int(fileDescriptor.Fd())
	return setBroadcastSockopt(fd)
}
