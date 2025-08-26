package discovery

import (
	"fmt"
	"net"
	"time"

	"github.com/pion/stun"
)

// ExternalUDPAddr gets our external IP:port by asking a STUN server
// Używa wielu serwerów STUN jako fallback, jeśli jeden nie odpowiada
func ExternalUDPAddr(localPort int) (string, error) {
	// Lista serwerów STUN do próbowania
	stunServers := []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
		"stun.twilio.com:3478",
		"stun.stunprotocol.org:3478",
	}

	// Sprawdź czy port jest dostępny
	if !isPortAvailable(localPort) {
		// Spróbuj znaleźć inny dostępny port
		for i := localPort + 1; i < localPort+100; i++ {
			if isPortAvailable(i) {
				localPort = i
				break
			}
		}
	}

	// Przechowuje ostatni błąd
	var lastError error

	// Spróbuj każdego serwera STUN z listy
	for _, server := range stunServers {
		addr, err := tryStunServer(server, localPort)
		if err != nil {
			lastError = err
			continue
		}
		return addr, nil
	}

	return "", fmt.Errorf("nie udało się uzyskać zewnętrznego adresu UDP: %v", lastError)
}

// tryStunServer próbuje uzyskać zewnętrzny adres z jednego serwera STUN
// Parametr localPort nie jest obecnie używany, ale jest zachowany dla
// kompatybilności z przyszłymi rozszerzeniami
func tryStunServer(serverAddr string, _ int) (string, error) {
	// Utwórz połączenie UDP do serwera STUN
	// Używamy standardowego Dial, które wybiera dowolny dostępny port lokalny
	conn, err := net.Dial("udp", serverAddr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Utwórz klienta STUN
	c, err := stun.NewClient(conn)
	if err != nil {
		return "", err
	}
	defer c.Close()

	// Ustaw timeout
	c.SetRTO(time.Second * 5)

	// Wykonaj zapytanie STUN
	var xorAddr stun.XORMappedAddress
	var resultErr error

	err = c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			resultErr = res.Error
			return
		}
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			resultErr = getErr
		}
	})

	if err != nil {
		return "", err
	}

	if resultErr != nil {
		return "", resultErr
	}

	return fmt.Sprintf("%s:%d", xorAddr.IP.String(), xorAddr.Port), nil
}

// isPortAvailable sprawdza czy port UDP jest dostępny
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
