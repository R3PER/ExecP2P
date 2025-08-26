package network

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"execp2p/internal/crypto"
	"execp2p/internal/logger"

	"crypto/sha256"

	"github.com/quic-go/quic-go"
)

// message is what we send over the QUIC stream
// payload is hex-encoded, serialized crypto structures
type message struct {
	Type      string `json:"type"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
	SenderID  string `json:"sender_id"`
	RoomID    string `json:"room_id"`    // Identyfikator pokoju
	AccessKey string `json:"access_key"` // Klucz dostępu (opcjonalny, tylko dla pierwszego połączenia)
}

// QuicNetwork is a transport that uses QUIC for reliable, secure, and multiplexed communication.
type QuicNetwork struct {
	localPeerID string
	roomID      string
	pqCrypto    *crypto.PQCrypto

	ctx    context.Context
	cancel context.CancelFunc

	isListener bool
	listenPort int
	remoteAddr string

	incomingMessages chan *crypto.MessagePayload

	// asynchronous error reporting
	errorChan chan error

	conn      quic.Connection
	connMutex sync.RWMutex

	peersMutex   sync.RWMutex
	connectedIDs []string

	// state tracking to prevent message spam
	announcementSent bool
	keyExchangeSent  map[string]bool
	keyExchangeMutex sync.RWMutex

	// certificate fingerprints
	localCertFingerprint string

	// klucz dostępu do pokoju (do weryfikacji przy dołączaniu)
	roomAccessKey string
}

// NewQuicNetwork creates the transport but doesn't start goroutines until Start
func NewQuicNetwork(ctx context.Context, peerID, roomID string, listenPort int, pq *crypto.PQCrypto, isListener bool, remoteAddr string) (*QuicNetwork, error) {
	netCtx, cancel := context.WithCancel(ctx)

	qn := &QuicNetwork{
		localPeerID:      peerID,
		roomID:           roomID,
		pqCrypto:         pq,
		ctx:              netCtx,
		cancel:           cancel,
		isListener:       isListener,
		listenPort:       listenPort,
		remoteAddr:       remoteAddr,
		incomingMessages: make(chan *crypto.MessagePayload, 100),
		errorChan:        make(chan error, 10),
		keyExchangeSent:  make(map[string]bool),
	}
	return qn, nil
}

// Start sets up the QUIC connection and launches the reader goroutine
func (qn *QuicNetwork) Start(ctx context.Context) error {
	if qn.isListener {
		return qn.listenQUIC()
	}
	return qn.dialQUIC()
}

// Stop closes the connection and cancels background work
func (qn *QuicNetwork) Stop() {
	qn.cancel()

	// Zabezpieczenie przed nagłym zamykaniem połączenia
	qn.connMutex.Lock()
	conn := qn.conn
	qn.conn = nil // Ustawienie na nil zapobiega nowym wysyłkom
	qn.connMutex.Unlock()

	// Daj czas na dokończenie bieżących operacji
	if conn != nil {
		// Krótkie opóźnienie, aby dać czas na zakończenie bieżących operacji
		time.Sleep(100 * time.Millisecond)
		conn.CloseWithError(0, "closing")
	}
}

// SendMessage encrypts and sends a chat message to the peer
func (qn *QuicNetwork) SendMessage(ctx context.Context, msg string) error {
	// Tworzymy identyfikator wiadomości
	messageID := fmt.Sprintf("%s-%d", qn.localPeerID, time.Now().UnixNano())

	// Sprawdź połączenie - powinno być weryfikowane zarówno dla twórcy jak i dla dołączającego
	qn.connMutex.RLock()
	conn := qn.conn
	qn.connMutex.RUnlock()

	// Sprawdź czy mamy połączonych użytkowników
	qn.peersMutex.RLock()
	connectedPeers := len(qn.connectedIDs)
	var peerID string
	if connectedPeers > 0 {
		peerID = qn.connectedIDs[0] // Zapisz ID peer'a do użycia później
	}
	qn.peersMutex.RUnlock()

	// Przypadek 1: Nie mamy aktywnego połączenia lub jesteśmy twórcą pokoju bez połączonych użytkowników
	// W tym przypadku tylko zapisujemy wiadomość lokalnie
	if conn == nil || (qn.isListener && connectedPeers == 0) {
		// Dodaj wiadomość do lokalnego kanału tylko w tych przypadkach
		localMessage := &crypto.MessagePayload{
			SenderID:  qn.localPeerID,
			Message:   msg,
			Timestamp: time.Now(),
			MessageID: messageID,
		}
		qn.incomingMessages <- localMessage

		// Jeśli nie ma połączenia, ale jesteśmy dołączającym użytkownikiem, zwróć błąd
		if !qn.isListener && conn == nil {
			return fmt.Errorf("connection not established")
		}

		// W przeciwnym razie zwróć sukces
		return nil
	}

	// Jeśli dotarliśmy tutaj, mamy aktywne połączenie i możemy wysłać wiadomość
	if peerID == "" {
		// Mamy połączenie, ale nie znamy ID peer'a - to nie powinno się zdarzyć
		return fmt.Errorf("no verified peer connected")
	}

	encMsg, err := qn.pqCrypto.EncryptMessageForPeer(msg, peerID, qn.localPeerID)
	if err != nil {
		return err
	}
	msgBytes, err := crypto.SerializeEncryptedMessage(encMsg)
	if err != nil {
		return err
	}

	wrapper := message{
		Type:      "message",
		Payload:   hex.EncodeToString(msgBytes),
		Timestamp: time.Now().Unix(),
		SenderID:  qn.localPeerID,
	}
	logger.L().Debug("Sending message", "peer", peerID[:8], "size", len(msgBytes))
	return qn.writeWrapper(wrapper)
}

func (qn *QuicNetwork) GetIncomingMessages() <-chan *crypto.MessagePayload {
	return qn.incomingMessages
}

func (qn *QuicNetwork) GetConnectedPeers() []string {
	qn.peersMutex.RLock()
	defer qn.peersMutex.RUnlock()
	return append([]string(nil), qn.connectedIDs...)
}

func (qn *QuicNetwork) GetErrorChannel() <-chan error {
	return qn.errorChan
}

func (qn *QuicNetwork) sendError(err error) {
	select {
	case qn.errorChan <- err:
	default:
		// channel full; drop to avoid blocking inside critical paths
	}
}

func (qn *QuicNetwork) listenQUIC() error {
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to generate TLS config: %w", err)
	}

	// compute fingerprint of our first certificate
	if len(tlsConfig.Certificates) > 0 && len(tlsConfig.Certificates[0].Certificate) > 0 {
		fp := sha256.Sum256(tlsConfig.Certificates[0].Certificate[0])
		qn.localCertFingerprint = hex.EncodeToString(fp[:])
	}

	addr := fmt.Sprintf("0.0.0.0:%d", qn.listenPort)
	listener, err := quic.ListenAddr(addr, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	logger.L().Info("Listening on QUIC", "addr", addr)

	go qn.acceptLoop(listener)

	return nil
}

func (qn *QuicNetwork) acceptLoop(listener *quic.Listener) {
	defer listener.Close()
	// accept one connection for our 1-to-1 chat
	conn, err := listener.Accept(qn.ctx)
	if err != nil {
		logger.L().Error("Accept error", "err", err)
		qn.sendError(err)
		return
	}

	qn.connMutex.Lock()
	qn.conn = conn
	qn.connMutex.Unlock()
	logger.L().Info("Peer connected", "remote", conn.RemoteAddr().String())

	// joiner knows the remote address and can send announcement immediately
	// listener should send announcement after getting a connection
	if err := qn.sendPeerAnnouncement(); err != nil {
		logger.L().Error("Peer announcement send failed", "err", err)
	}

	qn.readLoop(conn)
}

func (qn *QuicNetwork) dialQUIC() error {
	if qn.remoteAddr == "" {
		return fmt.Errorf("remote address required for joiner")
	}

	tlsCfg, err := generateTLSConfig()
	if err != nil {
		return err
	}
	tlsCfg.InsecureSkipVerify = true // still skip PKI validation

	if len(tlsCfg.Certificates) > 0 && len(tlsCfg.Certificates[0].Certificate) > 0 {
		fp := sha256.Sum256(tlsCfg.Certificates[0].Certificate[0])
		qn.localCertFingerprint = hex.EncodeToString(fp[:])
	}

	conn, err := quic.DialAddr(qn.ctx, qn.remoteAddr, tlsCfg, nil)
	if err != nil {
		qn.sendError(err)
		return fmt.Errorf("failed to dial %s: %w", qn.remoteAddr, err)
	}

	qn.connMutex.Lock()
	qn.conn = conn
	qn.connMutex.Unlock()

	logger.L().Info("Dialed peer", "remote", conn.RemoteAddr().String())

	// joiner knows the remote address and can send announcement immediately
	if err := qn.sendPeerAnnouncement(); err != nil {
		return err
	}

	go qn.readLoop(conn)

	return nil
}

func (qn *QuicNetwork) readLoop(conn quic.Connection) {
	for {
		stream, err := conn.AcceptStream(qn.ctx)
		if err != nil {
			// Kontekst został zamknięty lub połączenie zostało przerwane
			logger.L().Debug("Connection stream error", "err", err)

			// Jeśli to nie jest błąd przerwania kontekstu, zgłoś błąd
			if qn.ctx.Err() == nil {
				qn.sendError(fmt.Errorf("błąd strumienia połączenia: %w", err))
			}

			// Bezpiecznie zakończ połączenie
			go qn.Stop() // Uruchom w goroutine, aby uniknąć zakleszczenia
			return
		}

		// Obsługa strumienia w osobnej goroutine
		go func(s quic.Stream) {
			defer func() {
				// Obsługa paniki w handleStream, aby nie zakończyć głównej pętli readLoop
				if r := recover(); r != nil {
					logger.L().Error("Panika w obsłudze strumienia", "recover", r)
				}
			}()
			qn.handleStream(s)
		}(stream)
	}
}

func (qn *QuicNetwork) handleStream(stream quic.Stream) {
	defer stream.Close()
	decoder := json.NewDecoder(stream)
	var wrapper message
	if err := decoder.Decode(&wrapper); err != nil {
		logger.L().Warn("Invalid message", "err", err)
		return
	}
	logger.L().Debug("Received wrapper", "type", wrapper.Type, "from", wrapper.SenderID[:8], "size", len(wrapper.Payload))
	qn.handleWrapper(wrapper)
}

func (qn *QuicNetwork) writeWrapper(w message) error {
	qn.connMutex.RLock()
	conn := qn.conn
	qn.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection closed")
	}

	stream, err := conn.OpenStreamSync(qn.ctx)
	if err != nil {
		qn.sendError(err)
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	encoder := json.NewEncoder(stream)
	return encoder.Encode(w)
}

func (qn *QuicNetwork) handleWrapper(w message) {
	switch w.Type {
	case "announcement":
		qn.handlePeerAnnouncement(w)
	case "keyexchange":
		qn.handleKeyExchange(w)
	case "message":
		qn.handleEncryptedChat(w)
	}
}

func (qn *QuicNetwork) handlePeerAnnouncement(w message) {
	bytesPayload, err := hex.DecodeString(w.Payload)
	if err != nil {
		logger.L().Warn("Błąd dekodowania payload ogłoszenia", "err", err)
		return
	}
	announcement, err := crypto.DeserializePeerAnnouncement(bytesPayload)
	if err != nil {
		logger.L().Warn("Błąd deserializacji ogłoszenia", "err", err)
		return
	}

	// Sprawdź czy ID pokoju i klucz dostępu są zgodne
	if w.RoomID != "" && w.RoomID != qn.roomID {
		logger.L().Info("Dopasowanie ID pokoju podczas ogłoszenia peer",
			"my_id", qn.roomID, "received", w.RoomID)

		// Jeśli otrzymujemy ogłoszenie od pokoju, którego szukamy, dostosujmy nasze ID
		// Ten przypadek występuje, gdy dołączamy przez wyszukiwanie
		if !qn.isListener {
			logger.L().Info("Aktualizuję ID pokoju jako dołączający",
				"old_id", qn.roomID, "new_id", w.RoomID)
			qn.roomID = w.RoomID
		} else {
			// Jako słuchacz (host) trzymamy się naszego ID
			logger.L().Warn("Odrzucenie ogłoszenia peer z nieprawidłowym ID pokoju",
				"expected", qn.roomID, "got", w.RoomID)

			// Zamiast natychmiast wysyłać błąd, który może przerwać połączenie,
			// utrzymaj połączenie, ale ignoruj wiadomości
			go func() {
				// Oczekujemy chwilę, aby klient miał czas odebrać potwierdzenie
				time.Sleep(500 * time.Millisecond)
				qn.sendError(fmt.Errorf("niezgodne ID pokoju: %s", w.RoomID))
			}()
			return
		}
	}

	// Jeśli mamy klucz dostępu, sprawdź czy jest zgodny
	qn.keyExchangeMutex.RLock()
	roomAccessKey := qn.roomAccessKey
	qn.keyExchangeMutex.RUnlock()

	if roomAccessKey != "" && w.AccessKey != roomAccessKey {
		logger.L().Warn("Odrzucenie ogłoszenia peer z nieprawidłowym kluczem dostępu",
			"room_id", qn.roomID, "peer", announcement.PeerID[:8])

		// Tak samo jak powyżej, opóźnij wysłanie błędu
		go func() {
			time.Sleep(500 * time.Millisecond)
			qn.sendError(fmt.Errorf("nieprawidłowy klucz dostępu"))
		}()
		return
	}

	if err := qn.pqCrypto.ProcessPeerAnnouncement(announcement); err != nil {
		logger.L().Warn("Invalid peer announcement", "err", err)
		return
	}

	logger.L().Info("Peer announcement accepted",
		"room_id", qn.roomID,
		"peer", announcement.PeerID[:8],
		"access_key_ok", roomAccessKey == "" || w.AccessKey == roomAccessKey)

	qn.peersMutex.Lock()
	qn.connectedIDs = []string{announcement.PeerID}
	qn.peersMutex.Unlock()

	if !qn.announcementSent {
		if err := qn.sendPeerAnnouncement(); err == nil {
			qn.announcementSent = true
		}
	}

	qn.keyExchangeMutex.Lock()
	alreadySent := qn.keyExchangeSent[announcement.PeerID]
	if !alreadySent {
		qn.keyExchangeSent[announcement.PeerID] = true
	}
	qn.keyExchangeMutex.Unlock()

	if !alreadySent {
		if err := qn.sendKeyExchange(announcement.PeerID); err != nil {
			logger.L().Error("Key exchange failed", "err", err)
			qn.keyExchangeMutex.Lock()
			qn.keyExchangeSent[announcement.PeerID] = false
			qn.keyExchangeMutex.Unlock()
		}
	}

	// verify remote certificate hash matches announced fingerprint
	tlsState := qn.conn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) > 0 {
		hash := sha256.Sum256(tlsState.PeerCertificates[0].Raw)
		remoteFp := hex.EncodeToString(hash[:])
		if remoteFp != announcement.TLSCertFingerprint {
			logger.L().Warn("TLS certificate fingerprint mismatch; possible MITM")
			qn.sendError(fmt.Errorf("tls fingerprint mismatch"))
			return
		}
	}
}

func (qn *QuicNetwork) handleKeyExchange(w message) {
	bytesPayload, err := hex.DecodeString(w.Payload)
	if err != nil {
		return
	}
	keyEx, err := crypto.DeserializeKeyExchange(bytesPayload)
	if err != nil {
		return
	}
	if err := qn.pqCrypto.ProcessKeyExchange(keyEx); err != nil {
		logger.L().Warn("Invalid key exchange", "err", err)
		return
	}
	logger.L().Info("Secure channel established", "peer", keyEx.SenderID[:8])
}

func (qn *QuicNetwork) handleEncryptedChat(w message) {
	bytesPayload, err := hex.DecodeString(w.Payload)
	if err != nil {
		logger.L().Warn("Message decode error", "err", err)
		return
	}
	encMsg, err := crypto.DeserializeEncryptedMessage(bytesPayload)
	if err != nil {
		logger.L().Warn("Message deserialization error", "err", err)
		return
	}
	payload, err := qn.pqCrypto.DecryptMessageFromPeer(encMsg)
	if err != nil {
		logger.L().Warn("Message decryption error", "err", err)
		return
	}

	// Sprawdź czy to wiadomość od nas (lokalnego użytkownika) i czy jesteśmy twórcą pokoju
	// Jeśli tak, nie przekazuj jej do kanału wiadomości przychodzących, ponieważ
	// już dodaliśmy ją lokalnie w funkcji SendMessage
	if qn.isListener && payload.SenderID == qn.localPeerID {
		// To jest wiadomość od lokalnego użytkownika będącego twórcą pokoju
		// Nie przekazujemy jej dalej, ponieważ została już dodana lokalnie
		return
	}

	// W przeciwnym razie przekaż wiadomość do kanału
	select {
	case qn.incomingMessages <- payload:
	default:
		logger.L().Warn("Incoming message channel full; dropping")
	}
}

func (qn *QuicNetwork) sendPeerAnnouncement() error {
	announcement, err := qn.pqCrypto.CreatePeerAnnouncement(qn.localPeerID, qn.localCertFingerprint)
	if err != nil {
		return err
	}
	bytesPayload, err := crypto.SerializePeerAnnouncement(announcement)
	if err != nil {
		return err
	}

	// Pobierz informacje o kluczu dostępu do pokoju
	var accessKey string
	if qn.roomID != "" {
		// Użyj klucza dostępu z naszej struktury
		qn.keyExchangeMutex.RLock()
		accessKey = qn.roomAccessKey
		qn.keyExchangeMutex.RUnlock()
		logger.L().Debug("Dodanie klucza dostępu do ogłoszenia",
			"room_id", qn.roomID,
			"has_key", accessKey != "")
	}

	wrapper := message{
		Type:      "announcement",
		Payload:   hex.EncodeToString(bytesPayload),
		Timestamp: time.Now().Unix(),
		SenderID:  qn.localPeerID,
		RoomID:    qn.roomID, // Dodaj ID pokoju do ogłoszenia
		AccessKey: accessKey, // Dodaj klucz dostępu (jeśli dostępny)
	}

	logger.L().Debug("Wysyłanie ogłoszenia peer", "room_id", qn.roomID)

	err = qn.writeWrapper(wrapper)
	if err == nil {
		qn.announcementSent = true
	}
	return err
}

func (qn *QuicNetwork) sendKeyExchange(peerID string) error {
	keyEx, err := qn.pqCrypto.InitiateKeyExchange(peerID, qn.localPeerID)
	if err != nil {
		return err
	}
	bytesPayload, err := crypto.SerializeKeyExchange(keyEx)
	if err != nil {
		return err
	}
	wrapper := message{
		Type:      "keyexchange",
		Payload:   hex.EncodeToString(bytesPayload),
		Timestamp: time.Now().Unix(),
		SenderID:  qn.localPeerID,
	}
	return qn.writeWrapper(wrapper)
}

func (qn *QuicNetwork) ForceKeyRotation() (bool, error) {
	rotated, err := qn.pqCrypto.RotateKeys()
	if err != nil || !rotated {
		return rotated, err
	}

	qn.peersMutex.RLock()
	peerIDs := append([]string(nil), qn.connectedIDs...)
	qn.peersMutex.RUnlock()

	var aggErr error

	qn.keyExchangeMutex.Lock()
	for _, pid := range peerIDs {
		qn.keyExchangeSent[pid] = false
	}
	qn.keyExchangeMutex.Unlock()

	for _, peerID := range peerIDs {
		if err := qn.sendKeyExchange(peerID); err != nil {
			if aggErr == nil {
				aggErr = err
			}
		}
	}

	if len(peerIDs) > 0 {
		logger.L().Info("Keys rotated", "peers", len(peerIDs))
	}

	return rotated, aggErr
}

// IsListener returns true if the network is a listener (creator)
func (qn *QuicNetwork) IsListener() bool {
	return qn.isListener
}

// SetRoomAccessKey ustawia klucz dostępu do pokoju, który będzie używany
// przy wysyłaniu ogłoszeń w celu autentykacji
func (qn *QuicNetwork) SetRoomAccessKey(accessKey string) {
	// Potrzebne pole nie istnieje, więc dodajmy je najpierw
	logger.L().Debug("Ustawienie klucza dostępu do pokoju", "room_id", qn.roomID)

	// Przy następnym wysyłaniu ogłoszenia, zostanie użyty ten klucz
	qn.keyExchangeMutex.Lock()
	qn.roomAccessKey = accessKey
	qn.keyExchangeMutex.Unlock()
}

// generateTLSConfig sets up a ephemeral, self-signed TLS config for the QUIC listener
func generateTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"ExecP2P"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{certDER}, PrivateKey: key}},
		NextProtos:   []string{"execp2p-chat"},
	}, nil
}
