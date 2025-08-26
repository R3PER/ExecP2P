# Serwer Sygnalizacyjny dla Entropia

Ten serwer sygnalizacyjny pomaga w nawiązaniu połączeń P2P między użytkownikami aplikacji Entropia, którzy znajdują się za NAT-em lub firewallami.

## Jak działa serwer sygnalizacyjny?

1. **Rola pośrednika** - serwer sygnalizacyjny działa jako pośrednik w procesie nawiązywania połączenia, ale sam nie przesyła żadnych wiadomości między użytkownikami. Pomaga tylko w znalezieniu i połączeniu użytkowników.

2. **Rejestracja pokojów** - gdy użytkownik tworzy pokój, jego adres IP i port są rejestrowane na serwerze.

3. **Wyszukiwanie pokojów** - gdy użytkownik chce dołączyć do pokoju, aplikacja pyta serwer o adresy tego pokoju.

4. **UDP Hole Punching** - po otrzymaniu adresów, aplikacja używa techniki UDP hole punching, aby nawiązać bezpośrednie połączenie P2P.

## Czy serwer jest wymagany?

**Serwer sygnalizacyjny jest opcjonalny**. Bez serwera, aplikacja nadal działa w następujących przypadkach:

- **Użytkownicy w tej samej sieci lokalnej (LAN)** - aplikacja wykorzystuje automatyczne wykrywanie przez mDNS i broadcast UDP
- **Użytkownicy na tym samym komputerze** - aplikacja wykrywa lokalne instancje na różnych portach

Serwer sygnalizacyjny jest potrzebny tylko wtedy, gdy:
- Użytkownicy są w różnych sieciach (np. jeden w domu, drugi w biurze)
- Użytkownicy są za NAT-em lub firewallem, który blokuje bezpośrednie połączenia

## Uruchomienie serwera

### Szybkie uruchomienie

1. Upewnij się, że masz zainstalowany Go (1.18 lub nowszy)
2. W katalogu `server` wykonaj:

```bash
# Zainstaluj zależności
go mod tidy

# Uruchom serwer
go run signaling_server.go
```

Serwer domyślnie nasłuchuje na porcie 8085.

### Uruchomienie na serwerze publicznym

Aby inni użytkownicy mogli korzystać z serwera, musisz uruchomić go na serwerze z publicznym adresem IP:

1. Skopiuj katalog `server` na serwer
2. Zainstaluj Go na serwerze
3. Uruchom serwer jak wyżej
4. Upewnij się, że port 8085 jest otwarty w zaporze serwera

### Uruchomienie jako usługa systemd (Linux)

Aby serwer działał w tle jako usługa systemowa na Linuksie:

1. Utwórz plik `entropia-signaling.service` w `/etc/systemd/system/`:

```
[Unit]
Description=Entropia Signaling Server
After=network.target

[Service]
ExecStart=/usr/local/bin/entropia-signaling
Restart=on-failure
User=entropia
Group=entropia
WorkingDirectory=/home/entropia/server

[Install]
WantedBy=multi-user.target
```

2. Zbuduj aplikację:

```bash
cd server
go build -o entropia-signaling signaling_server.go
sudo mv entropia-signaling /usr/local/bin/
```

3. Włącz i uruchom usługę:

```bash
sudo systemctl enable entropia-signaling
sudo systemctl start entropia-signaling
```

## Konfiguracja klienta Entropia

Aby korzystać z niestandardowego serwera sygnalizacyjnego, zmodyfikuj plik `internal/discovery/signaling.go`:

```go
// Zmień domyślny adres serwera z:
const DefaultSignalingServer = "https://entropia-signaling.example.com"

// Na adres twojego serwera:
const DefaultSignalingServer = "http://twoj-serwer.com:8085"
```

Następnie ponownie zbuduj aplikację.

## Uwagi bezpieczeństwa

Ten prosty serwer sygnalizacyjny nie wymaga uwierzytelniania i działa przez HTTP. W środowisku produkcyjnym zalecane jest:

1. Dodanie warstwy TLS (HTTPS)
2. Dodanie prostej autentykacji
3. Ograniczenie maksymalnej liczby rejestracji z jednego adresu IP

Domyślna implementacja nadaje się do testów i małych wdrożeń. Dla większych wdrożeń należy rozważyć rozbudowę zabezpieczeń.
