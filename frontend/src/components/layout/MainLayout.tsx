import React from "react";
import { Sidebar } from "./Sidebar";

type MainLayoutProps = {
  children: React.ReactNode;
  activeView: string;
  onViewChange: (view: string) => void;
  connectionStatus?: {
    connected: boolean;
    secure: boolean;
  };
};

export function MainLayout({
  children,
  activeView,
  onViewChange,
  connectionStatus = { connected: false, secure: false }
}: MainLayoutProps) {
  return (
    <div className="flex h-screen bg-gray-950 text-gray-100 overflow-hidden">
      <Sidebar 
        activeView={activeView} 
        onViewChange={onViewChange} 
        connectionStatus={connectionStatus} 
      />
      <main className="flex-1 overflow-hidden">
        {children}
      </main>
    </div>
  );
}
