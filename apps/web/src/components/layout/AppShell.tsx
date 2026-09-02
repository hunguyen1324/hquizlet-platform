// AppShell — layout wrapper: Navbar + Sidebar + main content
import React from "react";
import { Navbar } from "./Navbar";
import { Sidebar } from "./Sidebar";
import { useAuth } from "../../features/auth/AuthContext";

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
    <div className="flex flex-col min-h-screen bg-[var(--background)]">
      <Navbar
        user={user}
        onSearch={(q) => {
          // Search navigates to dashboard with query
          onNavigate("dashboard");
        }}
        onCreateSet={() => onNavigate("editor")}
        onLogout={() => void logout()}
        onNavigate={onNavigate}
      />
      <div className="flex flex-1">
        <Sidebar
          currentView={currentView}
          onNavigate={onNavigate}
          onCreateFolder={onCreateFolder}
        />
        <main className="flex-1 min-w-0 overflow-y-auto p-4 md:p-8">
          {children}
        </main>
      </div>
    </div>
  );
}
