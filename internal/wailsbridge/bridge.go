package wailsbridge

import (
	"context"
	"encoding/json"
	"execp2p/internal/app"
	"execp2p/internal/crypto"
	"execp2p/internal/network" // potrzebne dla typu zwracanego z GetNetworkAccess
	"fmt"
	"math"
	"net"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Bufor wiadomości, które nie zostały wysłane z powodu problemów z połączeniem
var pendingMessages = make([]string, 0)

// EventTypes - typy zdarzeń emitowanych do frontendu
const (
	EventMessageReceived  = "message:received"
	EventStatusUpdate     = "status:update"
	EventSecurityMessage  = "security:message"
	EventNetworkError     = "network:error"
	EventPeerFingerprints = "peer:fingerprints"
	EventNicknameUpdate   = "nickname:update"
)

// Bridge łączy istniejący back-end z Wails
type Bridge struct {
	ctx     context.Context
	execp2p *app.ExecP2P
}

// NewBridge tworzy nową instancję Bridge
func NewBridge(execp2p *app.ExecP2P) *Bridge {
	return &Bridge{
		execp2p: execp2p,
	}
}

// SetContext ustawia kontekst Wails
func (b *Bridge) SetContext(ctx context.Context) {
	b.ctx = ctx
	// Rozpoczęcie monitorowania zdarzeń
	go b.startEventMonitoring(ctx)
	// Uruchomienie mechanizmu keep-alive
	go b.startKeepAlive(ctx)
}

// startKeepAlive wysyła regularne sygnały, aby utrzymać połączenie aktywne
func (b *Bridge) startKeepAlive(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second) // Znacznie częstsze sygnały dla maksymalnej stabilności
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if b.execp2p != nil && b.ctx != nil {
				// Sprawdź status sieci
				status := b.execp2p.GetNetworkStatus()
				if status["is_running"].(bool) && status["connected_peers"].(int) > 0 {
					// Wyślij pusty sygnał keep-alive
					keepAliveMsg := map[string]interface{}{
						"type":    "keep_alive",
						"content": "",
						"time":    time.Now().Unix(),
					}

					msgBytes, err := json.Marshal(keepAliveMsg)
					if err == nil {
						// Ignorujemy błędy, bo to tylko sygnał keep-alive
						_ = b.execp2p.SendMessage(b.ctx, string(msgBytes))
					}
				}
			}
		}
	}
}

// CreateRoom tworzy nowy pokój
func (b *Bridge) CreateRoom() (map[string]interface{}, error) {
	result, err := b.execp2p.CreateRoom(b.ctx)
	if err != nil {
		return nil, err
	}
	// Zwracamy jako mapę ze wszystkimi polami
	return map[string]interface{}{
		"room_id":     result.RoomID,
		"access_key":  result.AccessKey,
		"listen_port": result.ListenPort,
	}, nil
}

// FindRoom wyszukuje pokój w sieci lokalnej i zwraca adres hosta z portem
func (b *Bridge) FindRoom(roomID string) (map[string]interface{}, error) {
	// Emituj komunikat o rozpoczęciu wyszukiwania
	b.EmitSecurityMessage("Wyszukiwanie pokoju " + roomID + "...")

	// Uruchom autodetekcję, aby znaleźć pokój
	ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
	defer cancel()

	// Użyj autodetekcji, aby znaleźć pokój
	addr, err := b.execp2p.TryLocalNetworkDiscovery(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("nie znaleziono pokoju: %w", err)
	}

	// Wyodrębnij adres i port
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("nieprawidłowy format adresu: %w", err)
	}

	// Zwróć znalezione informacje
	return map[string]interface{}{
		"room_id": roomID,
		"address": addr,
		"host":    host,
		"port":    portStr,
	}, nil
}

// GetRoomAccessKey zwraca klucz dostępu do aktualnego pokoju
func (b *Bridge) GetRoomAccessKey() (string, error) {
	// Sprawdź czy bieżący pokój ma klucz dostępu w GetSecuritySummary
	secSummary := b.execp2p.GetSecuritySummary()
	if roomInfo, ok := secSummary["room_info"].(map[string]interface{}); ok {
		if accessKey, ok := roomInfo["access_key"].(string); ok {
			return accessKey, nil
		}
	}

	// Jeśli nie ma klucza, spróbuj go wygenerować
	return b.execp2p.RegenerateRoomAccessKey()
}

// RegenerateRoomAccessKey generuje nowy klucz dostępu dla bieżącego pokoju
func (b *Bridge) RegenerateRoomAccessKey() (string, error) {
	return b.execp2p.RegenerateRoomAccessKey()
}

// JoinRoom dołącza do pokoju (stara metoda)
func (b *Bridge) JoinRoom(roomID string, remoteAddr string, accessKey string) error {
	// Weryfikacja klucza dostępu
	if accessKey == "" {
		return fmt.Errorf("brak klucza dostępu do pokoju")
	}
	return b.execp2p.JoinRoom(b.ctx, roomID, remoteAddr, accessKey)
}

// JoinRoomWithFallback dołącza do pokoju z automatycznymi próbami różnych metod połączenia
// Jest to ulepszona wersja metody JoinRoom, która próbuje różnych metod połączenia
func (b *Bridge) JoinRoomWithFallback(roomID string, accessKey string) error {
	// Weryfikacja klucza dostępu
	if accessKey == "" {
		return fmt.Errorf("brak klucza dostępu do pokoju")
	}

	// Emituj komunikat o rozpoczęciu zaawansowanego łączenia
	b.EmitSecurityMessage("Rozpoczynam zaawansowaną procedurę łączenia...")

	// Używa nowej metody w ExecP2P, która próbuje różnych sposobów połączenia
	return b.execp2p.JoinRoomWithFallback(b.ctx, roomID, accessKey)
}

// SendMessage wysyła wiadomość (tekst lub multimedia)
// retransmitPendingMessages próbuje okresowo wysłać oczekujące wiadomości
func (b *Bridge) retransmitPendingMessages(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(pendingMessages) > 0 && b.execp2p != nil && b.ctx != nil {
				// Sprawdź status połączenia
				status := b.execp2p.GetNetworkStatus()
				if status["is_running"].(bool) && status["connected_peers"].(int) > 0 {
					// Próbuj ponownie wysłać oczekujące wiadomości
					var remainingMessages []string
					for _, msg := range pendingMessages {
						if err := b.execp2p.SendMessage(b.ctx, msg); err != nil {
							// Jeśli nadal nie można wysłać, zachowaj w buforze
							remainingMessages = append(remainingMessages, msg)
						} else {
							fmt.Printf("Pomyślnie wysłano buforowaną wiadomość\n")
						}
					}
					// Zaktualizuj listę oczekujących wiadomości
					pendingMessages = remainingMessages
				}
			}
		}
	}
}

func (b *Bridge) SendMessage(message string) error {
	// Sprawdź czy połączenie istnieje
	if b.execp2p == nil || b.ctx == nil {
		// Dodaj wiadomość do bufora oczekujących
		pendingMessages = append(pendingMessages, message)
		return fmt.Errorf("brak połączenia - wiadomość buforowana")
	}

	// Status połączenia
	status := b.execp2p.GetNetworkStatus()
	if !status["is_running"].(bool) || status["connected_peers"].(int) == 0 {
		// Dodaj wiadomość do bufora oczekujących
		pendingMessages = append(pendingMessages, message)
		return fmt.Errorf("połączenie nie jest aktywne - wiadomość buforowana")
	}

	// Dodatkowe sprawdzenie dla pierwszej wiadomości - 3 próby wysłania
	const maxRetries = 3

	// Pomocnicza funkcja do wielokrotnych prób wysłania wiadomości
	sendWithRetries := func(msg string) error {
		var err error
		for attempt := 0; attempt < maxRetries; attempt++ {
			err = b.execp2p.SendMessage(b.ctx, msg)
			if err == nil {
				return nil // Sukces - wiadomość wysłana
			}

			// Jeśli nie udało się, poczekaj przed kolejną próbą
			// Z każdą próbą zwiększaj czas oczekiwania
			waitTime := time.Duration(50*(attempt+1)) * time.Millisecond
			time.Sleep(waitTime)
			fmt.Printf("Próba wysłania wiadomości %d/%d...\n", attempt+1, maxRetries)
		}
		return err // Zwróć ostatni błąd, jeśli wszystkie próby zawiodły
	}

	// Sprawdź, czy wiadomość jest w formacie JSON (dla multimediów)
	var msgData map[string]interface{}
	if err := json.Unmarshal([]byte(message), &msgData); err == nil {
		// Sprawdź, czy to wiadomość multimedialna
		msgType, hasType := msgData["type"].(string)
		if hasType && (msgType == "audio" || msgType == "image" || msgType == "gif") {
			// Upewnij się, że mamy mediaUrl
			if mediaUrl, hasMedia := msgData["mediaUrl"].(string); hasMedia && mediaUrl != "" {
				// Loguj informację o wykryciu wiadomości multimedialnej
				fmt.Printf("Wykryto wiadomość multimedialną typu %s\n", msgType)
				// Wyślij pełną wiadomość JSON z ponownymi próbami
				return sendWithRetries(message)
			} else {
				// Brak mediaUrl w wiadomości multimedialnej
				return fmt.Errorf("brak URL mediów w wiadomości typu %s", msgType)
			}
		} else {
			// Wiadomość jest poprawnym JSON, ale nie multimedia - wyślij normalnie
			return sendWithRetries(message)
		}
	} else {
		// Standardowa wiadomość tekstowa
		return sendWithRetries(message)
	}
}

// GetNetworkStatus zwraca status sieci
func (b *Bridge) GetNetworkStatus() map[string]interface{} {
	return b.execp2p.GetNetworkStatus()
}

// GetSecuritySummary zwraca podsumowanie bezpieczeństwa
func (b *Bridge) GetSecuritySummary() map[string]interface{} {
	return b.execp2p.GetSecuritySummary()
}

// GetPeerFingerprint zwraca odcisk palca
func (b *Bridge) GetPeerFingerprint() (string, error) {
	return b.execp2p.GetPeerFingerprint()
}

// JoinUserByID dołącza do użytkownika przez ID
// Traktujemy ID użytkownika jako ID pokoju, który jest używany w DHT
func (b *Bridge) JoinUserByID(userID string, accessKey string) error {
	// Weryfikacja klucza dostępu
	if accessKey == "" {
		return fmt.Errorf("brak klucza dostępu do pokoju")
	}
	return b.execp2p.JoinRoom(b.ctx, userID, "", accessKey)
}

// GetUserID zwraca ID tego użytkownika
func (b *Bridge) GetUserID() string {
	// Obecnie używamy peerID jako userID
	return b.execp2p.GetNetworkStatus()["peer_id"].(string)
}

// CloseConnection zamyka bieżące połączenie z pokojem
func (b *Bridge) CloseConnection() error {
	if b.execp2p == nil {
		return fmt.Errorf("bridge nie zainicjalizowany")
	}

	// Wywołaj metodę Close z ExecP2P, która zamyka wszystkie połączenia
	b.execp2p.Close()

	// Emituj komunikat o opuszczeniu pokoju
	runtime.EventsEmit(b.ctx, "room:left")

	return nil
}

// UpdateNickname aktualizuje nickname użytkownika i przekazuje informację do innych uczestników
func (b *Bridge) UpdateNickname(nickname string) error {
	if b.ctx == nil {
		return fmt.Errorf("bridge nie zainicjalizowany")
	}

	// Wyślij wiadomość specjalną zawierającą informację o zmianie nickname'a
	specialMsg := map[string]interface{}{
		"type":     "nickname_update",
		"nickname": nickname,
	}

	msgBytes, err := json.Marshal(specialMsg)
	if err != nil {
		return fmt.Errorf("błąd serializacji: %w", err)
	}

	// Wyślij przez normalny kanał wiadomości
	return b.execp2p.SendMessage(b.ctx, string(msgBytes))
}

// startEventMonitoring monitoruje zdarzenia z back-endu i przekazuje je do frontendu
func (b *Bridge) startEventMonitoring(ctx context.Context) {
	// Monitorowanie wiadomości
	go b.monitorMessages(ctx)

	// Monitorowanie statusu sieci
	go b.monitorNetworkStatus(ctx)

	// Monitorowanie zdarzeń bezpieczeństwa
	go b.monitorSecurity(ctx)
}

// getMessageChannel zwraca kanał wiadomości z istniejącego back-endu
func (b *Bridge) getMessageChannel() <-chan *crypto.MessagePayload {
	var _ network.Network // Trick aby zapobiec usuwaniu importu przez kompilator
	if b.execp2p == nil {
		return nil
	}

	// Pobieramy status sieci aby sprawdzić czy network jest inicjalizowany
	netStatus := b.execp2p.GetNetworkStatus()
	if !netStatus["is_running"].(bool) {
		return nil
	}

	// Uzyskujemy dostęp do kanału wiadomości z sieci
	// Używamy WEWNĘTRZNEJ wiedzy o strukturze ExecP2P, co nie jest idealne
	// ale jest konieczne, dopóki nie dodamy odpowiednich eksporterów do ExecP2P
	network := b.execp2p.GetNetworkAccess()
	if network == nil {
		return nil
	}

	return network.GetIncomingMessages()
}

// monitorMessages odbiera wiadomości z back-endu i przekazuje je do frontendu
func (b *Bridge) monitorMessages(ctx context.Context) {
	if b.execp2p == nil || b.ctx == nil {
		return
	}

	// Uruchom mechanizm retransmisji oczekujących wiadomości
	go b.retransmitPendingMessages(ctx)

	// Monitorowanie rzeczywistych wiadomości
	go func() {
		// Oczekiwanie na inicjalizację połączenia
		reconnectAttempts := 0
		maxReconnectAttempts := 5

		// Licznik aktywności dla adaptacyjnego monitorowania
		lastMsgTime := time.Now()
		adaptiveInterval := 300 * time.Millisecond

		for {
			// Pobierz kanał wiadomości
			msgChan := b.getMessageChannel()
			if msgChan != nil {
				// Resetuj licznik prób po udanym połączeniu
				reconnectAttempts = 0

				// Adaptacyjne dostosowanie interwału sprawdzania - częściej gdy czat jest aktywny
				elapsed := time.Since(lastMsgTime)
				if elapsed < 30*time.Second {
					// Czat był aktywny w ciągu ostatnich 30 sekund - częste sprawdzanie (100ms)
					adaptiveInterval = 100 * time.Millisecond
				} else if elapsed < 2*time.Minute {
					// Czat był aktywny w ciągu ostatnich 2 minut - umiarkowane sprawdzanie (200ms)
					adaptiveInterval = 200 * time.Millisecond
				} else {
					// Czat nieaktywny dłużej niż 2 minuty - rzadsze sprawdzanie (300ms)
					adaptiveInterval = 300 * time.Millisecond
				}

				// Kanał jest dostępny, monitoruj go
				for msg := range msgChan {
					// Zaktualizuj czas ostatniej wiadomości
					lastMsgTime = time.Now()
					if msg == nil {
						continue
					}

					// Obsługa specjalnych wiadomości keep-alive
					var msgDataKeepAlive map[string]interface{}
					if err := json.Unmarshal([]byte(msg.Message), &msgDataKeepAlive); err == nil {
						if msgType, ok := msgDataKeepAlive["type"].(string); ok && msgType == "keep_alive" {
							// Ignoruj wiadomości keep-alive, nie pokazuj ich użytkownikowi
							continue
						}
					}

					// Sprawdź, czy wiadomość zawiera multimedia lub jest wiadomością specjalną (jest w formacie JSON)
					var msgData map[string]interface{}
					messageType := "text"
					messageContent := msg.Message
					var mediaUrl string

					if err := json.Unmarshal([]byte(msg.Message), &msgData); err == nil {
						// Wiadomość może być w formacie JSON
						if msgType, ok := msgData["type"].(string); ok {
							messageType = msgType

							// Obsługa specjalnej wiadomości o aktualizacji nickname'a
							if messageType == "nickname_update" {
								if nickname, ok := msgData["nickname"].(string); ok {
									// Emituj zdarzenie aktualizacji nickname'a
									runtime.EventsEmit(b.ctx, EventNicknameUpdate, map[string]interface{}{
										"sender":   msg.SenderID,
										"nickname": nickname,
									})
									// Nie emituj tej wiadomości jako zwykłej wiadomości
									continue
								}
							}
						}
						if content, ok := msgData["content"].(string); ok {
							messageContent = content
						}
						if url, ok := msgData["mediaUrl"].(string); ok {
							mediaUrl = url
						}
					}

					// Emituj wiadomość do frontendu z dodatkowymi polami dla multimediów
					messageData := map[string]interface{}{
						"sender":    msg.SenderID,
						"message":   messageContent,
						"timestamp": msg.Timestamp,
						"isLocal":   false,
						"verified":  true,
						"type":      messageType,
					}

					// Dodaj URL do multimediów, jeśli istnieje
					if mediaUrl != "" {
						messageData["mediaUrl"] = mediaUrl
					} else if messageType == "audio" || messageType == "image" || messageType == "gif" {
						// Dodatkowe sprawdzenie dla multimediów - sprawdź, czy w oryginalnej wiadomości JSON
						// jest URL, który mogliśmy przeoczyć
						var msgDataMedia map[string]interface{}
						if err := json.Unmarshal([]byte(msg.Message), &msgDataMedia); err == nil {
							if url, ok := msgDataMedia["mediaUrl"].(string); ok && url != "" {
								messageData["mediaUrl"] = url
								// Loguj informację o znalezieniu URL
								fmt.Printf("Znaleziono URL multimediów w wiadomości typu %s\n", messageType)
							}
						}
					}

					runtime.EventsEmit(b.ctx, EventMessageReceived, messageData)
				}
				// Jeśli kanał został zamknięty, spróbuj go pobrać ponownie
				// Użyj krótszego interwału dla szybszego wykrycia ponownego połączenia
				time.Sleep(adaptiveInterval)
			} else {
				// Kanał nie jest dostępny, spróbuj ponownego połączenia
				reconnectAttempts++

				if reconnectAttempts <= maxReconnectAttempts {
					// Logarytmiczne wydłużanie czasu między próbami
					backoffTime := time.Duration(math.Pow(2, float64(reconnectAttempts))) * time.Second
					if backoffTime > 30*time.Second {
						backoffTime = 30 * time.Second // Maksymalnie 30 sekund między próbami
					}

					// Emituj komunikat o próbie ponownego połączenia
					if b.ctx != nil {
						runtime.EventsEmit(b.ctx, EventSecurityMessage, fmt.Sprintf("Próba ponownego połączenia (%d/%d)...", reconnectAttempts, maxReconnectAttempts))
					}

					time.Sleep(backoffTime)
				} else {
					// Po przekroczeniu maksymalnej liczby prób, poczekaj dłużej przed kolejnymi próbami
					if b.ctx != nil {
						runtime.EventsEmit(b.ctx, EventNetworkError, "Nie można nawiązać stabilnego połączenia. Spróbuj ponownie połączyć się z pokojem.")
					}
					reconnectAttempts = 0 // Resetuj licznik, aby spróbować ponownie
					time.Sleep(10 * time.Second)
				}
			}

			select {
			case <-ctx.Done():
				return
			default:
				// Kontynuuj pętlę
			}
		}
	}()
}

// monitorNetworkStatus regularnie emituje aktualizacje statusu sieci
func (b *Bridge) monitorNetworkStatus(ctx context.Context) {
	if b.execp2p == nil || b.ctx == nil {
		return
	}

	// Śledź aktualnie połączonych użytkowników
	ticker := time.NewTicker(100 * time.Millisecond) // Jeszcze częstsze sprawdzanie dla maksymalnej responsywności
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := b.execp2p.GetNetworkStatus()
			runtime.EventsEmit(b.ctx, EventStatusUpdate, status)

			// Zawsze aktualizuj listę użytkowników
			connectedUsers := []map[string]interface{}{}

			// 1. Używamy domyślnego nicku (nie możemy pobrać z localStorage po stronie Go)
			localNickname := "Użytkownik"

			// 2. Zawsze dodaj lokalnego użytkownika do listy
			localUser := map[string]interface{}{
				"id":       status["peer_id"].(string),
				"nickname": localNickname,
				"isLocal":  true,
			}
			connectedUsers = append(connectedUsers, localUser)

			// 2. Dodaj zdalne połączenia
			if status["is_running"].(bool) && status["connected_peers"].(int) > 0 {
				if network := b.execp2p.GetNetworkAccess(); network != nil {
					peers := network.GetConnectedPeers()
					for _, peerID := range peers {
						// Dodaj zdalne ID do listy użytkowników
						remoteUser := map[string]interface{}{
							"id":       peerID,
							"nickname": "Użytkownik",
							"isLocal":  false,
						}
						connectedUsers = append(connectedUsers, remoteUser)
					}
				}
			}

			// 3. Zawsze emituj aktualną listę użytkowników
			runtime.EventsEmit(b.ctx, "users:update", connectedUsers)
		}
	}
}

// monitorSecurity monitoruje zdarzenia bezpieczeństwa
func (b *Bridge) monitorSecurity(ctx context.Context) {
	if b.execp2p == nil || b.ctx == nil {
		return
	}

	// Monitorowanie odcisków palca i zdarzeń bezpieczeństwa
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Sprawdź status e2e_encryption
			status := b.execp2p.GetNetworkStatus()
			if status["e2e_encryption"].(bool) && status["connected_peers"].(int) > 0 {
				// Emisja komunikatu o bezpiecznym połączeniu
				securityInfo := b.execp2p.GetSecuritySummary()
				if fingerprints, ok := securityInfo["peer_fingerprints"].(map[string]interface{}); ok && len(fingerprints) > 0 {
					runtime.EventsEmit(b.ctx, EventPeerFingerprints, fingerprints)
					b.EmitSecurityMessage("Kanał komunikacyjny zabezpieczony szyfrowaniem end-to-end.")
				}
			}
		}
	}
}

// EmitSecurityMessage wysyła komunikat bezpieczeństwa do frontendu
func (b *Bridge) EmitSecurityMessage(message string) {
	if b.ctx == nil {
		return
	}

	runtime.EventsEmit(b.ctx, EventSecurityMessage, message)
}

// EmitNetworkError wysyła błąd sieci do frontendu
func (b *Bridge) EmitNetworkError(err error) {
	if b.ctx == nil || err == nil {
		return
	}

	runtime.EventsEmit(b.ctx, EventNetworkError, err.Error())
}
