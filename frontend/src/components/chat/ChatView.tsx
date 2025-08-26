import React, { useState, useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { UserListTable, type ChatUser } from "./UserListTable";
import { RoomInfoTable } from "./RoomInfoTable";
import { Send, User, MessageSquare, AlertTriangle, Image, Mic, StopCircle, File } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

type Message = {
  id: string;
  sender: string;
  content: string;
  timestamp: string;
  isLocal: boolean;
  verified: boolean;
  type?: "text" | "image" | "audio" | "gif"; // Typ wiadomości
  mediaUrl?: string; // URL do pliku multimedialnego (zdjęcie, audio, gif)
  status?: "sent" | "pending" | "error"; // Status wysłania wiadomości
};

interface ChatViewProps {
  connected?: boolean;
  userID?: string;
  roomId?: string;
  accessKey?: string;
  isRoomCreator?: boolean;
  onRegenerateAccessKey?: () => Promise<string>;
}

export function ChatView({ 
  connected = false, 
  userID = "", 
  roomId, 
  accessKey, 
  isRoomCreator = false,
  onRegenerateAccessKey
}: ChatViewProps) {
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "system-1",
      sender: "System",
      content: "Witaj w ExecP2P Chat. Bezpieczny i kwantowo-odporny czat end-to-end.",
      timestamp: new Date().toISOString(),
      isLocal: false,
      verified: true,
      type: "text",
    },
  ]);
  const [inputValue, setInputValue] = useState("");
  const [nickname, setNickname] = useState(() => {
    // Wczytaj nickname z localStorage przy inicjalizacji
    const savedNickname = localStorage.getItem("execp2p_nickname");
    return savedNickname || "Użytkownik";
  });
  const [nicknameInput, setNicknameInput] = useState(() => {
    // Wczytaj nickname z localStorage przy inicjalizacji
    const savedNickname = localStorage.getItem("execp2p_nickname");
    return savedNickname || "Użytkownik";
  });
  const [users, setUsers] = useState<ChatUser[]>([]);
  const [userNicknames, setUserNicknames] = useState<Record<string, string>>({});
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [isRecording, setIsRecording] = useState(false);
  const [mediaRecorder, setMediaRecorder] = useState<MediaRecorder | null>(null);
  const [audioChunks, setAudioChunks] = useState<Blob[]>([]);
  
  // Przewijanie do najnowszej wiadomości
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);
  
  // Dodajemy bieżącego użytkownika do listy
  useEffect(() => {
    if (userID) {
      // Ustaw lokalnego użytkownika
      setUsers(prev => {
        // Sprawdź czy użytkownik już istnieje
        if (!prev.some(u => u.id === userID)) {
          return [...prev, { id: userID, nickname, isLocal: true }];
        }
        return prev.map(user => 
          user.isLocal ? { ...user, nickname } : user
        );
      });
    }
  }, [userID, nickname]);

  // Obsługa opuszczania pokoju - odinstalowanie eventów i czyszczenie
  const leaveRoom = () => {
    console.log("Opuszczanie pokoju - rozpoczynam procedurę...");
    
    try {
      // Natychmiast odinstaluj wszystkie listenery
      window.runtime.EventsOff('message:received');
      window.runtime.EventsOff('security:message');
      window.runtime.EventsOff('users:update');
      window.runtime.EventsOff('nickname:update');
      window.runtime.EventsOff('room:left');
      
      // Wyślij wiadomość o opuszczeniu pokoju i zamknij połączenie
      if (connected) {
        try {
          window.go.wailsbridge.Bridge.SendMessage(JSON.stringify({
            type: "user_left",
            content: `Użytkownik ${nickname} opuścił pokój`,
          }));
          console.log("Wiadomość o opuszczeniu pokoju wysłana");
        } catch (e) {
          console.error("Błąd wysyłania wiadomości o opuszczeniu:", e);
        }
        
        try {
          window.go.wailsbridge.Bridge.CloseConnection();
          console.log("Połączenie zamknięte");
        } catch (e) {
          console.error("Błąd zamykania połączenia:", e);
        }
      }
      
      // Całkowite czyszczenie stanu aplikacji
      setMessages([]);
      setUsers([]);
      
      // Bezpośrednio przekieruj na ekran główny
      console.log("Przekierowuję do ekranu głównego...");
      window.runtime.EventsEmit('room:left');
      window.runtime.EventsEmit('view:change', 'connect');
      
      // Awaryjny mechanizm przekierowania
      window.location.hash = '#connect';
      
    } catch (error) {
      console.error("Błąd podczas opuszczania pokoju:", error);
      // Awaryjne przekierowanie na ekran główny
      window.location.hash = '#connect';
      window.runtime.EventsEmit('view:change', 'connect');
    }
  };
  
  // Obsługa opuszczania pokoju przy zamykaniu okna
  useEffect(() => {
    const handleBeforeUnload = () => {
      // Wyślij wiadomość o opuszczeniu pokoju
      if (connected) {
        try {
          window.go.wailsbridge.Bridge.SendMessage(JSON.stringify({
            type: "user_left",
            content: `Użytkownik ${nickname} opuścił pokój`,
          }));
        } catch (error) {
          console.error("Błąd podczas wysyłania wiadomości o opuszczeniu pokoju:", error);
        }
      }
    };
    
    window.addEventListener('beforeunload', handleBeforeUnload);
    
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [connected, nickname]);
  
  // Nasłuchiwanie zdarzenia opuszczenia pokoju
  useEffect(() => {
    const handleRoomLeft = () => {
      if (window.location.hash === '#chat') {
        // Jeśli jesteśmy na ekranie czatu, przekieruj do ekranu połączenia
        window.runtime.EventsEmit('view:change', 'connect');
      }
    };
    
    window.runtime.EventsOn('room:left', handleRoomLeft);
    
    return () => {
      window.runtime.EventsOff('room:left');
    };
  }, []);
  
  useEffect(() => {
    if (!connected) return;
    
    // Nasłuchiwanie zdarzeń z Wails
    window.runtime.EventsOn('message:received', (data: any) => {
      const msgData = data as {
        sender: string;
        message: string;
        timestamp: string;
        isLocal: boolean;
        verified: boolean;
        type?: string;
        mediaUrl?: string;
      };
      
      // Obsługa specjalnej wiadomości o opuszczeniu pokoju
      if (msgData.type === "user_left") {
        // Dodaj komunikat systemowy o opuszczeniu pokoju
        setMessages(prev => [
          ...prev,
          {
            id: `user-left-${Date.now()}`,
            sender: "System",
            content: msgData.message,
            timestamp: typeof msgData.timestamp === 'string' 
              ? msgData.timestamp 
              : new Date().toISOString(),
            isLocal: false,
            verified: true,
            type: "text",
          }
        ]);
        return;
      }
      
      // Pobierz nickname nadawcy z mapy nicków, jeśli istnieje
      const senderNickname = userNicknames[msgData.sender] || msgData.sender;
      
      setMessages(prev => [
        ...prev,
        {
          id: `remote-${Date.now()}`,
          sender: senderNickname,
          content: msgData.message,
          timestamp: typeof msgData.timestamp === 'string' 
            ? msgData.timestamp 
            : new Date(msgData.timestamp).toISOString(),
          isLocal: false, // Zawsze ustawiamy na false, aby wiadomości były widoczne dla wszystkich
          verified: msgData.verified,
          type: (msgData.type as "text" | "image" | "audio" | "gif") || "text",
          mediaUrl: msgData.mediaUrl,
          status: "sent", // Wiadomości odebrane zawsze mają status "sent"
        }
      ]);
    });
    
    // Nasłuchiwanie komunikatów bezpieczeństwa
    window.runtime.EventsOn('security:message', (message: string) => {
      setMessages(prev => [
        ...prev,
        {
          id: `security-${Date.now()}`,
          sender: "System",
          content: message,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    });
    
    // Nasłuchiwanie aktualizacji użytkowników
    window.runtime.EventsOn('users:update', (userList: any) => {
      // Zaktualizuj listę użytkowników, zachowując lokalnego użytkownika
      setUsers(prev => {
        const localUser = prev.find(u => u.isLocal);
        // Filtruj nową listę, aby nie zawierała lokalnego użytkownika
        const remoteUsers = userList
          .filter((u: any) => u.id !== userID)
          .map((u: any) => ({
            ...u,
            // Użyj zapisanego nickname'a jeśli jest dostępny
            nickname: userNicknames[u.id] || u.nickname,
            isLocal: false
          }));
        
        // Połącz lokalnego użytkownika z listą zdalnych użytkowników
        return localUser ? [...remoteUsers, localUser] : remoteUsers;
      });
    });
    
    // Nasłuchiwanie aktualizacji nicków
    window.runtime.EventsOn('nickname:update', (data: { sender: string, nickname: string }) => {
      // Aktualizuj mapę nicków
      setUserNicknames(prev => ({
        ...prev,
        [data.sender]: data.nickname
      }));
      
      // Aktualizuj listę użytkowników
      setUsers(prev => 
        prev.map(user => 
          user.id === data.sender ? { ...user, nickname: data.nickname } : user
        )
      );
      
      // Dodaj komunikat systemowy o zmianie nicku
      setMessages(prev => [
        ...prev,
        {
          id: `nick-update-${Date.now()}`,
          sender: "System",
          content: `Użytkownik zmienił nazwę na: ${data.nickname}`,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    });
    
    return () => {
      window.runtime.EventsOff('message:received');
      window.runtime.EventsOff('security:message');
      window.runtime.EventsOff('users:update');
      window.runtime.EventsOff('nickname:update');
    };
  }, [connected, userNicknames, userID]);
  
  // Funkcja do sprawdzania i żądania uprawnień do mikrofonu
  const requestMicrophonePermission = async () => {
    try {
      // Pokaż dialog z przyciskiem do żądania uprawnień
      setMessages(prev => [
        ...prev,
        {
          id: `mic-info-${Date.now()}`,
          sender: "System",
          content: `Kliknij przycisk poniżej, aby udzielić dostępu do mikrofonu:`,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
      
      // Dodaj przycisk do udzielenia dostępu (będzie renderowany jako element React)
      setMessages(prev => [
        ...prev,
        {
          id: `mic-button-${Date.now()}`,
          sender: "System",
          content: "<mic-permission-button>",
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
      
      return false; // Zwracamy false, co oznacza że jeszcze nie mamy uprawnień
    } catch (error) {
      console.error("Błąd podczas żądania uprawnień mikrofonu:", error);
      return false;
    }
  };
  
  // Funkcja do rozpoczęcia nagrywania audio - bezpośrednia próba nagrywania
  const startRecording = async () => {
    console.log("Rozpoczynam nagrywanie...");
    
    try {
      // Bezpośrednio żądamy dostępu do mikrofonu i rozpoczynamy nagrywanie
      const stream = await navigator.mediaDevices.getUserMedia({ 
        audio: true,
        video: false
      });
      
      console.log("Dostęp do mikrofonu uzyskany, konfiguracja rekordera...");
      
      const recorder = new MediaRecorder(stream);
      const chunks: Blob[] = [];
      
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) {
          chunks.push(e.data);
        }
      };
      
      recorder.onstop = () => {
        console.log("Nagrywanie zakończone, przetwarzanie audio...");
        const audioBlob = new Blob(chunks, { type: 'audio/webm' });
        setAudioChunks(chunks);
        sendAudioMessage(audioBlob);
        // Zatrzymaj wszystkie ścieżki audio po zakończeniu nagrywania
        stream.getTracks().forEach(track => track.stop());
      };
      
      setMediaRecorder(recorder);
      setIsRecording(true);
      recorder.start();
      
      // Powiadomienie o rozpoczęciu nagrywania
      setMessages(prev => [
        ...prev,
        {
          id: `mic-start-${Date.now()}`,
          sender: "System",
          content: `Nagrywanie rozpoczęte. Kliknij "Zatrzymaj nagrywanie" aby zakończyć.`,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    } catch (error: any) {
      console.error("Błąd podczas uzyskiwania dostępu do mikrofonu:", error);
      // Bardziej przyjazny komunikat dla użytkownika
      let errorMessage = "Błąd dostępu do mikrofonu";
      
      if (error.name === "NotAllowedError") {
        errorMessage = "Nie udzielono dostępu do mikrofonu. Sprawdź ustawienia przeglądarki i zezwól na dostęp do mikrofonu.";
      } else if (error.name === "NotFoundError") {
        errorMessage = "Nie znaleziono mikrofonu. Sprawdź, czy urządzenie jest podłączone.";
      } else if (error.name === "NotReadableError") {
        errorMessage = "Mikrofon jest zajęty lub nie odpowiada. Zamknij inne aplikacje, które mogą go używać.";
      }
      
      setMessages(prev => [
        ...prev,
        {
          id: `error-${Date.now()}`,
          sender: "System",
          content: errorMessage,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    }
  };
  
  // Funkcja do zatrzymania nagrywania
  const stopRecording = () => {
    if (mediaRecorder && isRecording) {
      mediaRecorder.stop();
      setIsRecording(false);
    }
  };
  
  // Funkcja do wysyłania wiadomości audio
  const sendAudioMessage = async (audioBlob: Blob) => {
    if (!connected) return;
    
    try {
      // Konwertuj Blob na Base64
      const reader = new FileReader();
      reader.readAsDataURL(audioBlob);
      reader.onloadend = async () => {
        const base64data = reader.result as string;
        
        // Dodaj lokalną wiadomość audio do widoku z początkowym statusem "pending"
        const newMessage: Message = {
          id: `local-audio-${Date.now()}`,
          sender: nickname,
          content: "Wiadomość głosowa",
          timestamp: new Date().toISOString(),
          isLocal: false, // Ustawiamy na false, aby była widoczna dla wszystkich
          verified: true,
          type: "audio",
          mediaUrl: base64data,
          status: "pending", // Początkowy status "pending"
        };
        
        // Najpierw dodajemy wiadomość lokalnie, aby była od razu widoczna
        setMessages(prev => [...prev, newMessage]);
        
        // Przygotuj strukturę JSON dla wiadomości
        const audioMessage = {
          type: "audio",
          content: "Wiadomość głosowa",
          mediaUrl: base64data
        };
        
        console.log("Wysyłanie wiadomości audio");
        
        // Wysyłamy wiadomość przez Wails
        try {
          const result = await window.go.wailsbridge.Bridge.SendMessage(JSON.stringify(audioMessage));
          
          // Sprawdź wynik wysyłania
          if (result && result.includes("buforowana")) {
            // Wiadomość została buforowana, pozostawiamy status "pending"
            console.log("Wiadomość audio buforowana");
          } else {
            // Wiadomość wysłana pomyślnie, aktualizujemy status na "sent"
            setMessages(prev => 
              prev.map(msg => 
                msg.id === newMessage.id ? { ...msg, status: "sent" } : msg
              )
            );
            console.log("Wiadomość audio wysłana pomyślnie");
          }
        } catch (error) {
          console.error("Błąd podczas wysyłania wiadomości audio:", error);
          // Pokaż komunikat o błędzie i oznacz wiadomość jako błędną
          // Najpierw aktualizujemy status istniejącej wiadomości
          setMessages(prev => 
            prev.map(msg => 
              msg.id === newMessage.id ? { ...msg, status: "error" } : msg
            )
          );
          
          // Następnie dodajemy komunikat o błędzie
          setMessages(prev => [
            ...prev,
            {
              id: `error-audio-${Date.now()}`,
              sender: "System",
              content: `Błąd wysyłania wiadomości audio: ${error}`,
              timestamp: new Date().toISOString(),
              isLocal: false,
              verified: true,
              type: "text",
            }
          ]);
        }
      };
    } catch (error) {
      console.error("Błąd podczas wysyłania wiadomości audio:", error);
      setMessages(prev => [
        ...prev,
        {
          id: `error-${Date.now()}`,
          sender: "System",
          content: `Błąd wysyłania: ${error}`,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    }
  };
  
  // Funkcja do obsługi wyboru pliku
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    
    const file = files[0];
    const reader = new FileReader();
    
    reader.onloadend = async () => {
      const base64data = reader.result as string;
      
      let messageType: "image" | "gif" = "image";
      if (file.type.includes("gif")) {
        messageType = "gif";
      }
      
      // Dodaj lokalną wiadomość ze zdjęciem do widoku z początkowym statusem "pending"
      const newMessage: Message = {
        id: `local-media-${Date.now()}`,
        sender: nickname,
        content: file.name,
        timestamp: new Date().toISOString(),
        isLocal: false, // Ustawiamy na false, aby była widoczna dla wszystkich
        verified: true,
        type: messageType,
        mediaUrl: base64data,
        status: "pending", // Początkowy status "pending"
      };
      
      try {
        // Wysyłamy wiadomość przez Wails z dodatkowymi polami
        const message = JSON.stringify({
          type: messageType,
          content: file.name,
          mediaUrl: base64data
        });
        
        // Dodajemy wiadomość do widoku przed wysłaniem
        setMessages(prev => [...prev, newMessage]);
        
        // Wysyłamy wiadomość przez Wails
        const result = await window.go.wailsbridge.Bridge.SendMessage(message);
        
        // Sprawdź wynik wysyłania
        if (result && result.includes("buforowana")) {
          // Wiadomość została buforowana, pozostawiamy status "pending"
          console.log("Wiadomość multimedialna buforowana");
        } else {
          // Wiadomość wysłana pomyślnie, aktualizujemy status na "sent"
          setMessages(prev => 
            prev.map(msg => 
              msg.id === newMessage.id ? { ...msg, status: "sent" } : msg
            )
          );
        }
      } catch (error) {
        console.error("Błąd podczas wysyłania multimediów:", error);
        // Oznacz wiadomość jako błędną
        setMessages(prev => 
          prev.map(msg => 
            msg.id === newMessage.id ? { ...msg, status: "error" } : msg
          )
        );
        // Dodaj komunikat o błędzie
        setMessages(prev => [
          ...prev,
          {
            id: `error-${Date.now()}`,
            sender: "System",
            content: `Błąd wysyłania: ${error}`,
            timestamp: new Date().toISOString(),
            isLocal: false,
            verified: true,
            type: "text",
          }
        ]);
      }
    };
    
    if (file.type.includes("image")) {
      reader.readAsDataURL(file);
    } else {
      // Informacja o nieobsługiwanym typie pliku
      setMessages(prev => [
        ...prev,
        {
          id: `error-${Date.now()}`,
          sender: "System",
          content: `Nieobsługiwany typ pliku: ${file.type}`,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    }
  };
  
  const handleSendMessage = async () => {
    if (!inputValue.trim() || !connected) return;
    
    // Tworzymy kopię wiadomości do wysłania
    const messageToSend = inputValue.trim();
    const messageJson = JSON.stringify({
      type: "text",
      content: messageToSend
    });
    
    // Dodaj lokalną wiadomość do widoku z początkowym statusem "pending"
    const newMessage: Message = {
      id: `local-${Date.now()}`,
      sender: nickname,
      content: inputValue,
      timestamp: new Date().toISOString(),
      isLocal: false, // Zmieniamy na false, aby wiadomość była widoczna dla wszystkich
      verified: true,
      type: "text",
      status: "pending", // Wiadomość początkowo oczekująca
    };
    
    // Najpierw dodajemy wiadomość lokalnie, aby natychmiast ją wyświetlić
    setMessages(prev => [...prev, newMessage]);
    setInputValue("");
    
    try {
      // Wysyłamy wiadomość przez Wails
      const result = await window.go.wailsbridge.Bridge.SendMessage(messageJson);
      
      // Sprawdź wynik wysyłania
      if (result && result.includes("buforowana")) {
        // Wiadomość została buforowana, pozostawiamy status "pending"
        console.log("Wiadomość buforowana:", messageToSend);
      } else {
        // Wiadomość wysłana pomyślnie, aktualizujemy status na "sent"
        setMessages(prev => 
          prev.map(msg => 
            msg.id === newMessage.id ? { ...msg, status: "sent" } : msg
          )
        );
      }
    } catch (error) {
      console.error("Błąd podczas wysyłania wiadomości:", error);
      // Oznacz wiadomość jako błędną
      setMessages(prev => 
        prev.map(msg => 
          msg.id === newMessage.id ? { ...msg, status: "error" } : msg
        )
      );
      // Dodaj komunikat o błędzie
      setMessages(prev => [
        ...prev,
        {
          id: `error-${Date.now()}`,
          sender: "System",
          content: `Błąd wysyłania: ${error}`,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    }
  };
  
  // Funkcja do ustawiania nicka
  const handleNicknameChange = () => {
    const newNickname = nicknameInput.trim();
    if (newNickname && newNickname !== nickname) {
      setNickname(newNickname);
      
      // Zapisz nickname w localStorage dla późniejszego użycia
      localStorage.setItem("execp2p_nickname", newNickname);
      
      // Aktualizuj użytkownika w liście
      setUsers(prev => 
        prev.map(user => 
          user.isLocal ? { ...user, nickname: newNickname } : user
        )
      );
      
      // Wysyłamy aktualizację do innych użytkowników
      if (connected) {
        try {
          window.go.wailsbridge.Bridge.UpdateNickname(newNickname);
          // Dodaj wiadomość systemową o zmianie nazwy
          setMessages(prev => [
            ...prev,
            {
              id: `nick-update-self-${Date.now()}`,
              sender: "System",
              content: `Zmieniłeś swoją nazwę na: ${newNickname}`,
              timestamp: new Date().toISOString(),
              isLocal: false,
              verified: true,
              type: "text",
            }
          ]);
        } catch (error) {
          console.error("Błąd podczas aktualizacji nickname:", error);
        }
      }
    }
  };
  
  // Ta funkcja nie jest już potrzebna, ponieważ teraz używamy bezpośredniego podejścia w startRecording
  const handleMicrophonePermission = async () => {
    try {
      startRecording();
    } catch (error: any) {
      console.error("Błąd podczas uzyskiwania dostępu do mikrofonu:", error);
      
      // Bardziej przyjazny komunikat dla użytkownika
      let errorMessage = "Błąd dostępu do mikrofonu";
      
      if (error.name === "NotAllowedError") {
        errorMessage = "Nie udzielono dostępu do mikrofonu. Sprawdź ustawienia przeglądarki i zezwól na dostęp do mikrofonu.";
      } else if (error.name === "NotFoundError") {
        errorMessage = "Nie znaleziono mikrofonu. Sprawdź, czy urządzenie jest podłączone.";
      } else if (error.name === "NotReadableError") {
        errorMessage = "Mikrofon jest zajęty lub nie odpowiada. Zamknij inne aplikacje, które mogą go używać.";
      }
      
      setMessages(prev => [
        ...prev,
        {
          id: `error-${Date.now()}`,
          sender: "System",
          content: errorMessage,
          timestamp: new Date().toISOString(),
          isLocal: false,
          verified: true,
          type: "text",
        }
      ]);
    }
  };
  
  // Funkcja renderująca zawartość wiadomości w zależności od typu
  const renderMessageContent = (msg: Message) => {
    // Specjalne renderowanie dla przycisku uprawnień mikrofonu
    if (msg.content === "<mic-permission-button>") {
      return (
        <Button 
          onClick={handleMicrophonePermission}
          className="bg-blue-500 hover:bg-blue-600"
        >
          Udziel dostępu do mikrofonu
        </Button>
      );
    }
    
    switch (msg.type) {
      case "image":
      case "gif":
        // Sprawdź czy mediaUrl istnieje
        if (msg.mediaUrl) {
          return (
            <img 
              src={msg.mediaUrl} 
              alt={msg.content}
              className="max-w-full rounded-md"
              style={{ maxHeight: "200px" }}
            />
          );
        } else {
          // Jeśli nie ma mediaUrl, pokaż informację o braku
          return (
            <div>
              <p>{msg.content}</p>
              <p className="text-xs text-red-400">Nie można wyświetlić obrazu</p>
            </div>
          );
        }
      case "audio":
        return (
          <audio 
            controls
            src={msg.mediaUrl}
            className="max-w-full mt-1"
          >
            Twoja przeglądarka nie obsługuje elementu audio.
          </audio>
        );
      case "text":
      default:
        return msg.content;
    }
  };
  
  // Nasłuchiwanie zdarzenia opuszczenia pokoju
  useEffect(() => {
    const handleRoomLeft = () => {
      if (window.location.hash === '#chat') {
        // Jeśli jesteśmy na ekranie czatu, przekieruj do ekranu połączenia
        window.runtime.EventsEmit('view:change', 'connect');
      }
    };
    
    window.runtime.EventsOn('room:left', handleRoomLeft);
    
    return () => {
      window.runtime.EventsOff('room:left');
    };
  }, []);
  
  // Sprawdzenie, czy faktycznie mamy dostęp do pokoju
  if (!roomId || (!connected && !isRoomCreator)) {
    return (
      <div className="flex flex-col h-full">
        <div className="bg-gray-900/60 px-6 py-4 border-b border-gray-800">
          <h2 className="text-xl font-semibold flex items-center">
            <MessageSquare className="h-5 w-5 mr-2 text-blue-400" />
            Czat
          </h2>
        </div>
        <div className="flex-1 flex items-center justify-center p-6">
          <Card className="max-w-md w-full border-gray-800 bg-gray-900/60">
            <CardHeader className="pb-2">
              <CardTitle className="text-xl flex items-center">
                <AlertTriangle className="h-5 w-5 mr-2 text-amber-500" />
                Brak aktywnego pokoju
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-gray-400 mb-6">
                Aby rozpocząć rozmowę, przejdź do zakładki "Połącz" i utwórz nowy pokój lub dołącz do istniejącego.
              </p>
              <Button 
                onClick={() => window.runtime.EventsEmit('view:change', 'connect')}
                className="w-full"
              >
                Przejdź do Połącz
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>
    );
  }
    
    return (
      <div className="flex flex-col h-full">
        <div className="bg-gray-900/60 px-6 py-4 border-b border-gray-800 flex justify-between items-center">
          <h2 className="text-xl font-semibold flex items-center">
            <MessageSquare className="h-5 w-5 mr-2 text-blue-400" />
            Czat
            {!isRoomCreator && (
              <Button
                onClick={leaveRoom}
                className="ml-3 bg-red-500 hover:bg-red-600 text-xs py-1"
                size="sm"
              >
                Opuść pokój
              </Button>
            )}
          </h2>
          <div className="flex items-center space-x-2">
            <User className="h-4 w-4 text-gray-400" />
            <span className="text-sm text-gray-400">Twoja nazwa:</span>
            <div className="flex items-center space-x-1">
              <Input 
                value={nicknameInput}
                onChange={(e) => setNicknameInput(e.target.value)}
                className="w-36 h-8 text-sm bg-gray-800/60 border-gray-700"
                placeholder="Podaj swoją nazwę"
              />
              <Button
                onClick={handleNicknameChange}
                className="h-8 px-2 py-0"
                size="sm"
              >
                Zapisz
              </Button>
            </div>
          </div>
        </div>
      
      <div className="flex-1 flex overflow-hidden">
        <div className="flex-1 overflow-y-auto px-6 py-4">
        <div className="space-y-4">
          {messages.map((msg) => (
            <div 
              key={msg.id}
              className={cn(
                "max-w-[80%]",
                msg.sender === nickname ? "ml-auto" : "mr-auto"
              )}
            >
              {msg.sender !== nickname && msg.sender !== "System" && (
                <div className="text-sm text-gray-400 mb-1 flex items-center">
                  <User className="h-3 w-3 mr-1 opacity-70" />
                  {msg.sender} {msg.verified && <span className="text-green-500 ml-1">(zweryfikowany)</span>}
                </div>
              )}
              <div 
                className={cn(
                  "px-4 py-2 rounded-lg",
                  msg.sender === nickname 
                    ? "bg-blue-600/90 text-white rounded-br-none shadow-sm" 
                    : msg.sender === "System"
                      ? "bg-gray-700/90 text-gray-200 shadow-sm"
                      : "bg-gray-800/90 text-gray-200 rounded-bl-none shadow-sm"
                )}
              >
                {renderMessageContent(msg)}
              </div>
              <div className="text-xs mt-1 text-right flex justify-end items-center gap-1">
                <span className="text-gray-500">
                  {new Date(msg.timestamp).toLocaleTimeString()}
                </span>
                {msg.sender === nickname && msg.status && (
                  <span className={
                    msg.status === "sent" 
                      ? "text-green-500" 
                      : msg.status === "pending" 
                        ? "text-orange-500" 
                        : "text-red-500"
                  }>
                    {msg.status === "sent" 
                      ? "Wysłano" 
                      : msg.status === "pending" 
                        ? "Oczekuje" 
                        : "Błąd"}
                  </span>
                )}
              </div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>
        </div>
        <div className="w-64 flex-shrink-0 border-l border-gray-800 p-4">
          <UserListTable users={users} />
          <RoomInfoTable 
            roomId={roomId}
            accessKey={accessKey}
            isRoomCreator={isRoomCreator}
            onRegenerateAccessKey={onRegenerateAccessKey}
          />
        </div>
      </div>
      
      <div className="bg-gray-900/60 px-6 py-4 border-t border-gray-800">
        <form 
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            handleSendMessage();
          }}
        >
          <div className="flex gap-2">
            <Input
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              placeholder={connected ? "Wpisz wiadomość..." : "Oczekiwanie na połączenie..."}
              disabled={!connected}
              className="flex-1 bg-gray-800/60 border-gray-700"
            />
            <Button 
              type="submit" 
              disabled={!connected || !inputValue.trim()}
              className="gap-2"
            >
              <Send className="h-4 w-4" />
              Wyślij
            </Button>
          </div>
          
          {/* Przyciski multimediów */}
          {connected && (
            <div className="flex gap-2 mt-2">
              <Button 
                type="button"
                onClick={() => fileInputRef.current?.click()}
                variant="outline"
                className="gap-2"
              >
                <Image className="h-4 w-4" />
                Zdjęcie/GIF
              </Button>
              
              <input 
                type="file" 
                ref={fileInputRef}
                onChange={handleFileSelect}
                accept="image/*"
                style={{ display: 'none' }}
              />
              
              {!isRecording ? (
                <Button 
                  type="button"
                  onClick={startRecording}
                  variant="outline"
                  className="gap-2"
                >
                  <Mic className="h-4 w-4" />
                  Nagrywaj
                </Button>
              ) : (
                <Button 
                  type="button"
                  onClick={stopRecording}
                  variant="destructive"
                  className="gap-2 animate-pulse"
                >
                  <StopCircle className="h-4 w-4" />
                  Zatrzymaj nagrywanie
                </Button>
              )}
            </div>
          )}
        </form>
      </div>
    </div>
  );
}
