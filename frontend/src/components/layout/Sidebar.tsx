import React from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { MessageSquare, Settings, Link2 } from "lucide-react";

type NavItem = {
  id: string;
  label: string;
  icon: React.ReactNode;
};

interface SidebarProps {
  activeView: string;
  onViewChange: (view: string) => void;
  connectionStatus?: {
    connected: boolean;
    secure: boolean;
  };
}

export function Sidebar({ 
  activeView, 
  onViewChange, 
  connectionStatus = { connected: false, secure: false } 
}: SidebarProps) {
  const navItems: NavItem[] = [
    { id: 'connect', label: 'Połącz', icon: <Link2 className="h-5 w-5" /> },
    { id: 'chat', label: 'Czat', icon: <MessageSquare className="h-5 w-5" /> },
    { id: 'settings', label: 'Ustawienia', icon: <Settings className="h-5 w-5" /> }
  ];

  const getStatusClass = () => {
    if (connectionStatus.secure) return "bg-green-900/20 text-green-500";
    if (connectionStatus.connected) return "bg-orange-900/20 text-orange-500";
    return "bg-red-900/20 text-red-500";
  };

  const getStatusText = () => {
    if (connectionStatus.secure) return "Bezpieczne";
    if (connectionStatus.connected) return "Łączenie...";
    return "Rozłączono";
  };

  return (
    <aside className="w-60 bg-gray-900 p-5 flex flex-col border-r border-gray-800">
      <h1 className="text-2xl font-bold mb-8 bg-gradient-to-r from-blue-500 to-violet-700 bg-clip-text text-transparent">
        ExecP2P
      </h1>
      
      <ul className="flex-grow space-y-2">
        {navItems.map((item) => (
          <li key={item.id}>
            <Button
              variant={activeView === item.id ? "default" : "ghost"}
              className={cn(
                "w-full justify-start text-left",
                activeView === item.id ? "bg-blue-600" : ""
              )}
              onClick={() => onViewChange(item.id)}
            >
              <span className="mr-3">{item.icon}</span>
              {item.label}
            </Button>
          </li>
        ))}
      </ul>
      
      <div className={cn("mt-4 p-3 rounded-lg text-center font-medium", getStatusClass())}>
        {getStatusText()}
      </div>
    </aside>
  );
}
