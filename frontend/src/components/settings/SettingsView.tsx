import React from "react";
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { 
  Fingerprint, 
  Copy, 
  Shield, 
  KeyRound, 
  RefreshCw, 
  AlertTriangle, 
  Info, 
  Server, 
  Network, 
  Lock
} from "lucide-react";

interface SettingsViewProps {
  identityFingerprint?: string;
  roomId?: string;
  listenPort?: number;
  securityInfo?: {
    kemAlgo?: string;
    sigAlgo?: string;
    symAlgo?: string;
  };
  peerFingerprints?: Record<string, string>;
  isRoomCreator?: boolean;
  accessKey?: string;
  onRegenerateAccessKey?: () => Promise<string>;
}

export function SettingsView({
  identityFingerprint = "Nie wygenerowano.",
  roomId,
  listenPort,
  securityInfo = {},
  peerFingerprints = {},
  isRoomCreator = false,
  accessKey = "",
  onRegenerateAccessKey
}: SettingsViewProps) {
  const { kemAlgo = "CRYSTALS-Kyber-1024", sigAlgo = "CRYSTALS-DILITHIUM-5", symAlgo = "ChaCha20-Poly1305" } = securityInfo;

  const [regenerating, setRegenerating] = React.useState(false);
  const [currentAccessKey, setCurrentAccessKey] = React.useState(accessKey);
  const [regenerateStatus, setRegenerateStatus] = React.useState("");

  // Aktualizuj klucz dostępu, gdy zmienia się prop
  React.useEffect(() => {
    setCurrentAccessKey(accessKey);
  }, [accessKey]);

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
      .catch(err => console.error("Nie udało się skopiować do schowka:", err));
  };
  
  const handleRegenerateKey = async () => {
    if (!onRegenerateAccessKey) return;
    
    try {
      setRegenerating(true);
      setRegenerateStatus("Generowanie nowego klucza...");
      const newKey = await onRegenerateAccessKey();
      setCurrentAccessKey(newKey);
      setRegenerateStatus("Nowy klucz wygenerowany pomyślnie!");
    } catch (error) {
      console.error("Błąd podczas regeneracji klucza:", error);
      setRegenerateStatus(`Błąd: ${error}`);
    } finally {
      setRegenerating(false);
    }
  };

  return (
    <div className="p-6 space-y-6 max-w-3xl mx-auto overflow-y-auto h-full">
      <h2 className="text-2xl font-bold mb-6 flex items-center">
        <Shield className="h-6 w-6 mr-2 text-blue-400" />
        Bezpieczeństwo i Informacje
      </h2>
      
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center">
            <Fingerprint className="h-5 w-5 mr-2 text-blue-400" />
            Twój Odcisk Palca
          </CardTitle>
          <CardDescription>
            Udostępnij ten odcisk palca rozmówcy, aby zweryfikować swoją tożsamość.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          <div className="relative">
            <div className="bg-gray-900/70 p-3 rounded-md font-mono text-sm break-all border border-gray-800">
              {identityFingerprint}
            </div>
            <Button 
              variant="ghost" 
              size="sm" 
              className="absolute top-2 right-2 h-8 w-8 p-0"
              onClick={() => copyToClipboard(identityFingerprint)}
            >
              <Copy className="h-4 w-4" />
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center">
            <Lock className="h-5 w-5 mr-2 text-blue-400" />
            Kryptografia Postkwantowa
          </CardTitle>
          <CardDescription>
            Algorytmy odporne na ataki z wykorzystaniem komputerów kwantowych.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ul className="space-y-3">
            <li className="flex justify-between items-center">
              <span className="text-gray-400 flex items-center">
                <KeyRound className="h-4 w-4 mr-2 text-gray-500" />
                Wymiana kluczy:
              </span>
              <code className="bg-gray-900/70 px-2 py-1 rounded font-mono border border-gray-800">{kemAlgo}</code>
            </li>
            <li className="flex justify-between items-center">
              <span className="text-gray-400 flex items-center">
                <Fingerprint className="h-4 w-4 mr-2 text-gray-500" />
                Podpisy cyfrowe:
              </span>
              <code className="bg-gray-900/70 px-2 py-1 rounded font-mono border border-gray-800">{sigAlgo}</code>
            </li>
            <li className="flex justify-between items-center">
              <span className="text-gray-400 flex items-center">
                <Lock className="h-4 w-4 mr-2 text-gray-500" />
                Szyfrowanie:
              </span>
              <code className="bg-gray-900/70 px-2 py-1 rounded font-mono border border-gray-800">{symAlgo}</code>
            </li>
          </ul>
        </CardContent>
      </Card>

      {isRoomCreator && (
        <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center">
            <KeyRound className="h-5 w-5 mr-2 text-blue-400" />
            Klucz Dostępu do Pokoju
          </CardTitle>
          <CardDescription>
            Ten klucz jest wymagany do dołączenia do tego pokoju.
          </CardDescription>
        </CardHeader>
          <CardContent className="space-y-2">
            <div className="relative">
            <div className="bg-gray-900/70 p-3 rounded-md font-mono text-sm break-all border border-gray-800">
              {currentAccessKey || "Brak klucza dostępu"}
            </div>
            <Button 
              variant="ghost" 
              size="sm" 
              className="absolute top-2 right-2 h-8 w-8 p-0"
              onClick={() => copyToClipboard(currentAccessKey)}
              disabled={!currentAccessKey}
            >
              <Copy className="h-4 w-4" />
            </Button>
            </div>
            <div className="flex justify-between items-center mt-4">
              <Button 
                variant="outline" 
                onClick={handleRegenerateKey}
                disabled={regenerating || !onRegenerateAccessKey}
                className="w-full flex items-center justify-center gap-2"
              >
                {regenerating ? (
                  <>
                    <RefreshCw className="h-4 w-4 animate-spin" />
                    <span>Generowanie...</span>
                  </>
                ) : (
                  <>
                    <KeyRound className="h-4 w-4" />
                    <span>Wygeneruj nowy klucz dostępu</span>
                  </>
                )}
              </Button>
            </div>
            {regenerateStatus && (
              <div className={cn(
                "mt-2 text-sm text-center",
                regenerateStatus.includes("Błąd") ? "text-red-400" : "text-green-400"
              )}>
                {regenerateStatus}
              </div>
            )}
            <div className="text-amber-400 text-xs mt-2 flex items-start">
              <AlertTriangle className="h-3.5 w-3.5 mr-1 mt-0.5 flex-shrink-0" />
              <span>Istniejący użytkownicy pozostaną połączeni, ale nowi będą potrzebować nowego klucza.</span>
            </div>
          </CardContent>
        </Card>
      )}
      
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center">
            <Network className="h-5 w-5 mr-2 text-blue-400" />
            Informacje o Połączeniu
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <div className="flex justify-between items-center">
            <span className="text-gray-400 flex items-center">
              <Info className="h-4 w-4 mr-2 text-gray-500" />
              ID Pokoju:
            </span>
            <div className="flex items-center">
              <code className="bg-gray-900/70 px-2 py-1 rounded font-mono border border-gray-800">
                {roomId || "N/A"}
              </code>
              {roomId && (
                <Button 
                  variant="ghost" 
                  size="sm" 
                  onClick={() => copyToClipboard(roomId)}
                  className="ml-1 h-7 w-7 p-0"
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
              )}
            </div>
          </div>
          <div className="flex justify-between items-center mt-3">
            <span className="text-gray-400 flex items-center">
              <Server className="h-4 w-4 mr-2 text-gray-500" />
              Nasłuchiwanie na porcie:
            </span>
            <code className="bg-gray-900/70 px-2 py-1 rounded font-mono border border-gray-800">
              {listenPort || "N/A"}
            </code>
          </div>
        </CardContent>
      </Card>

      {Object.keys(peerFingerprints).length > 0 && (
        <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Shield className="h-5 w-5 mr-2 text-blue-400" />
            Zweryfikowane Odciski Palców Rozmówców
          </CardTitle>
          <CardDescription>
            Te odciski palców zostały potwierdzone w trakcie handshake'a.
          </CardDescription>
        </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {Object.entries(peerFingerprints).map(([peerId, fingerprint]) => (
                <li key={peerId} className="mb-2">
                  <div className="text-sm font-medium mb-1 flex items-center">
                    <Fingerprint className="h-4 w-4 mr-1 text-blue-400" />
                    ID: {peerId.substring(0, 8)}...
                  </div>
                  <div className="bg-gray-900/70 p-2 rounded-md font-mono text-xs break-all border border-gray-800">
                    {fingerprint}
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
