// main.tsx — Dev 3 [P2-WEB-01, P2-WEB-04]
// Entry point. Auth + study set flow gọi gateway API thật qua lib/api.

import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

import { AuthProvider, useAuth } from "./features/auth/AuthContext";
import { AuthScreen } from "./features/auth/AuthScreen";
import { Dashboard } from "./features/dashboard/Dashboard";
import { StudySetEditor } from "./features/study-sets/StudySetEditor";
import { StudyDetail } from "./features/study-sets/StudyDetail";
import { Folders } from "./features/folders/Folders";
import { studySetApi, flashcardApi, fetchHealth } from "./lib/api";
import type { StudySet, AppView, Flashcard, ServiceHealth, HealthStatus } from "./types";

function AppShell() {
  const { user, logout, token } = useAuth();
  const [view, setView] = useState<AppView>("dashboard");
  const [selectedSet, setSelectedSet] = useState<StudySet | null>(null);
  const [loadingSet, setLoadingSet] = useState(false);
  const [healthStatus, setHealthStatus] = useState<HealthStatus>("checking");
  const [services, setServices] = useState<ServiceHealth[]>([]);

  useEffect(() => {
    async function checkGateway() {
      try {
        const svcs = await fetchHealth();
        setServices(svcs);
        setHealthStatus(svcs.every((s) => s.status === "ok") ? "live" : "offline");
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
    setLoadingSet(true);
    try {
      const data = await studySetApi.get(token, id);
      setSelectedSet(data);
      setView("study");
    } finally {
      setLoadingSet(false);
    }
  }

  async function handleToggleStar(card: Flashcard) {
    await flashcardApi.toggleStar(token, card.id);
    if (selectedSet) await handleOpenSet(selectedSet.id);
  }

  async function handleDeleteSet() {
    if (!selectedSet) return;
    await studySetApi.delete(token, selectedSet.id);
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
        <button
          className="ghost-button"
          onClick={() => {
            setView("dashboard");
            setSelectedSet(null);
          }}
        >
          HQuizlet
        </button>
        <div className="user-menu">
          <button className="ghost-button" onClick={() => { setSelectedSet(null); setView("folders"); }}>Thư mục</button>
          <span>{user.name}</span>
          <button onClick={() => void logout()}>Logout</button>
        </div>
      </header>

      {loadingSet && (
        <div className="loading-overlay" aria-busy="true">
          <span>Đang tải học phần...</span>
        </div>
      )}

      {!loadingSet && view === "dashboard" && (
        <Dashboard
          healthStatus={healthStatus}
          onOpen={(id) => void handleOpenSet(id)}
          onCreate={() => {
            setSelectedSet(null);
            setView("editor");
          }}
        />
      )}

      {view === "editor" && (
        <StudySetEditor
          existingSet={selectedSet ?? undefined}
          onSave={handleSaveSet}
          onCancel={() => setView(selectedSet ? "study" : "dashboard")}
        />
      )}

      {!loadingSet && view === "study" && selectedSet && (
        <StudyDetail
          set={selectedSet}
          onEdit={() => setView("editor")}
          onDelete={() => void handleDeleteSet()}
          onBack={() => setView("dashboard")}
          onToggleStar={(card) => void handleToggleStar(card)}
        />
      )}

      {!loadingSet && view === "folders" && (
        <Folders
          onBack={() => setView("dashboard")}
          onOpenSet={(id) => void handleOpenSet(id)}
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
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
