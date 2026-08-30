// main.tsx - Dev 3 (FE-CORE-01: Feature folder refactor)
// Entry point. All logic is now in features/* and components/*.
// Auth state managed by AuthProvider (FE-CORE-04: Protected layout).

import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

import { AuthProvider, useAuth } from "./features/auth/AuthContext";
import { AuthScreen } from "./features/auth/AuthScreen";
import { Dashboard } from "./features/dashboard/Dashboard";
import { StudySetEditor } from "./features/study-sets/StudySetEditor";
import { StudyDetail } from "./features/study-sets/StudyDetail";
import type { StudySet, ServiceHealth, HealthStatus, AppView } from "./types";

const gatewayUrl = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

function AppShell() {
  const { user, logout } = useAuth();
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

  // Not logged in → auth screen (FE-CORE-04)
  if (!user) {
    return (
      <AuthScreen
        healthStatus={healthStatus}
        liveCount={services.filter((s) => s.status === "ok").length}
        serviceCount={services.length || 4}
      />
    );
  }

  function handleOpenSet(id: number) {
    // TODO (FE-CORE-07): fetch real set from Dev 2 API
    // For now find in mock
    import("./lib/mock/mockData").then(({ MOCK_SETS }) => {
      const found = MOCK_SETS.find((s) => s.id === id) ?? null;
      setSelectedSet(found);
      setView("study");
    });
  }

  function handleSaveSet(saved: StudySet) {
    setSelectedSet(saved);
    setView("study");
  }

  function handleDeleteSet() {
    setSelectedSet(null);
    setView("dashboard");
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <button className="ghost-button" onClick={() => { setView("dashboard"); setSelectedSet(null); }}>
          HQuizlet
        </button>
        <div className="user-menu">
          <span>{user.name}</span>
          <button onClick={logout}>Logout</button>
        </div>
      </header>

      {view === "dashboard" && (
        <Dashboard
          healthStatus={healthStatus}
          onOpen={handleOpenSet}
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
          onDelete={handleDeleteSet}
          onBack={() => setView("dashboard")}
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
