package platform

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
)

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsMacOS returns true if running on macOS
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// GetOSName returns a human-readable OS name
func GetOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	default:
		return runtime.GOOS
	}
}

// InitPlatform initializes platform-specific settings
func InitPlatform() error {
	log.Printf("Initializing platform-specific settings for %s", GetOSName())

	if IsWindows() {
		// Windows-specific initialization
		return initWindows()
	} else if IsLinux() {
		// Linux-specific initialization
		return initLinux()
	} else if IsMacOS() {
		// macOS-specific initialization
		return initMacOS()
	}

	return nil
}

// Windows-specific initialization
func initWindows() error {
	// Create debug log file in a known location
	logFile, err := os.Create(os.ExpandEnv("%USERPROFILE%\\execp2p_debug.log"))
	if err != nil {
		return fmt.Errorf("failed to create debug log file: %w", err)
	}

	// Set output to both stderr and file
	log.SetOutput(logFile)
	log.Printf("ExecP2P starting on Windows - %s", runtime.GOARCH)

	// Check WebView2 Runtime
	log.Printf("WebView2 environment: checking...")
	// WebView2 check is handled by Wails

	return nil
}

// Linux-specific initialization
func initLinux() error {
	// Nothing special needed for Linux yet
	return nil
}

// macOS-specific initialization
func initMacOS() error {
	// Nothing special needed for macOS yet
	return nil
}

// GetNetworkBroadcastAddresses returns dynamically detected broadcast addresses
func GetNetworkBroadcastAddresses() []string {
	broadcastAddrs := []string{
		"255.255.255.255:19847", // Global broadcast
	}

	// Dodaj standardowe zakresy prywatne jako fallback
	standardBroadcasts := []string{
		"192.168.255.255:19847",
		"10.255.255.255:19847",
		"172.31.255.255:19847",
	}

	// Dynamicznie wykryj adresy rozgłoszeniowe z interfejsów sieciowych
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			// Pomijamy interfejsy loopback i wyłączone
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				ipnet, ok := addr.(*net.IPNet)
				if !ok || ipnet.IP.To4() == nil {
					continue
				}

				// Oblicz adres broadcast dla tej podsieci
				broadcast := calculateBroadcastAddr(ipnet)
				if broadcast != "" {
					broadcastAddrs = append(broadcastAddrs, broadcast+":19847")
				}
			}
		}
	}

	// Jeśli nie udało się znaleźć żadnych adresów, użyj standardowych
	if len(broadcastAddrs) <= 1 { // tylko globalny broadcast
		broadcastAddrs = append(broadcastAddrs, standardBroadcasts...)
	}

	// Na Windows zawsze dodaj globalny broadcast
	if IsWindows() && !contains(broadcastAddrs, "255.255.255.255:19847") {
		broadcastAddrs = append(broadcastAddrs, "255.255.255.255:19847")
	}

	return broadcastAddrs
}

// calculateBroadcastAddr oblicza adres rozgłoszeniowy dla podanej podsieci
func calculateBroadcastAddr(ipnet *net.IPNet) string {
	ip := ipnet.IP.To4()
	if ip == nil {
		return ""
	}

	mask := ipnet.Mask
	broadcast := net.IP(make([]byte, 4))

	for i := 0; i < 4; i++ {
		broadcast[i] = ip[i] | ^mask[i]
	}

	return broadcast.String()
}

// contains sprawdza czy slice zawiera dany string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
