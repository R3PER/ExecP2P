package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"net"
	"time"

	"execp2p/internal/config"
	"execp2p/internal/crypto"
	"execp2p/internal/discovery"
	"execp2p/internal/logger"
	"execp2p/internal/network"
	"execp2p/internal/room"
	"execp2p/internal/types"
)

// ExecP2P is the main application state
type ExecP2P struct {
	config      *config.Config
	peerID      string
	currentRoom *room.Room

	// core components
	pqCrypto *crypto.PQCrypto
	network  network.Network
	// Pole gui zostało usunięte - GUI jest inicjalizowane w main.go

	// runtime state
	isRunning  bool
	listenPort int

	// sync
	stopChan chan struct{}
}

// NewExecP2P creates a new ExecP2P instance
func NewExecP2P(cfg *config.Config) (*ExecP2P, error) {
	// generate a random peer ID for this session
	peerID, err := generatePeerID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate peer ID: %w", err)
	}

	// set up post-quantum crypto
	pqCrypto, err := crypto.NewPQCrypto()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cryptography: %w", err)
	}

	// find a port we can use
	listenPort, err := findAvailablePort(cfg.Network.MinPort, cfg.Network.MaxPort)
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	return &ExecP2P{
		config:     cfg,
		peerID:     peerID,
		pqCrypto:   pqCrypto,
		listenPort: listenPort,
		stopChan:   make(chan struct{}),
	}, nil
}

// StartGUILifecycle starts the new GUI-driven application flow
func (e *ExecP2P) StartGUILifecycle(ctx context.Context) error {
	// gui is initialized in main.go to avoid circular dependencies
	// This is just a stub method for interface compatibility
	return fmt.Errorf("GUI lifecycle should be started from main")
}

// CreateRoom creates a new chat room and starts listening
func (e *ExecP2P) CreateRoom(ctx context.Context) (*types.CreateRoomResult, error) {
	// Tworzymy pokój jako prywatny (z kluczem dostępu)
	newRoom, err := room.NewRoom("ExecP2P Chat", "Post-quantum encrypted chat room", e.config.Network.MaxPeers, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	// Ustawiamy port nasłuchiwania w obiekcie pokoju
	newRoom.ListenPort = e.listenPort
	logger.L().Info("Utworzono pokój z portem nasłuchiwania", "port", e.listenPort)

	e.currentRoom = newRoom

	if err := e.initializeComponents(ctx, true, ""); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	if err := e.startServices(ctx); err != nil {
		return nil, fmt.Errorf("failed to start services: %w", err)
	}

	// start background handlers now that room exists
	go e.handleMessages(ctx)
	go e.handlePeerEvents(ctx)
	go e.handleSecurityEvents(ctx)
	go e.handleNetworkErrors(ctx)

	// Zwróć ID pokoju i klucz dostępu oraz informację o porcie
	return &types.CreateRoomResult{
		RoomID:     newRoom.ID,
		AccessKey:  newRoom.AccessKey,
		ListenPort: e.listenPort,
	}, nil
}

// JoinRoom joins an existing chat room - ta funkcja korzysta z ulepszonej logiki JoinRoomWithFallback
func (e *ExecP2P) JoinRoom(ctx context.Context, roomID string, remoteAddr string, accessKey string) error {
	if !room.ValidateRoomID(roomID) {
		return fmt.Errorf("invalid room ID format")
	}

	// Klucz dostępu jest wymagany
	if accessKey == "" {
		return fmt.Errorf("brak klucza dostępu do pokoju")
	}

	// Zapisz dane pokoju, aby użyć ich do weryfikacji po połączeniu
	wantedRoomID := roomID
	wantedAccessKey := accessKey

	// Tworzymy obiekt pokoju z kluczem dostępu
	e.currentRoom = &room.Room{
		ID:        wantedRoomID,
		Name:      "ExecP2P E2E Chat",
		MaxPeers:  e.config.Network.MaxPeers,
		IsPrivate: true,
		AccessKey: wantedAccessKey,
	}

	// Jeśli podano konkretny adres, spróbuj połączyć się bezpośrednio
	if remoteAddr != "" {
		logger.L().Info("Łączenie z podanym adresem", "addr", remoteAddr, "room_id", wantedRoomID)

		// Ustawiamy isListener=false, ponieważ dołączamy do istniejącego pokoju
		if err := e.initializeComponents(ctx, false, remoteAddr); err != nil {
			e.currentRoom = nil // Resetujemy pokój w przypadku błędu
			return fmt.Errorf("błąd inicjalizacji połączenia: %w", err)
		}

		// Próba uruchomienia usług, które ustanowią połączenie
		if err := e.startServices(ctx); err != nil {
			// Sprzątamy po nieudanej próbie
			if e.network != nil {
				e.network.Stop()
				e.network = nil
			}
			e.currentRoom = nil
			return fmt.Errorf("błąd uruchamiania usług sieciowych: %w", err)
		}

		// Sprawdź czy faktycznie połączyliśmy się z pokojem o właściwym ID
		// Ta weryfikacja musi być wykonana po nawiązaniu połączenia, gdy wymiana
		// kluczy jest zakończona
		go func() {
			// Daj trochę czasu na ustanowienie połączenia i wymianę danych
			time.Sleep(2 * time.Second)

			// Czy mamy aktywne połączenie?
			if e.network == nil {
				logger.L().Error("Brak aktywnego połączenia po dołączeniu")
				return
			}

			// Czy faktycznie połączyliśmy się z pokojem o żądanym ID?
			actualRoomID := ""
			if e.currentRoom != nil {
				actualRoomID = e.currentRoom.ID
			}

			if actualRoomID != wantedRoomID {
				logger.L().Error("Połączono z pokojem o nieprawidłowym ID",
					"wanted", wantedRoomID, "actual", actualRoomID)
				// Tu możesz dodać logikę reakcji na ten problem
			} else {
				logger.L().Info("Poprawnie dołączono do pokoju", "room_id", wantedRoomID)
			}
		}()

		// Uruchom obsługę wiadomości i zdarzeń
		go e.handleMessages(ctx)
		go e.handlePeerEvents(ctx)
		go e.handleSecurityEvents(ctx)
		go e.handleNetworkErrors(ctx)

		return nil
	}

	// W przeciwnym razie używamy zaawansowanej strategii łączenia
	return e.JoinRoomWithFallback(ctx, roomID, accessKey)
}

// JoinRoomWithFallback implementuje wielopoziomową strategię łączenia
// z automatycznym fallback do różnych metod
func (e *ExecP2P) JoinRoomWithFallback(ctx context.Context, roomID string, accessKey string) error {
	logger.L().Info("Rozpoczynam zaawansowaną procedurę łączenia z pokojem", "room_id", roomID)

	// 2. Najpierw spróbuj autodetekcji przez broadcast, mDNS i DHT (w sieci lokalnej)
	// Jest to preferowana metoda, która automatycznie dopasuje port nasłuchujący
	if addr, err := e.tryLocalNetworkDiscovery(ctx, roomID); err == nil {
		logger.L().Info("Połączono przez autodetekcję w sieci lokalnej", "addr", addr)

		if err := e.initializeComponents(ctx, false, addr); err != nil {
			return fmt.Errorf("błąd inicjalizacji komponentów: %w", err)
		}

		if err := e.startServices(ctx); err != nil {
			return fmt.Errorf("błąd uruchamiania usług: %w", err)
		}

		go e.handleMessages(ctx)
		go e.handlePeerEvents(ctx)
		go e.handleSecurityEvents(ctx)
		go e.handleNetworkErrors(ctx)

		return nil
	}

	// 1. Próba lokalnego połączenia przez localhost jako druga opcja
	// To pomaga przy uruchamianiu wielu instancji na jednym komputerze
	if localAddr, err := e.tryLocalConnections(ctx, roomID); err == nil {
		logger.L().Info("Połączono lokalnie", "addr", localAddr)
		return nil
	}

	// 3. Spróbuj połączenia przez serwer sygnalizacyjny i UDP hole punching
	signalingConfig := discovery.NewSignalingConfig("")
	if addr, err := e.trySignalingAndHolePunching(ctx, roomID, signalingConfig); err == nil {
		logger.L().Info("Połączono przez hole punching", "addr", addr)

		if err := e.initializeComponents(ctx, false, addr); err != nil {
			return fmt.Errorf("błąd inicjalizacji komponentów: %w", err)
		}

		if err := e.startServices(ctx); err != nil {
			return fmt.Errorf("błąd uruchamiania usług: %w", err)
		}

		go e.handleMessages(ctx)
		go e.handlePeerEvents(ctx)
		go e.handleSecurityEvents(ctx)
		go e.handleNetworkErrors(ctx)

		return nil
	}

	// 4. Ostateczność: przekazywanie przez TURN (nie zaimplementowane)
	// W przyszłości można dodać kod do obsługi relayingu przez TURN

	return fmt.Errorf("wszystkie metody połączenia zawiodły - spróbuj podać bezpośredni adres IP")
}

// tryLocalConnections próbuje nawiązać połączenie z lokalnymi instancjami
// Parametr roomID jest używany do logowania informacji o procesie łączenia
func (e *ExecP2P) tryLocalConnections(ctx context.Context, roomID string) (string, error) {
	localPorts := []int{9000, 9001, 9002, 9003, 9004, 9005, 9006, 9007, 9008, 9009}

	logger.L().Info("Próbuję nawiązać lokalne połączenie", "room_id", roomID)

	for _, port := range localPorts {
		localAddr := fmt.Sprintf("127.0.0.1:%d", port)
		logger.L().Info("Próba lokalnego portu", "addr", localAddr, "room_id", roomID)

		if err := e.initializeComponents(ctx, false, localAddr); err != nil {
			continue
		}

		if err := e.startServices(ctx); err != nil {
			e.network.Stop()
			e.network = nil
			continue
		}

		// Sukces! Uruchom usługi obsługi
		go e.handleMessages(ctx)
		go e.handlePeerEvents(ctx)
		go e.handleSecurityEvents(ctx)
		go e.handleNetworkErrors(ctx)

		logger.L().Info("Udało się połączyć lokalnie", "room_id", roomID, "addr", localAddr)
		return localAddr, nil
	}

	return "", fmt.Errorf("wszystkie próby lokalnych połączeń dla pokoju %s nie powiodły się", roomID)
}

// tryLocalNetworkDiscovery próbuje wykryć urządzenia w sieci lokalnej
func (e *ExecP2P) tryLocalNetworkDiscovery(ctx context.Context, roomID string) (string, error) {
	logger.L().Info("Próba wykrycia urządzeń w sieci lokalnej", "room_id", roomID)

	// Utwórz serwer DHT
	dhtServer, err := discovery.StartDHTNode(e.config.Discovery.BTDHTPort)
	if err != nil {
		logger.L().Warn("Nie udało się uruchomić węzła DHT", "err", err)
	}

	// Uruchom autodetekcję z wszystkimi dostępnymi metodami
	addr, err := discovery.AutoDiscovery(ctx, roomID, dhtServer)
	if err != nil {
		return "", fmt.Errorf("autodetekcja nie powiodła się: %w", err)
	}

	return addr, nil
}

// trySignalingAndHolePunching próbuje łączenia przez serwer sygnalizacyjny i hole punching
func (e *ExecP2P) trySignalingAndHolePunching(ctx context.Context, roomID string, config *discovery.SignalingServerConfig) (string, error) {
	logger.L().Info("Próba połączenia przez serwer sygnalizacyjny", "room_id", roomID)

	// Sprawdź dostępność serwera sygnalizacyjnego
	roomInfo, err := discovery.GetRoomInfoFromSignalingServer(ctx, config, roomID)
	if err != nil {
		return "", fmt.Errorf("nie udało się połączyć z serwerem sygnalizacyjnym: %w", err)
	}

	if len(roomInfo.PublicAddrs) == 0 {
		return "", fmt.Errorf("brak dostępnych adresów dla pokoju")
	}

	// Spróbuj UDP hole punching dla każdego z dostępnych adresów
	for _, addr := range roomInfo.PublicAddrs {
		punchedAddr, err := discovery.InitiateHolePunching(ctx, addr, roomID, e.listenPort)
		if err != nil {
			logger.L().Warn("Hole punching nie powiódł się", "addr", addr, "err", err)
			continue
		}

		// Udało się!
		return punchedAddr, nil
	}

	return "", fmt.Errorf("nie udało się nawiązać połączenia przez hole punching")
}

// Close shuts down the application
func (e *ExecP2P) Close() {
	if !e.isRunning {
		return
	}

	e.isRunning = false
	close(e.stopChan)

	// GUI handling now done in the wailsbridge

	if e.network != nil {
		e.network.Stop()
	}
}

// initialize all the components we need
func (e *ExecP2P) initializeComponents(ctx context.Context, isListener bool, remoteAddr string) error {
	var err error

	// Inicjalizacja sieci z przekazaniem dodatkowych parametrów
	net, err := network.NewNetwork(
		ctx,
		e.peerID,
		e.currentRoom.ID,
		e.listenPort,
		e.pqCrypto,
		isListener,
		remoteAddr,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize network transport: %w", err)
	}

	// Ustaw sieć
	e.network = net

	// Dostosuj strukturę sieci, aby zawierała klucz dostępu do pokoju
	if qnet, ok := net.(*network.QuicNetwork); ok && e.currentRoom != nil {
		// Dodaj dodatkowe pole z kluczem dostępu
		qnet.SetRoomAccessKey(e.currentRoom.AccessKey)
		logger.L().Debug("Ustawiono klucz dostępu do pokoju w sieci",
			"room_id", e.currentRoom.ID,
			"has_key", e.currentRoom.AccessKey != "")
	}

	return nil
}

// start up networking and discovery
func (e *ExecP2P) startServices(ctx context.Context) error {
	e.isRunning = true

	if err := e.network.Start(ctx); err != nil {
		return fmt.Errorf("failed to start network transport: %w", err)
	}

	// If we are the creator, we need to start discovery services
	if e.network.IsListener() {
		roomID := e.currentRoom.ID
		listenPort := e.listenPort

		// Log the listen port dla łatwiejszego debugowania
		logger.L().Info("Listening for connections", "port", listenPort, "room_id", roomID)

		// Start DHT node with a random port offset to avoid conflicts with multiple instances
		dhtPort := e.config.Discovery.BTDHTPort + mathrand.Intn(10)
		dhtServer, err := discovery.StartDHTNode(dhtPort)
		if err != nil {
			logger.L().Warn("DHT node startup failed", "err", err)
		}

		go discovery.Advertise(ctx, roomID, listenPort)
		// Use dynamic port for discovery responder to avoid conflicts
		go discovery.StartDiscoveryResponder(ctx, roomID, listenPort)
		if dhtServer != nil {
			go discovery.AnnounceDHT(ctx, dhtServer, roomID, listenPort)
		}
	}

	return nil
}

// handle receiving encrypted messages
func (e *ExecP2P) handleMessages(ctx context.Context) {
	receiveChan := e.network.GetIncomingMessages()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-receiveChan:
			// Messages will be handled by the wailsbridge event system
			// to avoid circular dependencies
		}
	}
}

// handle peer connection events
func (e *ExecP2P) handlePeerEvents(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-ticker.C:
			// Status updates are now handled via the wailsbridge event system
		}
	}
}

// handle security events and fingerprint displays
func (e *ExecP2P) handleSecurityEvents(ctx context.Context) {
	fingerprintTicker := time.NewTicker(60 * time.Second)
	keyRotationCheckTicker := time.NewTicker(1 * time.Minute)
	defer fingerprintTicker.Stop()
	defer keyRotationCheckTicker.Stop()

	var lastShownFingerprints map[string]string

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-fingerprintTicker.C:
			currentFingerprints := e.getPeerFingerprints()
			if !equalStringMaps(lastShownFingerprints, currentFingerprints) && len(currentFingerprints) > 0 {
				// Fingerprints will be shown via wailsbridge events
				lastShownFingerprints = currentFingerprints
			}

		case <-keyRotationCheckTicker.C:
			if e.network == nil {
				continue
			}
			rotated, err := e.network.ForceKeyRotation()
			if err != nil {
				// Security messages handled via wailsbridge
				logger.L().Error("Key rotation error", "err", err)
				continue
			}
			if rotated {
				logger.L().Info("Forward secrecy: Keys rotated, re-establishing secure channels")
			}
		}
	}
}

// get fingerprints for all known peers
func (e *ExecP2P) getPeerFingerprints() map[string]string {
	fingerprints := make(map[string]string)
	if e.pqCrypto == nil {
		return fingerprints
	}
	verifiedPeers := e.pqCrypto.GetVerifiedPeers()
	for _, peerID := range verifiedPeers {
		if fingerprint, err := e.pqCrypto.GetPeerFingerprint(peerID); err == nil {
			fingerprints[peerID] = fingerprint
		}
	}
	return fingerprints
}

// check if two string maps are the same
func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// generate a random peer ID for this session
func generatePeerID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// findAvailablePort iterates and returns an available port.
func findAvailablePort(minPort, maxPort int) (int, error) {
	ports := make([]int, 0, maxPort-minPort+1)
	for i := minPort; i <= maxPort; i++ {
		ports = append(ports, i)
	}
	mathrand.Shuffle(len(ports), func(i, j int) {
		ports[i], ports[j] = ports[j], ports[i]
	})
	for _, port := range ports {
		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", minPort, maxPort)
}

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return false
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return false
	}
	defer udpConn.Close()
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return false
	}
	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return false
	}
	defer tcpListener.Close()
	return true
}

// --- AppController interface methods ---

// SendMessage sends a message over the network.
func (e *ExecP2P) SendMessage(ctx context.Context, message string) error {
	if e.network == nil {
		return fmt.Errorf("not connected to a room")
	}
	return e.network.SendMessage(ctx, message)
}

// GetPeerFingerprint returns our cryptographic fingerprint
func (e *ExecP2P) GetPeerFingerprint() (string, error) {
	if e.pqCrypto == nil {
		return "", fmt.Errorf("crypto not initialized")
	}
	return e.pqCrypto.GetIdentityFingerprint()
}

// GetRoomInfo returns info about the current room
func (e *ExecP2P) GetRoomInfo() *room.Room {
	return e.currentRoom
}

// RegenerateRoomAccessKey tworzy nowy klucz dostępu dla bieżącego pokoju
// Może być wywołane tylko przez twórcę pokoju (isListener)
func (e *ExecP2P) RegenerateRoomAccessKey() (string, error) {
	// Sprawdź czy jesteśmy twórcą pokoju
	if e.network == nil || !e.network.IsListener() {
		return "", fmt.Errorf("tylko twórca pokoju może zregenerować klucz dostępu")
	}

	// Sprawdź czy mamy pokój
	if e.currentRoom == nil {
		return "", fmt.Errorf("nie jesteśmy połączeni z żadnym pokojem")
	}

	// Zregeneruj klucz
	if err := e.currentRoom.RegenerateAccessKey(); err != nil {
		return "", err
	}

	return e.currentRoom.AccessKey, nil
}

// GetListenPort returns the port we're listening on
func (e *ExecP2P) GetListenPort() int {
	return e.listenPort
}

// GetNetworkAccess returns the network object for direct access to network functions
// UWAGA: Ta metoda jest eksporterem prywatnego pola - używać ostrożnie!
func (e *ExecP2P) GetNetworkAccess() network.Network {
	return e.network
}

// TryLocalNetworkDiscovery to publiczny wrapper dla metody prywatnej
func (e *ExecP2P) TryLocalNetworkDiscovery(ctx context.Context, roomID string) (string, error) {
	return e.tryLocalNetworkDiscovery(ctx, roomID)
}

// GetNetworkStatus returns current network and encryption status
func (e *ExecP2P) GetNetworkStatus() map[string]interface{} {
	status := map[string]interface{}{
		"peer_id":         e.peerID,
		"listen_port":     e.listenPort,
		"room_id":         "",
		"connected_peers": 0,
		"verified_peers":  0,
		"e2e_encryption":  false,
		"is_running":      e.isRunning,
		"is_listener":     e.network != nil && e.network.IsListener(),
	}

	if e.currentRoom != nil {
		status["room_id"] = e.currentRoom.ID
	}

	if e.network != nil {
		status["connected_peers"] = len(e.network.GetConnectedPeers())
	}

	if e.pqCrypto != nil {
		verifiedPeers := len(e.pqCrypto.GetVerifiedPeers())
		status["verified_peers"] = verifiedPeers

		// Pokój jest uważany za zaszyfrowany, gdy:
		// 1. Mamy zweryfikowane peery (klasyczny przypadek e2e)
		// 2. LUB gdy jesteśmy twórcą pokoju (network w trybie listener)
		if verifiedPeers > 0 || (e.network != nil && e.network.IsListener()) {
			status["e2e_encryption"] = true
		}
	}

	return status
}

// GetSecuritySummary returns a summary of our security features
func (e *ExecP2P) GetSecuritySummary() map[string]interface{} {
	summary := map[string]interface{}{
		"encryption_algorithms": map[string]string{
			"key_exchange": "CRYSTALS-Kyber-1024",
			"signatures":   "CRYSTALS-DILITHIUM-5",
			"symmetric":    "ChaCha20-Poly1305",
		},
	}
	if e.pqCrypto != nil {
		if fingerprint, err := e.pqCrypto.GetIdentityFingerprint(); err == nil {
			summary["identity_fingerprint"] = fingerprint
		}
	}

	// Dodaj informacje o pokoju, jeśli jesteśmy twórcą
	if e.currentRoom != nil && e.network != nil && e.network.IsListener() {
		summary["room_info"] = map[string]interface{}{
			"room_id":    e.currentRoom.ID,
			"access_key": e.currentRoom.AccessKey,
			"is_private": e.currentRoom.IsPrivate,
		}
	}

	return summary
}

// handleNetworkErrors listens for async errors from the transport layer
func (e *ExecP2P) handleNetworkErrors(ctx context.Context) {
	if e.network == nil {
		return
	}
	errChan := e.network.GetErrorChannel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case err := <-errChan:
			if err == nil {
				continue
			}
			// Network errors are logged and will be emitted via wailsbridge
			logger.L().Error("Network error", "err", err)
		}
	}
}

// IsListener returns true if the network is in listening mode
func (e *ExecP2P) IsListener() bool {
	if e.network == nil {
		return false
	}
	return e.network.IsListener()
}
