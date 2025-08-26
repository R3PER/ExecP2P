package room

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcutil/base58"
)

const (
	// room ID length in characters
	RoomIDLength = 32

	// prefix for all ExecP2P room IDs
	RoomIDPrefix = "ExecP2P_"
)

// Room represents a chat room with its metadata
type Room struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
	MaxPeers    int    `json:"max_peers"`
	IsPrivate   bool   `json:"is_private"`
	AccessKey   string `json:"access_key,omitempty"`  // Klucz dostępu do pokoju
	ListenPort  int    `json:"listen_port,omitempty"` // Port, na którym nasłuchuje host pokoju
}

// GenerateRoomID creates a cryptographically secure room ID
func GenerateRoomID() (string, error) {
	// generate 24 random bytes (192 bits of entropy)
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// encode using Base58 (Bitcoin-style alphabet)
	encoded := base58.Encode(randomBytes)

	// ensure consistent length by padding or truncating
	targetLength := RoomIDLength - len(RoomIDPrefix)
	if len(encoded) < targetLength {
		// pad with additional random characters if needed
		padding := make([]byte, targetLength-len(encoded))
		rand.Read(padding)
		encoded += base58.Encode(padding)
	} else if len(encoded) > targetLength {
		// truncate if too long
		encoded = encoded[:targetLength]
	}

	// combine prefix with encoded random data
	roomID := RoomIDPrefix + encoded

	return roomID, nil
}

// ValidateRoomID checks if a room ID has the correct format
func ValidateRoomID(roomID string) bool {
	// check prefix
	if !strings.HasPrefix(roomID, RoomIDPrefix) {
		return false
	}

	// check total length
	if len(roomID) != RoomIDLength {
		return false
	}

	// extract and validate the encoded part
	encoded := roomID[len(RoomIDPrefix):]

	// decode to ensure it's valid Base58
	decoded := base58.Decode(encoded)
	return len(decoded) > 0
}

// GetInfoHash creates a BitTorrent-compatible InfoHash from room ID
func GetInfoHash(roomID string) string {
	// create SHA-256 hash of room ID
	hash := sha256.Sum256([]byte(roomID))

	// return first 20 bytes as hex string (BitTorrent InfoHash format)
	return hex.EncodeToString(hash[:20])
}

// GetDiscoveryHash creates a discovery hash for mDNS and DNS
func GetDiscoveryHash(roomID string) string {
	// create a shorter hash for service discovery
	hash := sha256.Sum256([]byte(roomID))

	// return first 8 bytes as hex string for shorter service names
	return hex.EncodeToString(hash[:8])
}

// GenerateAccessKey tworzy losowy klucz dostępu do pokoju
func GenerateAccessKey() (string, error) {
	// Generuj 16 bajtów losowych danych (128 bitów entropii)
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("błąd generowania klucza dostępu: %w", err)
	}

	// Koduj w formacie Base58 dla czytelności i łatwego kopiowania
	encodedKey := base58.Encode(keyBytes)

	// Upewnij się, że klucz ma odpowiednią długość (ok. 22-24 znaki)
	if len(encodedKey) > 24 {
		encodedKey = encodedKey[:24]
	}

	return encodedKey, nil
}

// NewRoom creates a new room with the given parameters
func NewRoom(name, description string, maxPeers int, isPrivate bool) (*Room, error) {
	roomID, err := GenerateRoomID()
	if err != nil {
		return nil, err
	}

	// Generuj klucz dostępu dla prywatnych pokojów
	var accessKey string
	if isPrivate {
		accessKey, err = GenerateAccessKey()
		if err != nil {
			return nil, err
		}
	}

	return &Room{
		ID:          roomID,
		Name:        name,
		Description: description,
		MaxPeers:    maxPeers,
		IsPrivate:   isPrivate,
		AccessKey:   accessKey,
	}, nil
}

// RegenerateAccessKey tworzy nowy klucz dostępu do pokoju
func (r *Room) RegenerateAccessKey() error {
	if !r.IsPrivate {
		return fmt.Errorf("nie można wygenerować klucza dla publicznego pokoju")
	}

	newKey, err := GenerateAccessKey()
	if err != nil {
		return err
	}

	r.AccessKey = newKey
	return nil
}

// ValidateAccessKey sprawdza, czy podany klucz dostępu jest prawidłowy
func (r *Room) ValidateAccessKey(key string) bool {
	if !r.IsPrivate {
		return true // Pokoje publiczne nie wymagają klucza
	}

	return r.AccessKey == key
}

// GetShortID returns a shortened version of the room ID for display
func (r *Room) GetShortID() string {
	if len(r.ID) > 16 {
		return r.ID[:8] + "..." + r.ID[len(r.ID)-8:]
	}
	return r.ID
}

// GetServiceName returns the service name for mDNS discovery
func (r *Room) GetServiceName() string {
	hash := GetDiscoveryHash(r.ID)
	return fmt.Sprintf("_execp2p_%s._tcp", hash)
}
