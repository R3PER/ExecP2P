import React, { useState, useEffect } from "react";
import { Card, CardHeader, CardTitle, CardContent, CardFooter, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Clipboard, Key, UserPlus, Search, Lock, RefreshCw } from "lucide-react";

// Importuj runtime Wails, aby móc emitować zdarzenia
declare global {
  interface Window {
    runtime: {
      EventsOn: (event: string, callback: (data: any) => void) => void;
      EventsOff: (event: string) => void;
      EventsEmit: (event: string, ...args: any[]) => void;
    };
  }
}

interface ConnectViewProps {
  onSuccess?: () => void;
}

// Etapy dołączania do pokoju
enum JoinSteps {
  ENTER_ROOM_ID = 0,
  SEARCHING = 1,
  ENTER_ACCESS_KEY = 2,
  CONNECTING = 3,
  CONNECTED = 4,
  ERROR = 5
}

export function ConnectView({ onSuccess }: ConnectViewProps) {
  // Tworzenie pokoju
  const [creatingRoom, setCreatingRoom] = useState(false);
  const [createdRoomInfo, setCreatedRoomInfo] = useState<{room_id: string, access_key: string, listen_port?: number} | null>(null);
  
  // Dołączanie do pokoju
  const [roomId, setRoomId] = useState("");
  const [accessKey, setAccessKey] = useState("");
  const [joinStep, setJoinStep] = useState<JoinSteps>(JoinSteps.ENTER_ROOM_ID);
  const [foundRoomInfo, setFoundRoomInfo] = useState<{users_count: number, address?: string} | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Nasłuchiwanie zdarzeń bezpieczeństwa
  useEffect(() => {
    window.runtime.EventsOn("security:message", (message: string) => {
      console.log("Komunikat bezpieczeństwa:", message);
    });

    return () => {
      window.runtime.EventsOff("security:message");
    };
  }, []);

  // Tworzenie nowego pokoju
  const handleCreate = async () => {
    try {
      setCreatingRoom(true);
      
      // Wywołanie funkcji z Wails
      const result = await window.go.wailsbridge.Bridge.CreateRoom();
      
      // Zapisz informacje o utworzonym pokoju, wraz z portem nasłuchiwania
      setCreatedRoomInfo({
        room_id: result.room_id,
        access_key: result.access_key,
        listen_port: result.listen_port
      });
      
      if (onSuccess) onSuccess(); // Przejdź od razu do czatu
      
    } catch (error) {
      console.error("Błąd podczas tworzenia pokoju:", error);
      setError(`Błąd podczas tworzenia pokoju: ${error}`);
    } finally {
      setCreatingRoom(false);
    }
  };

  // Etap 1: Wyszukiwanie pokoju po ID
  const handleSearchRoom = async () => {
    if (!roomId) {
      setError("Podaj ID pokoju");
      return;
    }

    try {
      setJoinStep(JoinSteps.SEARCHING);
      setError(null);
      
      // Wywołaj funkcję wyszukiwania
      const result = await window.go.wailsbridge.Bridge.FindRoom(roomId);
      
      // Ustawienie informacji o znalezionym pokoju
      setFoundRoomInfo({
        users_count: result.users_count || 1, // Domyślnie zakładamy 1 użytkownika (twórcę)
        address: result.address
      });
      
      // Przejdź do następnego etapu
      setJoinStep(JoinSteps.ENTER_ACCESS_KEY);
      
    } catch (error) {
      console.error("Błąd podczas wyszukiwania pokoju:", error);
      setError(`Nie znaleziono pokoju: ${error}`);
      setJoinStep(JoinSteps.ERROR);
    }
  };

  // Etap 3: Łączenie z pokojem
  const handleConnect = async () => {
    if (!accessKey) {
      setError("Podaj klucz dostępu do pokoju");
      return;
    }
    
    try {
      setJoinStep(JoinSteps.CONNECTING);
      setError(null);
      
      // Łączenie z pokojem
      if (foundRoomInfo?.address) {
        // Jeśli znaleziono adres, użyj go do bezpośredniego połączenia
        await window.go.wailsbridge.Bridge.JoinRoom(roomId, foundRoomInfo.address, accessKey);
      } else {
        // W przeciwnym razie użyj automatycznego wyszukiwania
        await window.go.wailsbridge.Bridge.JoinRoomWithFallback(roomId, accessKey);
      }
      
      // Jeśli dotarliśmy tutaj, połączenie się powiodło
      setJoinStep(JoinSteps.CONNECTED);
      
      // Przejdź do widoku czatu
      if (onSuccess) onSuccess();
      
    } catch (error) {
      console.error("Błąd podczas łączenia z pokojem:", error);
      setError(`Błąd połączenia: ${error}`);
      setJoinStep(JoinSteps.ERROR);
    }
  };

  // Resetuj proces dołączania
  const resetJoinProcess = () => {
    setJoinStep(JoinSteps.ENTER_ROOM_ID);
    setError(null);
    setFoundRoomInfo(null);
    setAccessKey("");
  };

  // Renderowanie różnych etapów procesu dołączania
  const renderJoinStep = () => {
    switch (joinStep) {
      case JoinSteps.ENTER_ROOM_ID:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-400 mb-4">
              Podaj ID pokoju, do którego chcesz dołączyć. ID możesz otrzymać od twórcy pokoju.
            </p>
            <div className="relative">
              <Input 
                placeholder="Wprowadź ID pokoju..." 
                value={roomId}
                onChange={(e) => setRoomId(e.target.value)}
                className="pr-12"
              />
              <Key className="absolute right-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-500" />
            </div>
            <Button 
              onClick={handleSearchRoom} 
              disabled={!roomId || creatingRoom}
              className="w-full mt-4 flex items-center justify-center gap-2"
            >
              <Search className="h-4 w-4" />
              <span>Wyszukaj pokój</span>
            </Button>
          </div>
        );
        
      case JoinSteps.SEARCHING:
        return (
          <div className="py-8 text-center">
            <div className="flex flex-col items-center justify-center">
              <RefreshCw className="h-10 w-10 text-blue-500 animate-spin mb-4" />
            </div>
            <p>Wyszukiwanie pokoju {roomId}...</p>
          </div>
        );
        
      case JoinSteps.ENTER_ACCESS_KEY:
        return (
          <div className="space-y-4">
            <div className="p-4 bg-blue-900/20 rounded-md border border-blue-800/40">
              <div className="flex items-center">
                <div className="bg-blue-500/20 rounded-full p-2 mr-3">
                  <Search className="h-5 w-5 text-blue-400" />
                </div>
                <div>
                  <p className="text-sm font-semibold text-blue-300">Znaleziono pokój!</p>
                  <p className="text-xs text-gray-300 mt-1">Aktywni użytkownicy: {foundRoomInfo?.users_count}</p>
                </div>
              </div>
            </div>
            
            <p className="text-sm text-gray-400">
              Aby dołączyć do pokoju, podaj klucz dostępu. Możesz go otrzymać od twórcy pokoju.
            </p>
            
            <div className="relative">
              <Input 
                placeholder="Klucz dostępu" 
                value={accessKey}
                onChange={(e) => setAccessKey(e.target.value)}
                className="pr-12"
                type="password"
              />
              <Lock className="absolute right-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-500" />
            </div>
            
            <div className="flex gap-2">
              <Button 
                onClick={resetJoinProcess} 
                variant="outline"
                className="flex-1"
              >
                Wróć
              </Button>
              <Button 
                onClick={handleConnect} 
                disabled={!accessKey}
                className="flex-1"
              >
                Połącz
              </Button>
            </div>
          </div>
        );
        
      case JoinSteps.CONNECTING:
        return (
          <div className="py-8 text-center">
            <div className="flex flex-col items-center justify-center">
              <RefreshCw className="h-10 w-10 text-blue-500 animate-spin mb-4" />
              <p>Łączenie z pokojem...</p>
              <p className="text-xs text-gray-400 mt-2">Trwa nawiązywanie bezpiecznego połączenia</p>
            </div>
          </div>
        );
        
      case JoinSteps.ERROR:
        return (
          <div className="space-y-4">
            <div className="p-4 bg-red-900/20 rounded-md border border-red-800/40">
              <div className="flex items-start">
                <div className="bg-red-500/20 rounded-full p-2 mr-3 mt-0.5">
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-5 w-5 text-red-400">
                    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                    <line x1="12" y1="9" x2="12" y2="13"></line>
                    <line x1="12" y1="17" x2="12.01" y2="17"></line>
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-semibold text-red-300">Wystąpił błąd</p>
                  <p className="text-xs text-gray-300 mt-1">{error}</p>
                </div>
              </div>
            </div>
            
            <Button 
              onClick={resetJoinProcess} 
              className="w-full"
            >
              Spróbuj ponownie
            </Button>
          </div>
        );
        
      default:
        return null;
    }
  };

  return (
    <div className="p-6 space-y-6">
      <div className="max-w-md mx-auto">
        <h2 className="text-2xl font-bold mb-6">Połącz się z pokojem</h2>
        
        <Card className="mb-6 border-gray-800 bg-gray-900/60">
          <CardHeader className="pb-3">
            <CardTitle className="text-xl flex items-center">
              <UserPlus className="h-5 w-5 mr-2 text-blue-400" />
              Utwórz nowy pokój
            </CardTitle>
            <CardDescription className="text-gray-400">
              Stwórz bezpieczną przestrzeń do komunikacji end-to-end
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button 
              onClick={handleCreate} 
              disabled={creatingRoom || joinStep === JoinSteps.CONNECTING}
              className="w-full mb-4 flex items-center justify-center gap-2"
            >
              {creatingRoom ? (
                <>
                  <RefreshCw className="h-4 w-4 animate-spin" />
                  <span>Tworzenie...</span>
                </>
              ) : (
                <>
                  <UserPlus className="h-4 w-4" />
                  <span>Utwórz nowy pokój</span>
                </>
              )}
            </Button>
            
            {createdRoomInfo && (
              <div className="mt-4 text-sm space-y-2">
                <div className="p-2 bg-gray-800/60 rounded-md flex justify-between items-center">
                  <span className="font-mono text-gray-300">ID: {createdRoomInfo.room_id}</span>
                  <Button 
                    variant="ghost" 
                    size="sm"
                    onClick={() => {
                      navigator.clipboard.writeText(createdRoomInfo.room_id);
                    }}
                    className="h-8 px-2"
                  >
                    <Clipboard className="h-4 w-4 mr-1" />
                    Kopiuj
                  </Button>
                </div>
                
                <div className="p-2 bg-gray-800/60 rounded-md flex justify-between items-center">
                  <span className="font-mono text-gray-300 truncate">Klucz: {createdRoomInfo.access_key}</span>
                  <Button 
                    variant="ghost" 
                    size="sm"
                    onClick={() => {
                      navigator.clipboard.writeText(createdRoomInfo.access_key);
                    }}
                    className="h-8 px-2"
                  >
                    <Clipboard className="h-4 w-4 mr-1" />
                    Kopiuj
                  </Button>
                </div>
                
                <div className="text-amber-400 text-xs mt-2">
                  Zapisz ten klucz dostępu! Jest wymagany do dołączenia do pokoju.
                </div>
              </div>
            )}
          </CardContent>
        </Card>
        
        <Card className="border-gray-800 bg-gray-900/60">
          <CardHeader className="pb-3">
            <CardTitle className="text-xl flex items-center">
              <Key className="h-5 w-5 mr-2 text-blue-400" />
              Dołącz do istniejącego pokoju
            </CardTitle>
            <CardDescription className="text-gray-400">
              Wprowadź ID i klucz dostępu, aby dołączyć
            </CardDescription>
          </CardHeader>
          <CardContent>
            {renderJoinStep()}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
