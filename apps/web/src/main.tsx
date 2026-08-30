// main.tsx - Dev 3
// Entry point. Auth + study set flow gọi gateway API thật.

import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

import { AuthProvider, useAuth, apiFetch } from "./features/auth/AuthContext";
import { AuthScreen } from "./features/auth/AuthScreen";
import { Dashboard } from "./features/dashboard/Dashboard";
import { StudySetEditor } from "./features/study-sets/StudySetEditor";
import { StudyDetail } from "./features/study-sets/StudyDetail";
import type { StudySet, ServiceHealth, HealthStatus, AppView, Flashcard } from "./types";

const gatewayUrl = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

function AppShell() {
  const { user, logout, token } = useAuth();
  const [view, setView] = useState<AppView>("dashboard");
  const [selectedSet, setSelectedSet] = useState<StudySet | null>(null);
  const [healthStatus, setHealthStatus] = useState<HealthStatus>("checking");
  const [services, setServices] = useState<ServiceHealth[]>([]);

  useEffect(() => {
    async function checkGateway() {
      try {
        const res = await fetch(`${gatewayUrl}/healthz/services`);
        const data = (await res.json()) as { services: ServiceHealth[] };
        setServices(data.services);
        setHealthStatus(data.services.every((s) => s.status === "ok") ? "live" : "offline");
      } catch {
        setHealthStatus("offline");
      }
    }
    void checkGateway();
    const id = window.setInterval(checkGateway, 5000);
    return () => window.clearInterval(id);
  }, []);

  if (!user) {
    return (
      <AuthScreen
        healthStatus={healthStatus}
        liveCount={services.filter((s) => s.status === "ok").length}
        serviceCount={services.length || 4}
      />
    );
  }

  async function handleOpenSet(id: number) {
    const data = await apiFetch<StudySet>(`/v1/study-sets/${id}`, token);
    setSelectedSet(data);
    setView("study");
  }

  async function handleToggleStar(card: Flashcard) {
    await apiFetch(`/v1/flashcards/${card.id}/star`, token, { method: "POST" });
    if (selectedSet) await handleOpenSet(selectedSet.id);
  }

  async function handleDeleteSet() {
    if (!selectedSet) return;
    await apiFetch(`/v1/study-sets/${selectedSet.id}`, token, { method: "DELETE" });
    setSelectedSet(null);
    setView("dashboard");
  }

  function handleSaveSet(saved: StudySet) {
    setSelectedSet(saved);
    setView("study");
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <button className="ghost-button" onClick={() => { setView("dashboard"); setSelectedSet(null); }}>
          HQuizlet
        </button>
        <div className="user-menu">
          <span>{user.name}</span>
          <button onClick={() => void logout()}>Logout</button>
        </div>
      </header>

      {view === "dashboard" && (
        <Dashboard
          healthStatus={healthStatus}
          onOpen={(id) => void handleOpenSet(id)}
          onCreate={() => { setSelectedSet(null); setView("editor"); }}
        />
      )}

      {view === "editor" && (
        <StudySetEditor
          existingSet={selectedSet ?? undefined}
          onSave={handleSaveSet}
          onCancel={() => setView(selectedSet ? "study" : "dashboard")}
        />
      )}

      {view === "study" && selectedSet && (
        <StudyDetail
          set={selectedSet}
          onEdit={() => setView("editor")}
          onDelete={() => void handleDeleteSet()}
          onBack={() => setView("dashboard")}
          onToggleStar={(card) => void handleToggleStar(card)}
        />
      )}
    </main>
  );
}

function App() {
  return (
    <AuthProvider>
      <AppShell />
    </AuthProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode><App /></React.StrictMode>
);
