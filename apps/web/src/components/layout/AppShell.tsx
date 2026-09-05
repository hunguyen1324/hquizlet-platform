// AppShell — unified layout: Navbar + Sidebar + main
import React from "react";
import { Navbar } from "./Navbar";
import { Sidebar } from "./Sidebar";
import { useAuth } from "../../features/auth/AuthContext";
import "./layout.css";

type AppShellProps = {
  currentView: string;
  onNavigate: (view: string) => void;
  onCreateFolder?: () => void;
  children: React.ReactNode;
};

export function AppShell({ currentView, onNavigate, onCreateFolder, children }: AppShellProps) {
  const { user, logout } = useAuth();
  if (!user) return null;

  return (
    <div className="hq-shell">
      <Navbar
        user={user}
        onSearch={() => onNavigate("dashboard")}
        onCreateSet={() => onNavigate("editor")}
        onLogout={() => void logout()}
        onNavigate={onNavigate}
      />
      <div className="hq-body">
        <Sidebar
          currentView={currentView}
          onNavigate={onNavigate}
          onCreateFolder={onCreateFolder}
        />
        <main className="hq-main">
          {children}
        </main>
      </div>
    </div>
  );
}
