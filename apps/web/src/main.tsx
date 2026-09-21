// main.tsx — Dev 3 [P2-WEB-01, P2-WEB-04]
// Entry point. Auth + study set flow gọi gateway API thật qua lib/api.

import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

import { AuthProvider, useAuth } from "./features/auth/AuthContext";
import { AuthScreen } from "./features/auth/AuthScreen";
import { StudySetEditor } from "./features/study-sets/StudySetEditor";
import { StudyDetail } from "./features/study-sets/StudyDetail";
import { CreateTypeSelector } from "./features/study-sets/CreateTypeSelector";
import { QuizSetEditor } from "./features/study-sets/QuizSetEditor";
import { GrammarSetEditor } from "./features/study-sets/GrammarSetEditor";
import { Folders } from "./features/folders/Folders";
import { LiveGame } from "./features/live/LiveGame";
import { ClassList } from "./features/classes/ClassList";
import { CreateClass } from "./features/classes/CreateClass";
import { ClassDetail as ClassDetailView } from "./features/classes/ClassDetail";
import { EditClass } from "./features/classes/EditClass";
import { JoinClass } from "./features/classes/JoinClass";
import { ActivityFeed } from "./features/activity/ActivityFeed";
import { WalletPage } from "./features/wallet/WalletPage";
import { DepositPage } from "./features/payment/DepositPage";
import { AdminPayments } from "./features/admin/AdminPayments";
import { studySetApi, flashcardApi, fetchHealth, classApi, quizQuestionApi } from "./lib/api";
import type { StudySet, AppView, Flashcard, ServiceHealth, HealthStatus, ClassDetail as ClassDetailType } from "./types";

import { AppShell } from "./components/layout/AppShell";
import { HomePage } from "./components/home/HomePage";

const VIEW_PATHS: Partial<Record<AppView, string>> = {
  home: "/", dashboard: "/library", folders: "/folders", live: "/live-quiz",
  classes: "/classes", activity: "/notifications", wallet: "/wallet",
  deposit: "/wallet/deposit", "create-type": "/create", editor: "/create/flashcards",
  "quiz-editor": "/create/quiz", "grammar-editor": "/create/grammar",
};

function viewToPath(view: AppView) { return VIEW_PATHS[view] ?? "/"; }
function pathToView(path: string): AppView {
  const found = Object.entries(VIEW_PATHS).find(([, value]) => value === path);
  return (found?.[0] as AppView | undefined) ?? "home";
}

function RootApp() {
  const { user, logout, token } = useAuth();
  const [view, setView] = useState<AppView>("home");
  const [selectedSet, setSelectedSet] = useState<StudySet | null>(null);
  const [selectedClass, setSelectedClass] = useState<ClassDetailType | null>(null);
  const [loadingSet, setLoadingSet] = useState(false);
  const [navigationError, setNavigationError] = useState("");
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

  useEffect(() => {
    if (!user) return;
    const restoreUrl = () => {
      const match = window.location.pathname.match(/^\/study-sets\/(\d+)$/);
      if (match) void handleOpenSet(Number(match[1]), false);
      else handleNavigate(pathToView(window.location.pathname), false);
    };
    restoreUrl();
    window.addEventListener("popstate", restoreUrl);
    return () => window.removeEventListener("popstate", restoreUrl);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.id]);

  if (!user) {
    return (
      <AuthScreen
        healthStatus={healthStatus}
        liveCount={services.filter((s) => s.status === "ok").length}
        serviceCount={services.length || 4}
      />
    );
  }

  async function handleOpenSet(id: number, pushHistory = true) {
    setLoadingSet(true);
    setNavigationError("");
    try {
      let data = await studySetApi.get(token, id);
      if (data.contentType === "quiz") {
        const quizQuestions = await quizQuestionApi.list(token, id);
        data = { ...data, quizQuestions };
      }
      setSelectedSet(data);
      setView("study");
      if (pushHistory) window.history.pushState({}, "", `/study-sets/${id}`);
    } catch (error) {
      setNavigationError(error instanceof Error ? error.message : "Không mở được học phần.");
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

  // Navigation handler for AppShell sidebar/navbar
  function handleNavigate(viewName: string, pushHistory = true) {
    setSelectedSet(null);
    setSelectedClass(null);
    const nextView = viewName as AppView;
    setView(nextView);
    setNavigationError("");
    if (pushHistory) window.history.pushState({}, "", viewToPath(nextView));
  }

  return (
    <AppShell
      currentView={view}
      onNavigate={handleNavigate}
    >
      {loadingSet && (
        <div className="loading-overlay" aria-busy="true">
          <span>Đang tải học phần...</span>
        </div>
      )}
      {navigationError && <div className="message message--error" role="alert">{navigationError}</div>}

      {/* NEW: HomePage — shown for home & dashboard views */}
      {(view === "home" || view === "dashboard") && !loadingSet && (
        <HomePage
          onOpenSet={(id) => void handleOpenSet(id)}
          onNavigate={handleNavigate}
        />
      )}

      {view === "create-type" && (
        <CreateTypeSelector
          onSelect={(type) => {
            if (type === "flashcard") setView("editor");
            else if (type === "quiz") setView("quiz-editor");
            else if (type === "grammar") setView("grammar-editor");
          }}
          onCancel={() => setView("dashboard")}
        />
      )}

      {view === "editor" && (
        <StudySetEditor
          existingSet={selectedSet ?? undefined}
          onSave={handleSaveSet}
          onCancel={() => setView(selectedSet ? "study" : "dashboard")}
        />
      )}

      {view === "quiz-editor" && (
        <QuizSetEditor
          existingSetId={selectedSet?.id}
          onSave={() => {
            if (selectedSet) void handleOpenSet(selectedSet.id);
            else setView("dashboard");
          }}
          onCancel={() => setView(selectedSet ? "study" : "create-type")}
        />
      )}

      {view === "grammar-editor" && (
        <GrammarSetEditor
          onSave={() => setView("dashboard")}
          onCancel={() => setView("create-type")}
        />
      )}

      {!loadingSet && view === "study" && selectedSet && (
        <StudyDetail
          set={selectedSet}
          onEdit={() => setView(selectedSet.contentType === "quiz" ? "quiz-editor" : "editor")}
          onDelete={() => void handleDeleteSet()}
          onBack={() => window.history.back()}
          onToggleStar={(card) => void handleToggleStar(card)}
        />
      )}

      {!loadingSet && view === "folders" && (
        <Folders
          onBack={() => setView("dashboard")}
          onOpenSet={(id) => void handleOpenSet(id)}
        />
      )}

      {!loadingSet && view === "live" && (
        <LiveGame />
      )}

      {!loadingSet && view === "classes" && (
        <ClassList
          onCreate={() => setView("class-create")}
          onJoin={() => setView("class-join")}
          onSelect={async (id) => {
            if (!token) return;
            try {
              const cls = await classApi.get(token, id);
              setSelectedClass(cls);
              setView("class-detail");
            } catch {}
          }}
        />
      )}

      {view === "class-create" && (
        <CreateClass
          onCreated={(cls) => { setSelectedClass(cls); setView("class-detail"); }}
          onCancel={() => setView("classes")}
        />
      )}

      {view === "class-join" && (
        <JoinClass
          onJoined={async (resp) => {
            if (token) {
              try {
                const cls = await classApi.get(token, resp.classId);
                setSelectedClass(cls);
              } catch {
                setSelectedClass({
                  id: resp.classId, name: resp.className, description: "", inviteCode: "",
                  memberCount: 0, studySetCount: 0, myRole: resp.myRole as any, maxMembers: 0,
                  createdAt: resp.joinedAt, updatedAt: resp.joinedAt,
                } as ClassDetailType);
              }
            }
            setView("class-detail");
          }}
          onCancel={() => setView("classes")}
        />
      )}

      {!loadingSet && view === "class-detail" && selectedClass && (
        <ClassDetailView
          classId={selectedClass.id}
          onStartLive={() => setView("live")}
          onOpenSet={(id) => void handleOpenSet(id)}
          onBack={() => { setSelectedClass(null); setView("classes"); }}
          onEdit={(cls) => { setSelectedClass(cls); setView("class-edit"); }}
          onDelete={async () => {
            if (!token || !selectedClass) return;
            if (confirm("Bạn có chắc muốn xóa lớp này?")) {
              await classApi.delete(token, selectedClass.id);
              setSelectedClass(null);
              setView("classes");
            }
          }}
        />
      )}

      {view === "class-edit" && selectedClass && (
        <EditClass
          cls={selectedClass}
          onSaved={(cls) => { setSelectedClass(cls); setView("class-detail"); }}
          onCancel={() => setView("class-detail")}
        />
      )}

      {!loadingSet && view === "wallet" && (
        <WalletPage
          onDeposit={() => setView("deposit")}
          onBack={() => setView("dashboard")}
        />
      )}

      {view === "deposit" && (
        <DepositPage
          onBack={() => setView("wallet")}
          onSuccess={() => setView("wallet")}
        />
      )}

      {!loadingSet && view === "activity" && (
        <ActivityFeed onBack={() => setView("dashboard")} />
      )}

      {!loadingSet && view === "admin-payments" && (
        <AdminPayments onBack={() => setView("dashboard")} />
      )}
    </AppShell>
  );
}

function App() {
  return (
    <AuthProvider>
      <RootApp />
    </AuthProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
