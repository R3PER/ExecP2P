import React, { useState } from "react";
import { cn } from "@/lib/utils";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Copy, RefreshCw, KeyRound, AlertTriangle, Info } from "lucide-react";

interface RoomInfoTableProps {
  roomId?: string;
  accessKey?: string;
  isRoomCreator: boolean;
  className?: string;
  onRegenerateAccessKey?: () => Promise<string>;
}

export function RoomInfoTable({ 
  roomId, 
  accessKey, 
  isRoomCreator,
  className,
  onRegenerateAccessKey 
}: RoomInfoTableProps) {
  const [regenerating, setRegenerating] = useState(false);
  const [currentAccessKey, setCurrentAccessKey] = useState(accessKey);
  const [regenerateStatus, setRegenerateStatus] = useState("");

  // Aktualizuj klucz dostępu, gdy zmienia się prop
  React.useEffect(() => {
    setCurrentAccessKey(accessKey);
  }, [accessKey]);

  const copyToClipboard = (text: string | undefined) => {
    if (!text) return;
    
    navigator.clipboard.writeText(text)
      .then(() => {
        setRegenerateStatus("Skopiowano do schowka");
        setTimeout(() => setRegenerateStatus(""), 2000);
      })
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
      setTimeout(() => setRegenerateStatus(""), 3000);
    } catch (error) {
      console.error("Błąd podczas regeneracji klucza:", error);
      setRegenerateStatus(`Błąd: ${error}`);
    } finally {
      setRegenerating(false);
    }
  };

  return (
    <Card className={cn("w-full mt-4", className)}>
      <CardHeader className="py-3">
        <CardTitle className="text-lg flex justify-between items-center">
          <span className="flex items-center">
            <Info className="h-4 w-4 text-blue-400 mr-2" />
            Informacje o pokoju {isRoomCreator ? (
              <span className="text-sm ml-1 text-blue-400 font-normal">(Twórca)</span>
            ) : (
              <span className="text-sm ml-1 text-gray-400 font-normal">(Gość)</span>
            )}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="px-3 py-2">
        <div className="space-y-3">
          {roomId && (
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-400">ID Pokoju:</span>
              <div className="flex items-center">
                <span className="font-mono text-xs bg-gray-800 px-2 py-1 rounded">
                  {roomId.substring(0, 10)}...
                </span>
                <Button 
                  variant="ghost" 
                  size="sm"
                  className="ml-1 h-7 w-7 p-0"
                  onClick={() => copyToClipboard(roomId)}
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          )}
          
          {/* Informacja o kluczu dostępu */}
          <div className="space-y-2">
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-400">Klucz dostępu:</span>
                <div className="flex items-center">
                  <span className="font-mono text-xs bg-gray-800 px-2 py-1 rounded truncate max-w-[120px]">
                    {currentAccessKey || "Brak klucza"}
                  </span>
                <Button 
                  variant="ghost" 
                  size="sm"
                  className="ml-1 h-7 w-7 p-0"
                  onClick={() => copyToClipboard(currentAccessKey)}
                  disabled={!currentAccessKey}
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
                </div>
              </div>
              
              {/* Tylko twórca pokoju może regenerować klucz */}
              {isRoomCreator && (
                <>
                  <Button 
                    variant="default" 
                    size="default"
                    className="w-full mt-2 bg-blue-600 hover:bg-blue-700 flex items-center justify-center gap-2"
                    onClick={handleRegenerateKey}
                    disabled={regenerating || !onRegenerateAccessKey}
                  >
                    {regenerating ? (
                      <>
                        <RefreshCw className="h-4 w-4 animate-spin" />
                        <span>Generowanie...</span>
                      </>
                    ) : (
                      <>
                        <KeyRound className="h-4 w-4" />
                        <span>Generuj nowy klucz dostępu</span>
                      </>
                    )}
                  </Button>
                  
                  {regenerateStatus && (
                    <div className={cn(
                      "text-xs text-center mt-1",
                      regenerateStatus.includes("Błąd") ? "text-red-400" : "text-green-400"
                    )}>
                      {regenerateStatus}
                    </div>
                  )}
                  
                  <div className="text-amber-400 text-xs mt-2 flex items-start">
                    <AlertTriangle className="h-3.5 w-3.5 mr-1 mt-0.5 flex-shrink-0" />
                    <span>Po regeneracji klucza nowi użytkownicy będą potrzebować nowego klucza do dołączenia.</span>
                  </div>
                </>
              )}
            </div>
          
          {!isRoomCreator && accessKey && (
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-400 flex items-center">
                  <KeyRound className="h-3.5 w-3.5 mr-1 text-gray-500" />
                  Klucz dostępu:
                </span>
                <div className="flex items-center">
                <span className="font-mono text-xs bg-gray-800 px-2 py-1 rounded truncate max-w-[120px]">
                  {accessKey}
                </span>
                <Button 
                  variant="ghost" 
                  size="sm"
                  className="ml-1 h-7 w-7 p-0"
                  onClick={() => copyToClipboard(accessKey)}
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
