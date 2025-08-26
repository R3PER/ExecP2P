package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// RoomRegistration zawiera dane do rejestracji pokoju
type RoomRegistration struct {
	RoomID         string `json:"room_id"`         // Identyfikator pokoju
	PublicAddr     string `json:"public_addr"`     // Publiczny adres IP:port
	IsNATed        bool   `json:"is_nated"`        // Czy jesteśmy za NATem
	STUNAddr       string `json:"stun_addr"`       // Adres uzyskany przez STUN
	BehindSymNAT   bool   `json:"behind_sym_nat"`  // Czy jesteśmy za symetrycznym NATem
	CreationTime   int64  `json:"creation_time"`   // Czas utworzenia pokoju
	ExpirationTime int64  `json:"expiration_time"` // Czas wygaśnięcia rejestracji
}

// RoomInfo zawiera informacje o pokoju
type RoomInfo struct {
	RoomID       string   `json:"room_id"`        // Identyfikator pokoju
	PublicAddrs  []string `json:"public_addrs"`   // Lista publicznych adresów
	LastSeen     int64    `json:"last_seen"`      // Kiedy ostatnio widziany
	BehindSymNAT bool     `json:"behind_sym_nat"` // Czy za symetrycznym NATem
}

// Prosta implementacja serwera sygnalizacyjnego
type SignalingServer struct {
	rooms      map[string]*RoomInfo
	roomsMutex sync.RWMutex
}

// Tworzy nowy serwer sygnalizacyjny
func NewSignalingServer() *SignalingServer {
	server := &SignalingServer{
		rooms: make(map[string]*RoomInfo),
	}
	// Uruchom oczyszczanie przestarzałych wpisów
	go server.cleanupExpiredRooms()
	return server
}

// Obsługuje rejestrację pokoju
func (s *SignalingServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Tylko metoda POST
	if r.Method != http.MethodPost {
		http.Error(w, "Metoda nie dozwolona", http.StatusMethodNotAllowed)
		return
	}

	// Parsuj żądanie JSON
	var reg RoomRegistration
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reg); err != nil {
		http.Error(w, "Nieprawidłowy format JSON", http.StatusBadRequest)
		return
	}

	// Weryfikuj dane
	if reg.RoomID == "" || reg.PublicAddr == "" {
		http.Error(w, "Brakujące wymagane pola", http.StatusBadRequest)
		return
	}

	// Utwórz lub zaktualizuj informacje o pokoju
	s.roomsMutex.Lock()
	roomInfo, exists := s.rooms[reg.RoomID]
	if !exists {
		roomInfo = &RoomInfo{
			RoomID:       reg.RoomID,
			PublicAddrs:  []string{},
			LastSeen:     time.Now().Unix(),
			BehindSymNAT: reg.BehindSymNAT,
		}
		s.rooms[reg.RoomID] = roomInfo
	}

	// Dodaj adresy do listy (jeśli jeszcze nie istnieją)
	addrExists := false
	for _, addr := range roomInfo.PublicAddrs {
		if addr == reg.PublicAddr {
			addrExists = true
			break
		}
	}
	if !addrExists {
		roomInfo.PublicAddrs = append(roomInfo.PublicAddrs, reg.PublicAddr)
	}

	// Jeśli podano adres STUN i różni się od publicAddr, dodaj go też
	if reg.STUNAddr != "" && reg.STUNAddr != reg.PublicAddr {
		stunExists := false
		for _, addr := range roomInfo.PublicAddrs {
			if addr == reg.STUNAddr {
				stunExists = true
				break
			}
		}
		if !stunExists {
			roomInfo.PublicAddrs = append(roomInfo.PublicAddrs, reg.STUNAddr)
		}
	}

	// Aktualizuj czas ostatniego widzenia
	roomInfo.LastSeen = time.Now().Unix()
	s.roomsMutex.Unlock()

	// Zwróć sukces
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "ok"}`)
}

// Obsługuje pobranie informacji o pokoju
func (s *SignalingServer) handleGetRoom(w http.ResponseWriter, r *http.Request) {
	// Tylko metoda GET
	if r.Method != http.MethodGet {
		http.Error(w, "Metoda nie dozwolona", http.StatusMethodNotAllowed)
		return
	}

	// Pobierz ID pokoju z URL
	vars := mux.Vars(r)
	roomID := vars["roomID"]
	if roomID == "" {
		http.Error(w, "Brak ID pokoju", http.StatusBadRequest)
		return
	}

	// Pobierz informacje o pokoju
	s.roomsMutex.RLock()
	roomInfo, exists := s.rooms[roomID]
	s.roomsMutex.RUnlock()

	if !exists {
		http.Error(w, "Pokój nie znaleziony", http.StatusNotFound)
		return
	}

	// Serializuj i zwróć informacje
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(roomInfo); err != nil {
		http.Error(w, "Błąd serializacji JSON", http.StatusInternalServerError)
		return
	}
}

// Obsługuje listę wszystkich aktywnych pokojów (dla celów diagnostycznych)
func (s *SignalingServer) handleListRooms(w http.ResponseWriter, r *http.Request) {
	// Tylko metoda GET
	if r.Method != http.MethodGet {
		http.Error(w, "Metoda nie dozwolona", http.StatusMethodNotAllowed)
		return
	}

	// Pobierz listę pokojów
	s.roomsMutex.RLock()
	rooms := make([]*RoomInfo, 0, len(s.rooms))
	for _, roomInfo := range s.rooms {
		rooms = append(rooms, roomInfo)
	}
	s.roomsMutex.RUnlock()

	// Serializuj i zwróć listę
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(rooms); err != nil {
		http.Error(w, "Błąd serializacji JSON", http.StatusInternalServerError)
		return
	}
}

// Czyści pokoje, które wygasły
func (s *SignalingServer) cleanupExpiredRooms() {
	// Uruchom co 5 minut
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()
		s.roomsMutex.Lock()
		// Usuń pokoje starsze niż 2 godziny
		for id, roomInfo := range s.rooms {
			if now-roomInfo.LastSeen > 2*60*60 {
				delete(s.rooms, id)
				log.Printf("Usunięto wygasły pokój: %s", id)
			}
		}
		s.roomsMutex.Unlock()
	}
}

func main() {
	// Utwórz serwer
	server := NewSignalingServer()

	// Utwórz router
	router := mux.NewRouter()
	router.HandleFunc("/api/register", server.handleRegister).Methods("POST")
	router.HandleFunc("/api/room/{roomID}", server.handleGetRoom).Methods("GET")
	router.HandleFunc("/api/rooms", server.handleListRooms).Methods("GET")

	// Obsługa CORS dla development
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Uruchom serwer
	port := 8085
	log.Printf("Uruchamianie serwera sygnalizacyjnego na porcie %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), router); err != nil {
		log.Fatalf("Nie można uruchomić serwera: %v", err)
	}
}
