# ExecP2P 🛡️

## Post-Quantum End-to-End Encrypted Desktop Messenger

> **⚠️ Prototype – avoid production use.** Software hasn’t undergone professional audits. Do **not** rely on it for highly sensitive information.

**ExecP2P delivers a modern peer-to-peer messaging experience with intuitive interface, leveraging post-quantum cryptography to protect communications against future threats.**

**Note:** The source code provided here originates from the main owner but has been modified by us. It requires several improvements, including fixing synchronization issues, nickname assignment, and enhancing the signaling server and bugs rooms.

Owner https://github.com/reschjonas/entropia

It demonstrates NIST-standard quantum-resistant algorithms combined with peer discovery to create a secure, serverless channel. Development is active, and contributions are welcome.

Explore details in [**Technical Overview**](TECHNICAL_OVERVIEW.md).

---

## Screens and Videos 

<img width="1288" height="850" alt="1" src="https://github.com/user-attachments/assets/8576082f-4d57-4638-aaaa-cb4a2e1949ab" />
<img width="1279" height="840" alt="2" src="https://github.com/user-attachments/assets/26ec4573-da07-4ce7-aff0-ada8a4f00eda" />
<img width="1285" height="849" alt="3" src="https://github.com/user-attachments/assets/8c267061-1f81-4274-b286-c5f23b237739" />
<img width="1283" height="828" alt="4" src="https://github.com/user-attachments/assets/91300fd9-e605-42fd-b50e-c3594ef62ad0" />
<img width="1277" height="827" alt="5" src="https://github.com/user-attachments/assets/b651eadd-5353-4ea3-a58f-66e4b44a768e" />
<img width="1274" height="842" alt="6" src="https://github.com/user-attachments/assets/2b33cbef-8b6b-4940-b7a2-33949dc1ea9e" />


---

## Why ExecP2P?

- **🛡️ Quantum-Resistant:** CRYSTALS-Kyber for key exchange, CRYSTALS-Dilithium for signatures—both NIST-approved to prevent future decryption attacks.
- **🖥️ Modern GUI:** Clean, cross-platform design using `webview`, consolidating all functionality into one user-friendly application.
- **🌐 Serverless Architecture:** True peer-to-peer messaging without central servers. Data never leaves your network.
- **🔍 Smart Peer Discovery:** Detects peers locally via mDNS/UDP broadcast or globally via BitTorrent DHT; manual IP optional.
- **🔄 Forward Secrecy:** Ephemeral keys rotated every 15 minutes; compromised keys can’t decrypt past conversations.
- **⚡ Lightweight & Efficient:** Single Go binary with embedded assets, using QUIC transport for reliable, fast delivery.

---

## Technologies Used

ExecP2P employs modern solutions ensuring private communication:

### Backend (Go)

- **Go 1.24+** – core programming language
- **QUIC** – encrypted streaming transport
- **Post-quantum crypto**:
  - **CRYSTALS-Kyber-1024** – key encapsulation
  - **CRYSTALS-Dilithium-5** – digital signatures
  - **XChaCha20-Poly1305** – symmetric encryption
- **BitTorrent DHT** – decentralized peer discovery
- **mDNS/UDP Broadcast** – local network detection

### Frontend (React)

- **Wails** – Go + modern web interface
- **React** – interactive UI library
- **TypeScript** – typed JavaScript for reliability
- **Tailwind CSS** – rapid styling framework

### Architecture

- **Modular code structure** – internal packages
- **Layered implementation**:
  - Crypto (`internal/crypto`)
  - Transport (`internal/network`)
  - Discovery (`internal/discovery`)
  - Interface (`internal/ui`)
- **WebView GUI** – embedded browser for React rendering
- **Embedded FS** – HTML/CSS/JS included in Go binary

---

## Installation & Usage

### Prerequisites

- **Go 1.24+** – [https://go.dev/dl/](https://go.dev/dl/) or system package manager
- **UDP-accessible network** – firewalls/NAT must allow QUIC

### Build from source (Original Author)

The following instructions clone and build the repository from the original software author:

```bash
# Clone & build
git clone https://github.com/reschjonas/entropia.git
cd entropia

# Static binary for your OS/arch
go build -trimpath -ldflags="-s -w" -o entropia .

# Or install into $GOBIN in one line
go install github.com/reschjonas/entropia@latest
```

### Build from source (Modified by R3PER)

This version is modified by user R3PER with custom changes, fixes, and experimental improvements:

```bash
git clone https://github.com/R3PER/execp2p.git
cd execp2p
go build -trimpath -ldflags="-s -w" -o execp2p .
# Or install directly
go install github.com/R3PER/execp2p@latest
```

### Launch Application

1. Start executable:

```bash
./execp2p
```

2. Peer 1 (Create Room):
   - Click “Create & Start Listening.”
   - Copy generated Room ID from “Settings.”
3. Peer 2 (Join Room):
   - Paste Room ID in “Join Existing Room.”
   - Click “Join Room”; auto-discovery finds creator.
   - Optional LAN: input `ip:port`.

Chat begins when both sides display **Secure**.

---

## Operation Principle

ExecP2P secures communication in four stages:

1. **Discovery:** mDNS, UDP broadcast, BitTorrent DHT locate peer addresses
2. **Handshake:** Quantum-safe key exchange via CRYSTALS-Kyber
3. **Authentication:** Identity verification using CRYSTALS-Dilithium; out-of-band fingerprint confirmation
4. **Encrypted Chat:** Messages protected by XChaCha20-Poly1305 symmetric cipher

Full architectural and cryptographic details: [**Technical Overview**](TECHNICAL_OVERVIEW.md).

---

## 🔒 Security

### Threat Model

- Prototype; no audit. For learning purposes only
- **TOFU (Trust On First Use):** verify peer fingerprint out-of-band
- **IP Visibility:** P2P nature exposes IP addresses; anonymity not guaranteed

### Fingerprint Verification

1. Open **Settings**
2. Locate your **Identity Fingerprint** and verified peers
3. Confirm via separate channel (call or in-person)

---

## Logging

ExecP2P includes **silent-by-default structured logging**:

```bash
execp2p --log-level info      # debug | info | warn | error
```

---

## Roadmap

- ICE/QUIC NAT traversal
- Group chats (multiple peers)
- File transfer & chat history persistence
- Formal security audit

PRs welcome.

---

