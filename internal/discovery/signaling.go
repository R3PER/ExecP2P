package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"execp2p/internal/logger"
)

// Domyślny serwer sygnalizacyjny
// W rzeczywistym wdrożeniu, zmień ten adres na swój serwer sygnalizacyjny
// Możesz użyć "https://execp2p-signaling.example.com" (przykładowy)
// lub "http://localhost:8085" (dla lokalnego testowania)
// Pozostawienie jako puste ("") wyłącza funkcję serwera sygnalizacyjnego
const DefaultSignalingServer = ""

// RoomRegistration zawiera dane do rejestracji pokoju na serwerze sygnalizacyjnym
type RoomRegistration struct {
	RoomID         string `json:"room_id"`         // Identyfikator pokoju
	PublicAddr     string `json:"public_addr"`     // Publiczny adres IP:port
	IsNATed        bool   `json:"is_nated"`        // Czy jesteśmy za NATem
	STUNAddr       string `json:"stun_addr"`       // Adres uzyskany przez STUN
	BehindSymNAT   bool   `json:"behind_sym_nat"`  // Czy jesteśmy za symetrycznym NATem
	CreationTime   int64  `json:"creation_time"`   // Czas utworzenia pokoju
	ExpirationTime int64  `json:"expiration_time"` // Czas wygaśnięcia rejestracji
}

// RoomInfo zawiera informacje o pokoju pobrane z serwera sygnalizacyjnego
type RoomInfo struct {
	RoomID       string   `json:"room_id"`        // Identyfikator pokoju
	PublicAddrs  []string `json:"public_addrs"`   // Lista publicznych adresów
	LastSeen     int64    `json:"last_seen"`      // Kiedy ostatnio widziany
	BehindSymNAT bool     `json:"behind_sym_nat"` // Czy za symetrycznym NATem
}

// SignalingServerConfig przechowuje konfigurację serwera sygnalizacyjnego
type SignalingServerConfig struct {
	ServerURL      string        // URL serwera sygnalizacyjnego
	RequestTimeout time.Duration // Timeout dla żądań HTTP
}

// NewSignalingConfig tworzy nową konfigurację serwera sygnalizacyjnego
func NewSignalingConfig(serverURL string) *SignalingServerConfig {
	if serverURL == "" {
		serverURL = DefaultSignalingServer
	}
	return &SignalingServerConfig{
		ServerURL:      serverURL,
		RequestTimeout: 10 * time.Second,
	}
}

// RegisterRoomOnSignalingServer rejestruje pokój na serwerze sygnalizacyjnym
func RegisterRoomOnSignalingServer(ctx context.Context, config *SignalingServerConfig, roomID, publicAddr string) error {
	logger.L().Info("Rejestracja pokoju na serwerze sygnalizacyjnym", "room_id", roomID, "addr", publicAddr)

	// Pobierz adres przez STUN (może być inny niż podany publicAddr)
	stunAddr, err := ExternalUDPAddr(9000)
	if err != nil {
		logger.L().Warn("Nie udało się uzyskać adresu STUN", "err", err)
		stunAddr = publicAddr // Użyj podanego adresu jako fallback
	}

	// Przygotuj dane do rejestracji
	reg := RoomRegistration{
		RoomID:         roomID,
		PublicAddr:     publicAddr,
		IsNATed:        true, // Domyślnie zakładamy, że jesteśmy za NATem
		STUNAddr:       stunAddr,
		BehindSymNAT:   false, // Domyślnie zakładamy, że NAT nie jest symetryczny
		CreationTime:   time.Now().Unix(),
		ExpirationTime: time.Now().Add(8 * time.Hour).Unix(), // Rejestracja na 8 godzin
	}

	// Serializuj do JSON
	regJSON, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("błąd serializacji danych rejestracji: %w", err)
	}

	// Utwórz żądanie HTTP
	reqURL := fmt.Sprintf("%s/api/register", config.ServerURL)
	httpCtx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "POST", reqURL, bytes.NewBuffer(regJSON))
	if err != nil {
		return fmt.Errorf("błąd tworzenia żądania HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Wyślij żądanie
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// W przypadku błędu, zaloguj ale nie zwracaj - funkcjonalność jest opcjonalna
		logger.L().Warn("Nie udało się połączyć z serwerem sygnalizacyjnym", "err", err)
		return nil
	}
	defer resp.Body.Close()

	// Sprawdź odpowiedź
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.L().Warn("Serwer sygnalizacyjny zwrócił błąd", "status", resp.StatusCode, "body", string(body))
		return nil
	}

	logger.L().Info("Pomyślnie zarejestrowano pokój na serwerze sygnalizacyjnym", "room_id", roomID)
	return nil
}

// GetRoomInfoFromSignalingServer pobiera informacje o pokoju z serwera sygnalizacyjnego
func GetRoomInfoFromSignalingServer(ctx context.Context, config *SignalingServerConfig, roomID string) (*RoomInfo, error) {
	logger.L().Info("Pobieranie informacji o pokoju z serwera sygnalizacyjnego", "room_id", roomID)

	// Utwórz żądanie HTTP
	reqURL := fmt.Sprintf("%s/api/room/%s", config.ServerURL, roomID)
	httpCtx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("błąd tworzenia żądania HTTP: %w", err)
	}

	// Wyślij żądanie
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nie udało się połączyć z serwerem sygnalizacyjnym: %w", err)
	}
	defer resp.Body.Close()

	// Sprawdź odpowiedź
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pokój %s nie został znaleziony", roomID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("serwer zwrócił błąd: %d - %s", resp.StatusCode, string(body))
	}

	// Parsuj odpowiedź
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("błąd odczytu odpowiedzi: %w", err)
	}

	var roomInfo RoomInfo
	if err := json.Unmarshal(body, &roomInfo); err != nil {
		return nil, fmt.Errorf("błąd parsowania odpowiedzi JSON: %w", err)
	}

	return &roomInfo, nil
}

// AnnounceExternalAddress rejestruje nasz zewnętrzny adres w DHT i na serwerze sygnalizacyjnym
func AnnounceExternalAddress(ctx context.Context, config *SignalingServerConfig, roomID string, port int) {
	logger.L().Info("Ogłaszanie zewnętrznego adresu", "room_id", roomID, "port", port)

	// Najpierw spróbuj uzyskać zewnętrzny adres IP
	externalIP, err := GetExternalIP()
	if err != nil {
		logger.L().Warn("Nie udało się uzyskać zewnętrznego IP", "err", err)
		return
	}

	// Uzyskaj pełny zewnętrzny adres (IP:port) przez STUN
	externalAddr, err := ExternalUDPAddr(port)
	if err != nil {
		logger.L().Warn("Nie udało się uzyskać zewnętrznego adresu przez STUN", "err", err)
		// Użyj zwykłego IP z portem jako fallback
		externalAddr = fmt.Sprintf("%s:%d", externalIP, port)
	}

	// Zarejestruj na serwerze sygnalizacyjnym (jeśli podano konfigurację)
	if config != nil {
		go func() {
			err := RegisterRoomOnSignalingServer(ctx, config, roomID, externalAddr)
			if err != nil {
				logger.L().Warn("Nie udało się zarejestrować na serwerze sygnalizacyjnym", "err", err)
			}
		}()
	}
}

// ConnectWithSignalingServer próbuje nawiązać połączenie przez serwer sygnalizacyjny
func ConnectWithSignalingServer(ctx context.Context, config *SignalingServerConfig, roomID string, localPort int) (string, error) {
	// Pobierz informacje o pokoju
	roomInfo, err := GetRoomInfoFromSignalingServer(ctx, config, roomID)
	if err != nil {
		return "", fmt.Errorf("nie udało się uzyskać informacji o pokoju: %w", err)
	}

	// Sprawdź czy lista adresów nie jest pusta
	if len(roomInfo.PublicAddrs) == 0 {
		return "", fmt.Errorf("brak dostępnych adresów dla pokoju")
	}

	logger.L().Info("Pobrano informacje z serwera sygnalizacyjnego", "addrs", roomInfo.PublicAddrs)

	// Spróbuj nawiązać połączenie z każdym z adresów
	var lastError error
	for _, addr := range roomInfo.PublicAddrs {
		// Próba UDP hole punching
		punchedAddr, err := InitiateHolePunching(ctx, addr, roomID, localPort)
		if err != nil {
			lastError = err
			logger.L().Warn("Hole punching nie powiódł się", "addr", addr, "err", err)
			continue
		}

		// Jeśli udało się nawiązać połączenie, zwróć adres
		return punchedAddr, nil
	}

	return "", fmt.Errorf("nie udało się nawiązać połączenia z żadnym z adresów: %v", lastError)
}
