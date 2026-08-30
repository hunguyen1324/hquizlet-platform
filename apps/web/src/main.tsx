import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type HealthStatus = "checking" | "live" | "offline";
type AuthMode = "login" | "register";
type View = "dashboard" | "editor" | "study";
type StudyMode = "dashboard" | "flashcards" | "learn" | "test" | "match";
type User = { id: number; name: string; email: string; role: string };
type AuthResponse = { authenticated: boolean; token: string; user: User };
type ServiceHealth = { name: string; url: string; status: string };
type StudySet = { id: number; title: string; description: string; flashcards?: Flashcard[] };
type Flashcard = { id: number; studySetId: number; term: string; definition: string; starred: boolean };
type DraftCard = { key: string; id?: number; term: string; definition: string; starred?: boolean };

const gatewayUrl = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";
const tokenKey = "hquizlet.sessionToken";

function App() {
  const [healthStatus, setHealthStatus] = useState<HealthStatus>("checking");
  const [services, setServices] = useState<ServiceHealth[]>([]);
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [token, setToken] = useState(() => localStorage.getItem(tokenKey) ?? "");
  const [user, setUser] = useState<User | null>(null);
  const [name, setName] = useState("Demo User");
  const [email, setEmail] = useState("demo@hquizlet.local");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("Dang nhap de vao dashboard hoc tap.");
  const [sets, setSets] = useState<StudySet[]>([]);
  const [selectedSet, setSelectedSet] = useState<StudySet | null>(null);
  const [view, setView] = useState<View>("dashboard");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [draftCards, setDraftCards] = useState<DraftCard[]>(() => emptyDraftCards());
  const [studyMode, setStudyMode] = useState<StudyMode>("dashboard");
  const [cardIndex, setCardIndex] = useState(0);
  const [showBack, setShowBack] = useState(false);
  const [answer, setAnswer] = useState("");
  const [lastResult, setLastResult] = useState("");
  const [loading, setLoading] = useState(false);

  const cards = selectedSet?.flashcards ?? [];
  const activeCard = cards[cardIndex] ?? null;
  const liveCount = services.filter((service) => service.status === "ok").length;

  useEffect(() => {
    async function checkGateway() {
      try {
        const response = await fetch(`${gatewayUrl}/healthz/services`);
        const data = (await response.json()) as { services: ServiceHealth[] };
        setServices(data.services);
        setHealthStatus(data.services.every((service) => service.status === "ok") ? "live" : "offline");
      } catch {
        setHealthStatus("offline");
      }
    }
    void checkGateway();
    const interval = window.setInterval(checkGateway, 5000);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => { if (token) void loadMe(token); }, [token]);
  useEffect(() => { if (user) void loadSets(); }, [user]);

  async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await fetch(`${gatewayUrl}${path}`, {
      ...init,
      headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...init.headers },
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error ?? `Request failed with ${response.status}`);
    return body as T;
  }

  async function loadMe(sessionToken: string) {
    try {
      const response = await fetch(`${gatewayUrl}/v1/auth/me`, { headers: { Authorization: `Bearer ${sessionToken}` } });
      const body = await response.json();
      if (body.authenticated) { setUser(body.user); return; }
      logoutLocal();
    } catch { logoutLocal(); }
  }

  async function submitAuth(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoading(true);
    try {
      const body = authMode === "login" ? { email, password } : { name, email, password };
      const response = await api<AuthResponse>(authMode === "login" ? "/v1/auth/login" : "/v1/auth/register", { method: "POST", body: JSON.stringify(body) });
      localStorage.setItem(tokenKey, response.token);
      setToken(response.token);
      setUser(response.user);
      setMessage(`Da dang nhap: ${response.user.email}`);
    } catch (error) { setMessage(error instanceof Error ? error.message : "Auth failed."); }
    finally { setLoading(false); }
  }

  async function logout() { if (token) await api("/v1/auth/logout", { method: "POST" }).catch(() => undefined); logoutLocal(); }
  function logoutLocal() { localStorage.removeItem(tokenKey); setToken(""); setUser(null); setSelectedSet(null); setView("dashboard"); }

  async function loadSets() {
    setLoading(true);
    try { setSets(await api<StudySet[]>("/v1/study-sets")); }
    catch (error) { setMessage(error instanceof Error ? error.message : "Khong tai duoc study sets."); }
    finally { setLoading(false); }
  }

  async function openSet(id: number) {
    const response = await api<StudySet>(`/v1/study-sets/${id}`);
    setSelectedSet(response);
    setTitle(response.title);
    setDescription(response.description);
    setDraftCards(toDraftCards(response.flashcards ?? []));
    setCardIndex(0);
    setShowBack(false);
    setStudyMode("dashboard");
    setView("study");
  }

  function startCreateSet() {
    setSelectedSet(null);
    setTitle("");
    setDescription("");
    setDraftCards(emptyDraftCards());
    setMessage("Nhap tieu de va them it nhat mot cap thuat ngu/dinh nghia.");
    setView("editor");
  }

  function startEditSet() {
    if (!selectedSet) return;
    setTitle(selectedSet.title);
    setDescription(selectedSet.description);
    setDraftCards(toDraftCards(selectedSet.flashcards ?? []));
    setView("editor");
  }

  function updateDraftCard(key: string, field: "term" | "definition", value: string) {
    setDraftCards((current) => current.map((card) => card.key === key ? { ...card, [field]: value } : card));
  }

  function addDraftCard() {
    setDraftCards((current) => [...current, newDraftCard()]);
  }

  function removeDraftCard(key: string) {
    setDraftCards((current) => current.length === 1 ? current : current.filter((card) => card.key !== key));
  }

  async function saveEditor(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cleanCards = draftCards
      .map((card) => ({ ...card, term: card.term.trim(), definition: card.definition.trim() }))
      .filter((card) => card.term || card.definition);

    if (!title.trim()) { setMessage("Can nhap tieu de bo the."); return; }
    if (cleanCards.some((card) => !card.term || !card.definition)) { setMessage("Moi the can co du thuat ngu va dinh nghia."); return; }

    setLoading(true);
    try {
      const payload = JSON.stringify({ title, description });
      const saved = selectedSet
        ? await api<StudySet>(`/v1/study-sets/${selectedSet.id}`, { method: "PUT", body: payload })
        : await api<StudySet>("/v1/study-sets", { method: "POST", body: payload });

      const existingIds = new Set((selectedSet?.flashcards ?? []).map((card) => card.id));
      for (const card of cleanCards) {
        if (card.id) {
          existingIds.delete(card.id);
          await api(`/v1/flashcards/${card.id}`, { method: "PUT", body: JSON.stringify({ term: card.term, definition: card.definition }) });
        } else {
          await api(`/v1/study-sets/${saved.id}/flashcards`, { method: "POST", body: JSON.stringify({ term: card.term, definition: card.definition }) });
        }
      }
      for (const id of existingIds) await api(`/v1/flashcards/${id}`, { method: "DELETE" });

      await loadSets();
      await openSet(saved.id);
      setMessage(`Da luu hoc phan "${saved.title}".`);
    } catch (error) { setMessage(error instanceof Error ? error.message : "Khong luu duoc hoc phan."); }
    finally { setLoading(false); }
  }

  async function deleteSet(id: number) { await api(`/v1/study-sets/${id}`, { method: "DELETE" }); setSelectedSet(null); setView("dashboard"); await loadSets(); }
  async function toggleStar(card: Flashcard) { await api(`/v1/flashcards/${card.id}/star`, { method: "POST" }); if (selectedSet) await openSet(selectedSet.id); }
  function nextCard(step: number) { if (!cards.length) return; setCardIndex((current) => (current + step + cards.length) % cards.length); setShowBack(false); setAnswer(""); setLastResult(""); }
  function checkAnswer() { if (!activeCard) return; const ok = answer.trim().toLowerCase() === activeCard.definition.trim().toLowerCase(); setLastResult(ok ? "Dung roi." : `Chua dung. Dap an: ${activeCard.definition}`); }

  if (!user) return <AuthScreen authMode={authMode} setAuthMode={setAuthMode} name={name} setName={setName} email={email} setEmail={setEmail} password={password} setPassword={setPassword} submitAuth={submitAuth} loading={loading} message={message} healthStatus={healthStatus} liveCount={liveCount} serviceCount={services.length || 4} />;

  return <main className="app-shell"><header className="topbar"><button className="ghost-button" onClick={() => setView("dashboard")}>HQuizlet</button><div className="user-menu"><span>{user.name}</span><button onClick={() => void logout()}>Logout</button></div></header>{view === "dashboard" ? <Dashboard sets={sets} healthStatus={healthStatus} loading={loading} onCreate={startCreateSet} onOpen={(id) => void openSet(id)} onReload={() => void loadSets()} /> : null}{view === "editor" ? <Editor title={title} setTitle={setTitle} description={description} setDescription={setDescription} draftCards={draftCards} updateDraftCard={updateDraftCard} addDraftCard={addDraftCard} removeDraftCard={removeDraftCard} saveEditor={saveEditor} cancel={() => setView(selectedSet ? "study" : "dashboard")} loading={loading} isEditing={Boolean(selectedSet)} /> : null}{view === "study" && selectedSet ? <StudyDetail selectedSet={selectedSet} cards={cards} studyMode={studyMode} setStudyMode={setStudyMode} activeCard={activeCard} showBack={showBack} setShowBack={setShowBack} cardIndex={cardIndex} nextCard={nextCard} answer={answer} setAnswer={setAnswer} lastResult={lastResult} checkAnswer={checkAnswer} startEditSet={startEditSet} deleteSet={() => void deleteSet(selectedSet.id)} toggleStar={(card) => void toggleStar(card)} /> : null}<p className="message global-message">{message}</p></main>;
}

function AuthScreen(props: { authMode: AuthMode; setAuthMode: (mode: AuthMode) => void; name: string; setName: (value: string) => void; email: string; setEmail: (value: string) => void; password: string; setPassword: (value: string) => void; submitAuth: (event: React.FormEvent<HTMLFormElement>) => void; loading: boolean; message: string; healthStatus: HealthStatus; liveCount: number; serviceCount: number }) {
  return <main className="auth-shell"><section className="auth-hero"><p className="eyebrow">HQuizlet Platform</p><h1>Hoc bang flashcard, backend Go, du lieu PostgreSQL.</h1><p>Dang nhap hoac dang ky de tao study set, them flashcard va thu Flashcards, Learn, Test, Match.</p><div className="service-strip"><strong>{props.liveCount} / {props.serviceCount}</strong><span>services live</span><span className={`badge badge--${props.healthStatus}`}>{props.healthStatus}</span></div></section><section className="panel auth-card"><div className="tabs"><button className={props.authMode === "login" ? "tab active" : "tab"} onClick={() => props.setAuthMode("login")} type="button">Dang nhap</button><button className={props.authMode === "register" ? "tab active" : "tab"} onClick={() => props.setAuthMode("register")} type="button">Dang ky</button></div><form className="stack" onSubmit={props.submitAuth}>{props.authMode === "register" ? <label>Ten<input value={props.name} onChange={(event) => props.setName(event.target.value)} /></label> : null}<label>Email<input type="email" value={props.email} onChange={(event) => props.setEmail(event.target.value)} /></label><label>Mat khau<input type="password" value={props.password} onChange={(event) => props.setPassword(event.target.value)} /></label><button className="primary-button" disabled={props.loading} type="submit">{props.authMode === "login" ? "Dang nhap" : "Tao tai khoan"}</button></form><p className="message">{props.message}</p></section></main>;
}

function Dashboard(props: { sets: StudySet[]; healthStatus: HealthStatus; loading: boolean; onCreate: () => void; onOpen: (id: number) => void; onReload: () => void }) {
  return <><section className="page-heading"><div><p className="eyebrow">Dashboard</p><h1>Thu vien hoc phan</h1><p>Quan ly cac bo the va tiep tuc hoc tu du lieu PostgreSQL.</p></div><button className="primary-button" onClick={props.onCreate}>Tao hoc phan</button></section><section className="summary-grid"><Metric label="Study sets" value={props.sets.length.toString()} /><Metric label="Backend" value={props.healthStatus} /><Metric label="Status" value={props.loading ? "loading" : "ready"} /></section><section className="set-grid">{props.sets.length === 0 ? <div className="empty-panel"><h2>Chua co hoc phan</h2><p>Tao bo the dau tien voi thuat ngu va dinh nghia.</p><button className="primary-button" onClick={props.onCreate}>Tao hoc phan</button></div> : null}{props.sets.map((set) => <button className="set-card" key={set.id} onClick={() => props.onOpen(set.id)}><span>{set.description || "No description"}</span><strong>{set.title}</strong><small>Mo hoc phan</small></button>)}</section></>;
}

function Editor(props: { title: string; setTitle: (value: string) => void; description: string; setDescription: (value: string) => void; draftCards: DraftCard[]; updateDraftCard: (key: string, field: "term" | "definition", value: string) => void; addDraftCard: () => void; removeDraftCard: (key: string) => void; saveEditor: (event: React.FormEvent<HTMLFormElement>) => void; cancel: () => void; loading: boolean; isEditing: boolean }) {
  return <form className="create-page" onSubmit={props.saveEditor}><section className="create-header"><div><p className="eyebrow">{props.isEditing ? "Sua hoc phan" : "Tao hoc phan moi"}</p><h1>{props.isEditing ? "Cap nhat the hoc" : "Tao the hoc"}</h1></div><div className="header-actions"><button className="ghost-button" type="button" onClick={props.cancel}>Huy</button><button className="primary-button" disabled={props.loading} type="submit">{props.isEditing ? "Luu thay doi" : "Tao hoc phan"}</button></div></section><section className="create-meta"><label>Tieu de<input autoFocus placeholder="Vi du: English Vocabulary Unit 1" value={props.title} onChange={(event) => props.setTitle(event.target.value)} /></label><label>Mo ta<textarea placeholder="Mo ta ngan ve noi dung bo the" value={props.description} onChange={(event) => props.setDescription(event.target.value)} /></label></section><section className="cards-editor"><div className="cards-editor-heading"><div><p className="eyebrow">Cards</p><h2>Thuat ngu va dinh nghia</h2></div><button className="secondary-button" type="button" onClick={props.addDraftCard}>+ Them the</button></div>{props.draftCards.map((card, index) => <article className="draft-card" key={card.key}><div className="draft-index">{index + 1}</div><label>Thuat ngu<input placeholder="apple" value={card.term} onChange={(event) => props.updateDraftCard(card.key, "term", event.target.value)} /></label><label>Dinh nghia<input placeholder="qua tao" value={card.definition} onChange={(event) => props.updateDraftCard(card.key, "definition", event.target.value)} /></label><button className="icon-button" type="button" onClick={() => props.removeDraftCard(card.key)} aria-label="Xoa the">×</button></article>)}<button className="add-row" type="button" onClick={props.addDraftCard}>+ Them mot the nua</button></section></form>;
}

function StudyDetail(props: { selectedSet: StudySet; cards: Flashcard[]; studyMode: StudyMode; setStudyMode: (mode: StudyMode) => void; activeCard: Flashcard | null; showBack: boolean; setShowBack: (value: boolean | ((value: boolean) => boolean)) => void; cardIndex: number; nextCard: (step: number) => void; answer: string; setAnswer: (value: string) => void; lastResult: string; checkAnswer: () => void; startEditSet: () => void; deleteSet: () => void; toggleStar: (card: Flashcard) => void }) {
  return <><section className="page-heading"><div><p className="eyebrow">Hoc phan</p><h1>{props.selectedSet.title}</h1><p>{props.selectedSet.description || "Chua co mo ta"}</p></div><div className="header-actions"><button className="secondary-button" onClick={props.startEditSet}>Sua the</button><button className="danger" onClick={props.deleteSet}>Xoa set</button></div></section><section className="panel study-panel"><div className="mode-tabs">{(["dashboard", "flashcards", "learn", "test", "match"] as StudyMode[]).map((mode) => <button className={props.studyMode === mode ? "tab active" : "tab"} key={mode} onClick={() => props.setStudyMode(mode)}>{mode}</button>)}</div>{props.studyMode === "dashboard" ? <div className="card-list">{props.cards.length === 0 ? <p className="empty">Chua co flashcard.</p> : null}{props.cards.map((card) => <article className="mini-card" key={card.id}><div><strong>{card.term}</strong><span>{card.definition}</span></div><button onClick={() => props.toggleStar(card)}>{card.starred ? "Starred" : "Star"}</button></article>)}</div> : null}{props.studyMode === "flashcards" && props.activeCard ? <div className="study-card" onClick={() => props.setShowBack((value) => !value)}><p>{props.showBack ? props.activeCard.definition : props.activeCard.term}</p><span>Click de lat the</span><div className="pager"><button onClick={(event) => { event.stopPropagation(); props.nextCard(-1); }}>Prev</button><strong>{props.cardIndex + 1} / {props.cards.length}</strong><button onClick={(event) => { event.stopPropagation(); props.nextCard(1); }}>Next</button></div></div> : null}{props.studyMode === "learn" && props.activeCard ? <div className="learn-box"><h2>{props.activeCard.term}</h2><input placeholder="Nhap definition" value={props.answer} onChange={(event) => props.setAnswer(event.target.value)} /><button onClick={props.checkAnswer}>Kiem tra</button><button onClick={() => props.nextCard(1)}>Cau tiep</button><p>{props.lastResult}</p></div> : null}{props.studyMode === "test" ? <div className="test-list">{props.cards.map((card, index) => <label key={card.id}>{index + 1}. {card.term}<input placeholder="Definition" /></label>)}</div> : null}{props.studyMode === "match" ? <div className="match-grid">{props.cards.slice(0, 6).map((card) => <div className="match-pair" key={card.id}><strong>{card.term}</strong><span>{card.definition}</span></div>)}</div> : null}</section></>;
}

function Metric({ label, value }: { label: string; value: string }) { return <div className="metric-card"><span>{label}</span><strong>{value}</strong></div>; }
function newDraftCard(): DraftCard { return { key: crypto.randomUUID(), term: "", definition: "" }; }
function emptyDraftCards(): DraftCard[] { return [newDraftCard(), newDraftCard(), newDraftCard()]; }
function toDraftCards(cards: Flashcard[]): DraftCard[] { return cards.length ? cards.map((card) => ({ key: String(card.id), id: card.id, term: card.term, definition: card.definition, starred: card.starred })) : emptyDraftCards(); }

createRoot(document.getElementById("root")!).render(<React.StrictMode><App /></React.StrictMode>);
