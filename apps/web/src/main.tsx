// main.tsx — Dev 3 [P2-WEB-01, P2-WEB-04]
// Entry point. Auth + study set flow gọi gateway API thật qua lib/api.

import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes, useLocation, useNavigate, useParams } from "react-router-dom";
import "./styles.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, gcTime: 5 * 60_000, refetchOnWindowFocus: false } },
});

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
const AdminCatalog = React.lazy(() => import("./features/admin/AdminCatalog").then(m => ({default: m.AdminCatalog})));
import { QuizSessionsPage } from "./features/learning/QuizSessionsPage";
import { studySetApi, flashcardApi, fetchHealth, classApi } from "./lib/api";
import type { StudySet, AppView, Flashcard, ServiceHealth, HealthStatus, ClassDetail as ClassDetailType } from "./types";

import { AppShell } from "./components/layout/AppShell";
import { HomePage } from "./components/home/HomePage";

// Dashboard screens are code-split so recharts/date libs stay out of the initial bundle.
const PersonalDashboard = React.lazy(() =>
  import("./features/dashboard/PersonalDashboard").then((m) => ({ default: m.PersonalDashboard }))
);
const ClassDashboard = React.lazy(() =>
  import("./features/classes/ClassDashboard").then((m) => ({ default: m.ClassDashboard }))
);

// Phase 4: admin console chunk (AdminShell + its pages).
const AdminShell = React.lazy(() =>
  import("./features/admin/AdminShell").then((m) => ({ default: m.AdminShell }))
);
const AdminOverview = React.lazy(() =>
  import("./features/admin/AdminOverview").then((m) => ({ default: m.AdminOverview }))
);
const AdminUsers = React.lazy(() =>
  import("./features/admin/AdminUsers").then((m) => ({ default: m.AdminUsers }))
);
const AdminRoles = React.lazy(() =>
  import("./features/admin/AdminRoles").then((m) => ({ default: m.AdminRoles }))
);
const AdminAudit = React.lazy(() =>
  import("./features/admin/AdminAudit").then((m) => ({ default: m.AdminAudit }))
);
const AdminSystem = React.lazy(() =>
  import("./features/admin/AdminSystem").then((m) => ({ default: m.AdminSystem }))
);

// ADM-07: the legacy payments screen lives inside the console now.
function AdminPaymentsRoute() {
  const navigate = useNavigate();
  return <AdminPayments onBack={() => navigate("/admin")} />;
}

// Route wrapper: reads :id from the URL so the page itself stays router-agnostic.
function ClassDashboardRoute({
  onNavigate,
  onOpenSet,
}: {
  onNavigate: (view: string) => void;
  onOpenSet: (id: number) => void;
}) {
  const { id } = useParams();
  return <ClassDashboard classId={Number(id)} onBack={() => onNavigate("classes")} onOpenSet={onOpenSet} />;
}

const savedTheme = window.localStorage.getItem("hquizlet-theme");
const initialDark = savedTheme === "dark" || (savedTheme === null && window.matchMedia("(prefers-color-scheme: dark)").matches);
document.documentElement.classList.toggle("dark", initialDark);
document.documentElement.classList.toggle("light", !initialDark);

const VIEW_PATHS: Partial<Record<AppView, string>> = {
  home: "/", dashboard: "/library", folders: "/folders", live: "/live-quiz",
  classes: "/classes", activity: "/notifications", wallet: "/wallet",
  deposit: "/wallet/deposit", "create-type": "/create", editor: "/create/flashcards",
  "quiz-editor": "/create/quiz", "grammar-editor": "/create/grammar", "quiz-sessions": "/my/quiz-sessions",
  // Phase 2+: routes handled by react-router (not the legacy view switch)
  overview: "/dashboard", admin: "/admin",
};

function viewToPath(view: AppView) { return VIEW_PATHS[view] ?? "/"; }
function pathToView(path: string): AppView {
  const found = Object.entries(VIEW_PATHS).find(([, value]) => value === path);
  return (found?.[0] as AppView | undefined) ?? "home";
}

/** Views served by <Routes>; when undefined the legacy view switch renders instead. */
function routeViewForPath(path: string): AppView | undefined {
  if (path === "/dashboard") return "overview";
  if (path.startsWith("/admin")) return "admin";
  if (/^\/classes\/\d+\/dashboard$/.test(path)) return "class-dashboard";
  return undefined;
}

function RootApp() {
  const { user, logout, token } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const routeView = routeViewForPath(location.pathname);
  const [view, setView] = useState<AppView>("home");
 useEffect(()=>{if(!user){setSelectedSet(null);setSelectedClass(null);setView("home");}},[user?.id]);
  const [resumeSessionId, setResumeSessionId] = useState<string | undefined>();
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
      // react-router serves /dashboard and /admin/*; the legacy switch handles the rest.
      if (routeViewForPath(window.location.pathname)) return;
      const editMatch = window.location.pathname.match(/^\/study-sets\/(\d+)\/edit$/);
      if (editMatch) {
        void handleOpenSet(Number(editMatch[1]), false).then(() => setView("quiz-editor"));
        return;
      }
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

  async function handleOpenSet(id: number, pushHistory = true, sessionId?: string) {
    setResumeSessionId(sessionId);
    setLoadingSet(true);
    setNavigationError("");
    try {
      let data = await studySetApi.get(token, id);
      setSelectedSet(data);
      setView("study");
      if (pushHistory) navigate(`/study-sets/${id}`);
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

  function handleEditSet() {
    if (!selectedSet) return;
    setView(selectedSet.contentType === "quiz" ? "quiz-editor" : "editor");
    navigate(`/study-sets/${selectedSet.id}/edit`);
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

  async function handleOpenClass(id: number) {
    if (!token) return;
    try {
      const cls = await classApi.get(token, id);
      setSelectedClass(cls);
      setView("class-detail");
    } catch (error) {
      setNavigationError(error instanceof Error ? error.message : "Không mở được lớp.");
    }
  }

  // Navigation handler for AppShell sidebar/navbar
  function handleNavigate(viewName: string, pushHistory = true) {
    setSelectedSet(null);
    setSelectedClass(null);
    const nextView = viewName as AppView;
    setView(nextView);
    setNavigationError("");
    if (pushHistory) navigate(viewToPath(nextView));
  }

  return (
    <AppShell
      currentView={routeView ?? view}
      onNavigate={handleNavigate}
    >
      {/* Phase 2+: routes owned by react-router. */}
      <React.Suspense fallback={<div className="loading-overlay" aria-busy="true"><span>Đang tải dashboard...</span></div>}>
      <Routes>
        <Route
          path="/dashboard"
          element={
            <PersonalDashboard
              onOpenSet={(id) => void handleOpenSet(id)}
              onOpenClass={(id) => void handleOpenClass(id)}
              onNavigate={(v) => handleNavigate(v)}
            />
          }
        />
        <Route
          path="/classes/:id/dashboard"
          element={
            <ClassDashboardRoute
              onNavigate={(v) => handleNavigate(v)}
              onOpenSet={(id) => {
                handleNavigate("dashboard");
                void handleOpenSet(id);
              }}
            />
          }
        />
        {/* Phase 4: admin console. AdminShell guards rendering; the gateway and the
            service both re-check dashboard:ViewAdmin on every call. */}
        <Route path="/admin" element={<AdminShell />}>
          <Route index element={<AdminOverview />} />
          <Route path="users" element={<AdminUsers />} />
          <Route path="content" element={<AdminCatalog kind="content" />} />
          <Route path="classes" element={<AdminCatalog kind="classes" />} />
          <Route path="roles" element={<AdminRoles />} />
          <Route path="payments" element={<AdminPaymentsRoute />} />
          <Route path="audit" element={<AdminAudit />} />
          <Route path="system" element={<AdminSystem />} />
        </Route>
      </Routes>
      </React.Suspense>

      {/* Legacy view switch — kept for the screens that are not on routes yet (UI-01). */}
      {!routeView && (
      <>
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
          onEdit={handleEditSet}
          onDelete={() => void handleDeleteSet()}
          onBack={() => window.history.back()}
          onToggleStar={(card) => void handleToggleStar(card)}
          resumeSessionId={resumeSessionId}
          onManageSessions={() => handleNavigate("quiz-sessions")}
        />
      )}

      {!loadingSet && view === "folders" && (
        <Folders
          onBack={() => setView("dashboard")}
          onOpenSet={(id) => void handleOpenSet(id)}
        />
      )}

      {view === "quiz-sessions" && (
        <QuizSessionsPage
          studySetId={selectedSet?.id}
          onBack={() => setView(selectedSet ? "study" : "dashboard")}
          onResume={(setId, sessionId) => {
             void handleOpenSet(setId, true, sessionId);
          }}
        />
      )}

      {!loadingSet && view === "live" && (
        <LiveGame classId={Number(new URLSearchParams(location.search).get("classId")) || undefined} />
      )}

      {!loadingSet && view === "classes" && (
        <ClassList
          onCreate={() => setView("class-create")}
          onJoin={() => setView("class-join")}
          onSelect={(id) => void handleOpenClass(id)}
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
          onStartLive={() => {setView("live");navigate(`/live-quiz?classId=${selectedClass.id}`);}}
          onOpenSet={(id) => void handleOpenSet(id)}
          onOpenDashboard={() => navigate(`/classes/${selectedClass.id}/dashboard`)}
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
      </>
      )}
    </AppShell>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <RootApp />
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
