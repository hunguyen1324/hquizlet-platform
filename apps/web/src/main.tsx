import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type HealthStatus = "checking" | "live" | "offline";
type AuthMode = "login" | "register";
type StudyMode = "dashboard" | "flashcards" | "learn" | "test" | "match";
type User = { id: number; name: string; email: string; role: string };
type AuthResponse = { authenticated: boolean; token: string; user: User };
type ServiceHealth = { name: string; url: string; status: string };
type StudySet = { id: number; title: string; description: string; flashcards?: Flashcard[] };
type Flashcard = { id: number; studySetId: number; term: string; definition: string; starred: boolean };

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
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [term, setTerm] = useState("");
  const [definition, setDefinition] = useState("");
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
  function logoutLocal() { localStorage.removeItem(tokenKey); setToken(""); setUser(null); setSelectedSet(null); setStudyMode("dashboard"); }

  async function loadSets() {
    setLoading(true);
    try { setSets(await api<StudySet[]>("/v1/study-sets")); }
    catch (error) { setMessage(error instanceof Error ? error.message : "Khong tai duoc study sets."); }
    finally { setLoading(false); }
  }

  async function openSet(id: number) {
    const response = await api<StudySet>(`/v1/study-sets/${id}`);
    setSelectedSet(response); setTitle(response.title); setDescription(response.description); setCardIndex(0); setShowBack(false); setStudyMode("dashboard");
  }

  async function saveSet(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const payload = JSON.stringify({ title, description });
    const saved = selectedSet
      ? await api<StudySet>(`/v1/study-sets/${selectedSet.id}`, { method: "PUT", body: payload })
      : await api<StudySet>("/v1/study-sets", { method: "POST", body: payload });
    setSelectedSet(saved); setTitle(saved.title); setDescription(saved.description); await loadSets(); await openSet(saved.id);
  }

  async function deleteSet(id: number) { await api(`/v1/study-sets/${id}`, { method: "DELETE" }); setSelectedSet(null); setTitle(""); setDescription(""); await loadSets(); }
  async function saveCard(event: React.FormEvent<HTMLFormElement>) { event.preventDefault(); if (!selectedSet) return; await api(`/v1/study-sets/${selectedSet.id}/flashcards`, { method: "POST", body: JSON.stringify({ term, definition }) }); setTerm(""); setDefinition(""); await openSet(selectedSet.id); }
  async function deleteCard(card: Flashcard) { await api(`/v1/flashcards/${card.id}`, { method: "DELETE" }); if (selectedSet) await openSet(selectedSet.id); }
  async function toggleStar(card: Flashcard) { await api(`/v1/flashcards/${card.id}/star`, { method: "POST" }); if (selectedSet) await openSet(selectedSet.id); }
  function nextCard(step: number) { if (!cards.length) return; setCardIndex((current) => (current + step + cards.length) % cards.length); setShowBack(false); setAnswer(""); setLastResult(""); }
  function checkAnswer() { if (!activeCard) return; const ok = answer.trim().toLowerCase() === activeCard.definition.trim().toLowerCase(); setLastResult(ok ? "Dung roi." : `Chua dung. Dap an: ${activeCard.definition}`); }

  if (!user) return <main className="auth-shell"><section className="auth-hero"><p className="eyebrow">HQuizlet Platform</p><h1>Hoc bang flashcard, backend Go, du lieu PostgreSQL.</h1><p>Dang nhap hoac dang ky de tao study set, them flashcard va thu Flashcards, Learn, Test, Match.</p><div className="service-strip"><strong>{liveCount} / {services.length || 4}</strong><span>services live</span><span className={`badge badge--${healthStatus}`}>{healthStatus}</span></div></section><section className="panel auth-card"><div className="tabs"><button className={authMode === "login" ? "tab active" : "tab"} onClick={() => setAuthMode("login")} type="button">Dang nhap</button><button className={authMode === "register" ? "tab active" : "tab"} onClick={() => setAuthMode("register")} type="button">Dang ky</button></div><form className="stack" onSubmit={submitAuth}>{authMode === "register" ? <label>Ten<input value={name} onChange={(event) => setName(event.target.value)} /></label> : null}<label>Email<input type="email" value={email} onChange={(event) => setEmail(event.target.value)} /></label><label>Mat khau<input type="password" value={password} onChange={(event) => setPassword(event.target.value)} /></label><button className="primary-button" disabled={loading} type="submit">{authMode === "login" ? "Dang nhap" : "Tao tai khoan"}</button></form><p className="message">{message}</p></section></main>;

  return <main className="app-shell"><header className="topbar"><div><p className="eyebrow">Dashboard</p><h1>HQuizlet</h1></div><div className="user-menu"><span>{user.name}</span><button onClick={() => void logout()}>Logout</button></div></header><section className="summary-grid"><Metric label="Study sets" value={sets.length.toString()} /><Metric label="Selected cards" value={cards.length.toString()} /><Metric label="Backend" value={healthStatus} /></section><section className="workspace-grid"><aside className="panel"><div className="panel-heading"><div><p className="eyebrow">Library</p><h2>Study sets</h2></div><button onClick={() => void loadSets()}>Reload</button></div><div className="set-list">{sets.length === 0 ? <p className="empty">Chua co bo the nao.</p> : null}{sets.map((set) => <button className={selectedSet?.id === set.id ? "set-row active" : "set-row"} key={set.id} onClick={() => void openSet(set.id)}><strong>{set.title}</strong><span>{set.description || "No description"}</span></button>)}</div></aside><section className="panel"><div className="panel-heading"><div><p className="eyebrow">Editor</p><h2>{selectedSet ? "Sua bo the" : "Tao bo the"}</h2></div>{selectedSet ? <button className="danger" onClick={() => void deleteSet(selectedSet.id)}>Xoa set</button> : null}</div><form className="stack" onSubmit={saveSet}><label>Tieu de<input value={title} onChange={(event) => setTitle(event.target.value)} /></label><label>Mo ta<textarea value={description} onChange={(event) => setDescription(event.target.value)} /></label><button className="primary-button" type="submit">Luu study set</button></form>{selectedSet ? <><hr /><form className="card-form" onSubmit={saveCard}><input placeholder="Term" value={term} onChange={(event) => setTerm(event.target.value)} /><input placeholder="Definition" value={definition} onChange={(event) => setDefinition(event.target.value)} /><button type="submit">Them the</button></form><div className="card-list">{cards.length === 0 ? <p className="empty">Them flashcard dau tien de bat dau hoc.</p> : null}{cards.map((card) => <article className="mini-card" key={card.id}><div><strong>{card.term}</strong><span>{card.definition}</span></div><button onClick={() => void toggleStar(card)}>{card.starred ? "Starred" : "Star"}</button><button className="danger" onClick={() => void deleteCard(card)}>Xoa</button></article>)}</div></> : null}</section></section>{selectedSet ? <section className="panel study-panel"><div className="mode-tabs">{(["dashboard", "flashcards", "learn", "test", "match"] as StudyMode[]).map((mode) => <button className={studyMode === mode ? "tab active" : "tab"} key={mode} onClick={() => setStudyMode(mode)}>{mode}</button>)}</div>{studyMode === "dashboard" ? <p className="message">Chon mode de hoc bo "{selectedSet.title}". Du lieu dang luu trong PostgreSQL.</p> : null}{studyMode === "flashcards" && activeCard ? <div className="study-card" onClick={() => setShowBack((value) => !value)}><p>{showBack ? activeCard.definition : activeCard.term}</p><span>Click de lat the</span><div className="pager"><button onClick={(event) => { event.stopPropagation(); nextCard(-1); }}>Prev</button><strong>{cardIndex + 1} / {cards.length}</strong><button onClick={(event) => { event.stopPropagation(); nextCard(1); }}>Next</button></div></div> : null}{studyMode === "learn" && activeCard ? <div className="learn-box"><h2>{activeCard.term}</h2><input placeholder="Nhap definition" value={answer} onChange={(event) => setAnswer(event.target.value)} /><button onClick={checkAnswer}>Kiem tra</button><button onClick={() => nextCard(1)}>Cau tiep</button><p>{lastResult}</p></div> : null}{studyMode === "test" ? <div className="test-list">{cards.map((card, index) => <label key={card.id}>{index + 1}. {card.term}<input placeholder="Definition" /></label>)}</div> : null}{studyMode === "match" ? <div className="match-grid">{cards.slice(0, 6).map((card) => <div className="match-pair" key={card.id}><strong>{card.term}</strong><span>{card.definition}</span></div>)}</div> : null}</section> : null}<p className="message">{message}</p></main>;
}

function Metric({ label, value }: { label: string; value: string }) { return <div className="metric-card"><span>{label}</span><strong>{value}</strong></div>; }

createRoot(document.getElementById("root")!).render(<React.StrictMode><App /></React.StrictMode>);
