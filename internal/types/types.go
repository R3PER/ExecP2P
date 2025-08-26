package types

// CreateRoomResult zawiera wynik tworzenia nowego pokoju
type CreateRoomResult struct {
	RoomID     string
	AccessKey  string
	ListenPort int // Port, na którym nasłuchuje twórca pokoju
}
