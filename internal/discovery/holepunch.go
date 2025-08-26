package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"execp2p/internal/logger"
)

// HolePunchingMessage to struktura wiadomości używana w procesie hole punching
type HolePunchingMessage struct {
	Type       string `json:"type"`
	SenderAddr string `json:"sender_addr"`
	RoomID     string `json:"room_id"`
	Port       int    `json:"port"`
}

// Stałe reprezentujące typy wiadomości
const (
	HPMsgPunch     = "punch"     // Wiadomość inicjująca hole punching
	HPMsgPong      = "pong"      // Odpowiedź na wiadomość punch
	HPMsgConnected = "connected" // Potwierdzenie nawiązania połączenia
)

// Globalny kanał używany do komunikacji między goroutines
var (
	successChan = make(chan string, 1) // Kanał do przekazywania adresu po udanym połączeniu
)

// InitiateHolePunching inicjuje procedurę hole punching do wskazanego adresu
// Zwraca adres, pod którym udało się nawiązać połączenie lub błąd
func InitiateHolePunching(ctx context.Context, remoteAddr, roomID string, localPort int) (string, error) {
	logger.L().Info("Inicjowanie UDP hole punching", "remote", remoteAddr, "local_port", localPort)

	// Najpierw spróbuj uzyskać zewnętrzny adres
	externalAddr, err := ExternalUDPAddr(localPort)
	if err != nil {
		logger.L().Warn("Nie udało się uzyskać zewnętrznego adresu", "err", err)
		// Kontynuujemy mimo tego, może zadziała
	}

	// Parsuj adres docelowy
	remoteUDPAddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return "", fmt.Errorf("nieprawidłowy adres docelowy: %w", err)
	}

	// Utwórz socket do komunikacji
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: localPort})
	if err != nil {
		return "", fmt.Errorf("nie można nasłuchiwać na porcie %d: %w", localPort, err)
	}
	defer conn.Close()

	// Utwórz kontekst z timeout
	punchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Wyczyść kanał przed użyciem
	select {
	case <-successChan:
		// Opróżnij kanał
	default:
		// Kanał jest pusty
	}

	// Goroutine do wysyłania pakietów "punch"
	go sendPunchingPackets(punchCtx, conn, remoteUDPAddr, externalAddr, roomID, localPort)

	// Goroutine do nasłuchiwania odpowiedzi
	go listenForPunchResponses(punchCtx, conn, roomID)

	// Czekaj na sukces lub timeout
	select {
	case addr := <-successChan:
		logger.L().Info("Hole punching zakończony sukcesem", "addr", addr)
		return addr, nil
	case <-punchCtx.Done():
		return "", fmt.Errorf("timeout podczas UDP hole punching")
	}
}

// RespondToHolePunching odpowiada na żądania hole punching
func RespondToHolePunching(ctx context.Context, localPort int, roomID string) error {
	logger.L().Info("Uruchamianie responder'a hole punching", "port", localPort)

	// Utwórz socket do nasłuchiwania
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: localPort})
	if err != nil {
		return fmt.Errorf("nie można nasłuchiwać na porcie %d: %w", localPort, err)
	}

	// Goroutine nasłuchująca żądań punch
	go func() {
		defer conn.Close()
		buf := make([]byte, 1024)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Ustaw deadline odczytu
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				n, addr, err := conn.ReadFromUDP(buf)
				if err != nil {
					continue
				}

				// Parsuj wiadomość
				var msg HolePunchingMessage
				if err := json.Unmarshal(buf[:n], &msg); err != nil {
					continue
				}

				// Jeśli to jest wiadomość punch dla naszego pokoju
				if msg.Type == HPMsgPunch && msg.RoomID == roomID {
					logger.L().Debug("Odebrano żądanie hole punching", "from", addr.String())

					// Odpowiedz pong
					response := HolePunchingMessage{
						Type:       HPMsgPong,
						SenderAddr: addr.String(), // Zwróć adres, z którego odebraliśmy
						RoomID:     roomID,
						Port:       localPort,
					}

					if respBytes, err := json.Marshal(response); err == nil {
						conn.WriteToUDP(respBytes, addr)

						// Wyślij też potwierdzenie connected
						time.Sleep(500 * time.Millisecond)
						confirm := HolePunchingMessage{
							Type:       HPMsgConnected,
							SenderAddr: addr.String(),
							RoomID:     roomID,
							Port:       localPort,
						}
						if confBytes, err := json.Marshal(confirm); err == nil {
							conn.WriteToUDP(confBytes, addr)
						}
					}
				}
			}
		}
	}()

	return nil
}

// sendPunchingPackets wysyła pakiety UDP "punch" do zdalnego adresu
func sendPunchingPackets(ctx context.Context, conn *net.UDPConn, remoteAddr *net.UDPAddr, externalAddr, roomID string, localPort int) {
	// Przygotuj wiadomość
	msg := HolePunchingMessage{
		Type:       HPMsgPunch,
		SenderAddr: externalAddr, // Nasz zewnętrzny adres (jeśli znamy)
		RoomID:     roomID,
		Port:       localPort,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		logger.L().Error("Błąd serializacji wiadomości", "err", err)
		return
	}

	// Wysyłaj pakiety co 500ms
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Wyślij pakiet "punch"
			if _, err := conn.WriteToUDP(msgBytes, remoteAddr); err != nil {
				logger.L().Warn("Nie udało się wysłać pakietu punch", "err", err)
			}
		}
	}
}

// listenForPunchResponses nasłuchuje odpowiedzi na pakiety punch
func listenForPunchResponses(ctx context.Context, conn *net.UDPConn, roomID string) {
	buf := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Ustaw timeout odczytu
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			// Parsuj odpowiedź
			var msg HolePunchingMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}

			// Sprawdź czy to odpowiedź pong lub connected dla naszego pokoju
			if (msg.Type == HPMsgPong || msg.Type == HPMsgConnected) && msg.RoomID == roomID {
				logger.L().Info("Odebrano odpowiedź hole punching", "type", msg.Type, "from", addr.String())

				// Jeśli to pong, wyślij potwierdzenie connected
				if msg.Type == HPMsgPong {
					confirmMsg := HolePunchingMessage{
						Type:       HPMsgConnected,
						SenderAddr: msg.SenderAddr, // Adres, który nam przysłano
						RoomID:     roomID,
						Port:       msg.Port,
					}
					if confBytes, err := json.Marshal(confirmMsg); err == nil {
						conn.WriteToUDP(confBytes, addr)
					}
				}

				// Jeśli to connected, oznajmij sukces
				if msg.Type == HPMsgConnected {
					// Zwróć adres, z którym udało się nawiązać połączenie
					successAddr := addr.String()
					select {
					case successChan <- successAddr:
						// Udało się wysłać
					default:
						// Kanał pełny, zignoruj
					}
					return
				}
			}
		}
	}
}
