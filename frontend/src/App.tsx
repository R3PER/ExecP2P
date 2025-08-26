import { useState, useEffect } from 'react';
import { MainLayout } from './components/layout/MainLayout';
import { ConnectView } from './components/connect/ConnectView';
import { ChatView } from './components/chat/ChatView';
import { SettingsView } from './components/settings/SettingsView';

// Interfejs do przechowywania stanu aplikacji
interface AppState {
  view: string;
  connectionStatus: {
    connected: boolean;
    secure: boolean;
    peerCount: number;
  };
  securityInfo: {
    identityFingerprint?: string;
    roomId?: string;
    listenPort?: number;
    kemAlgo?: string;
    sigAlgo?: string;
    symAlgo?: string;
    peer_id?: string;  // ID użytkownika (peerID)
    accessKey?: string; // Klucz dostępu do pokoju
  };
  peerFingerprints: Record<string, string>;
  isRoomCreator: boolean; // Czy użytkownik jest twórcą pokoju
}

function App() {
  // Stan aplikacji
  const [state, setState] = useState<AppState>({
    view: 'connect',
    connectionStatus: {
      connected: false,
      secure: false,
      peerCount: 0,
    },
    securityInfo: {
      identityFingerprint: undefined,
      roomId: undefined,
      listenPort: undefined,
      kemAlgo: 'CRYSTALS-Kyber-1024',
      sigAlgo: 'CRYSTALS-DILITHIUM-5',
      symAlgo: 'ChaCha20-Poly1305',
      accessKey: undefined,
    },
    peerFingerprints: {},
    isRoomCreator: false,
  });

  // Efekt do inicjalizacji i nasłuchiwania zdarzeń
  useEffect(() => {
    // Inicjalizacja nasłuchiwania zdarzeń z Wails
    const fetchInitialData = async () => {
      try {
        // Pobierz dane bezpieczeństwa
        const securitySummary = await window.go.wailsbridge.Bridge.GetSecuritySummary();
        const fingerprint = await window.go.wailsbridge.Bridge.GetPeerFingerprint();
        
      // Pobierz status sieci
      const networkStatus = await window.go.wailsbridge.Bridge.GetNetworkStatus();
      
      // Sprawdź, czy faktycznie istnieje pokój (czy roomId jest ustawiony i niepusty)
      const roomExists = networkStatus.room_id && networkStatus.room_id !== "";
        
        // Aktualizuj stan, ale NIE zmieniaj widoku automatycznie
        setState(prev => ({
          ...prev,
          connectionStatus: {
            connected: roomExists && (networkStatus.is_running || false),
            secure: networkStatus.e2e_encryption || false,
            peerCount: networkStatus.connected_peers || 0,
          },
          securityInfo: {
            ...prev.securityInfo,
            identityFingerprint: fingerprint,
            listenPort: networkStatus.listen_port,
            roomId: networkStatus.room_id || undefined,
            peer_id: networkStatus.peer_id || undefined,
            kemAlgo: securitySummary.encryption_algorithms?.key_exchange || 'CRYSTALS-Kyber-1024',
            sigAlgo: securitySummary.encryption_algorithms?.signatures || 'CRYSTALS-DILITHIUM-5',
            symAlgo: securitySummary.encryption_algorithms?.symmetric || 'ChaCha20-Poly1305',
          },
          // Upewnij się, że widok jest ustawiony na 'connect', jeśli nie ma pokoju
          view: roomExists ? prev.view : 'connect'
        }));
      } catch (error) {
        console.error('Błąd podczas pobierania danych:', error);
      }
    };
    
    // Nasłuchuj zdarzeń
    window.runtime.EventsOn('message:received', (message) => {
      console.log('Otrzymano wiadomość:', message);
      // ChatView obsługuje wiadomości samodzielnie, nie potrzebujemy tego tutaj
    });
    
    // Nasłuchuj zdarzeń zmiany widoku z komponentów
    window.runtime.EventsOn('view:change', (viewName: string) => {
      console.log('Zmiana widoku przez zdarzenie:', viewName);
      handleViewChange(viewName);
    });
    
    window.runtime.EventsOn('status:update', (status) => {
      console.log('Aktualizacja statusu:', status);
      setState(prev => ({
        ...prev,
        connectionStatus: {
          connected: status.is_running || false,
          secure: status.e2e_encryption || false,
          peerCount: status.connected_peers || 0,
        },
        securityInfo: {
          ...prev.securityInfo,
          roomId: status.room_id || prev.securityInfo.roomId,
        }
      }));
    });
    
    fetchInitialData();
    
    // Czyszczenie nasłuchiwania przy odmontowywaniu
    return () => {
      window.runtime.EventsOff('message:received');
      window.runtime.EventsOff('status:update');
      window.runtime.EventsOff('view:change');
    };
  }, []);

  // Funkcja do zmiany widoku
  const handleViewChange = (view: string) => {
    setState(prev => ({ ...prev, view }));
  };
  
  // Obsługa regeneracji klucza dostępu
  const handleRegenerateAccessKey = async () => {
    try {
      const newKey = await window.go.wailsbridge.Bridge.RegenerateRoomAccessKey();
      setState(prev => ({
        ...prev,
        securityInfo: {
          ...prev.securityInfo,
          accessKey: newKey
        }
      }));
      return newKey;
    } catch (error) {
      console.error('Błąd podczas regeneracji klucza dostępu:', error);
      throw error;
    }
  };

  // Funkcja wywoływana po pomyślnym połączeniu
  const handleConnectionSuccess = async () => {
    try {
      // Pobierz aktualny status
      const networkStatus = await window.go.wailsbridge.Bridge.GetNetworkStatus();
      
      // Znajdź odciski palca i informacje o bezpieczeństwie
      const securitySummary = await window.go.wailsbridge.Bridge.GetSecuritySummary();
      
      // Sprawdź, czy jesteśmy twórcą pokoju
      const isCreator = networkStatus.is_listener === true;
      console.log("Network status:", networkStatus);
      console.log("Is room creator:", isCreator);
      
      // Pobierz klucz dostępu jeśli jesteśmy twórcą
      let accessKey: string | undefined;
      if (isCreator) {
        try {
          accessKey = await window.go.wailsbridge.Bridge.GetRoomAccessKey();
        } catch (e) {
          console.warn('Nie udało się pobrać klucza dostępu:', e);
        }
      }
      
      // Sprawdź, czy w securitySummary mamy informacje o pokoju
      if (securitySummary.room_info) {
        accessKey = securitySummary.room_info.access_key;
      }
      
      setState(prev => ({
        ...prev,
        view: 'chat',
        connectionStatus: {
          connected: networkStatus.is_running || false,
          secure: networkStatus.e2e_encryption || false,
          peerCount: networkStatus.connected_peers || 0,
        },
        securityInfo: {
          ...prev.securityInfo,
          roomId: networkStatus.room_id || prev.securityInfo.roomId,
          accessKey: accessKey,
        },
        // Pobieramy odciski palca z podsumowania bezpieczeństwa
        peerFingerprints: securitySummary.peer_fingerprints || {},
        isRoomCreator: isCreator,
      }));
    } catch (error) {
      console.error('Błąd podczas aktualizacji statusu połączenia:', error);
    }
  };

  // Renderowanie odpowiedniego widoku
  const renderView = () => {
    switch (state.view) {
      case 'connect':
        return <ConnectView onSuccess={handleConnectionSuccess} />;
      case 'chat':
        return <ChatView 
          connected={state.connectionStatus.secure} 
          userID={state.securityInfo.peer_id || ''} 
          roomId={state.securityInfo.roomId}
          accessKey={state.securityInfo.accessKey}
          isRoomCreator={state.isRoomCreator}
          onRegenerateAccessKey={handleRegenerateAccessKey}
        />;
      case 'settings':
        return (
          <SettingsView 
            identityFingerprint={state.securityInfo.identityFingerprint}
            roomId={state.securityInfo.roomId}
            listenPort={state.securityInfo.listenPort}
            securityInfo={{
              kemAlgo: state.securityInfo.kemAlgo,
              sigAlgo: state.securityInfo.sigAlgo,
              symAlgo: state.securityInfo.symAlgo,
            }}
            peerFingerprints={state.peerFingerprints}
            isRoomCreator={state.isRoomCreator}
            accessKey={state.securityInfo.accessKey}
            onRegenerateAccessKey={handleRegenerateAccessKey}
          />
        );
      default:
        return <ConnectView onSuccess={handleConnectionSuccess} />;
    }
  };

  return (
    <MainLayout 
      activeView={state.view} 
      onViewChange={handleViewChange}
      connectionStatus={{
        connected: state.connectionStatus.connected,
        secure: state.connectionStatus.secure,
      }}
    >
      {renderView()}
    </MainLayout>
  );
}

export default App;
