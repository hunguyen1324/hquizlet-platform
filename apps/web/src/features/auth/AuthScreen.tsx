// AuthScreen - Dev 3 (FE-CORE-03: Login/Register UI)
// Clean auth form with loading/error states

import React, { useState } from "react";
import { useAuth } from "./AuthContext";
import type { AuthMode, HealthStatus } from "../../types";

type Props = {
  healthStatus: HealthStatus;
  liveCount: number;
  serviceCount: number;
};

export function AuthScreen({ healthStatus, liveCount, serviceCount }: Props) {
  const { login, register, loading, error } = useAuth();
  const [mode, setMode] = useState<AuthMode>("login");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("demo@hquizlet.local");
  const [password, setPassword] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (mode === "login") {
      await login(email, password);
    } else {
      await register(name, email, password);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-hero">
        <p className="eyebrow">HQuizlet Platform</p>
        <h1>Học bằng flashcard, backend Go, dữ liệu PostgreSQL.</h1>
        <p>Đăng nhập hoặc đăng ký để tạo study set, thêm flashcard và thử Flashcards, Learn, Test, Match.</p>
        <div className="service-strip">
          <strong>{liveCount} / {serviceCount}</strong>
          <span>services live</span>
          <span className={`badge badge--${healthStatus}`}>{healthStatus}</span>
        </div>
      </section>

      <section className="panel auth-card">
        <div className="tabs">
          <button
            className={mode === "login" ? "tab active" : "tab"}
            onClick={() => setMode("login")}
            type="button"
          >
            Đăng nhập
          </button>
          <button
            className={mode === "register" ? "tab active" : "tab"}
            onClick={() => setMode("register")}
            type="button"
          >
            Đăng ký
          </button>
        </div>

        <form className="stack" onSubmit={handleSubmit}>
          {mode === "register" && (
            <label>
              Tên
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Nguyen Van A"
                required
              />
            </label>
          )}
          <label>
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="email@example.com"
              required
            />
          </label>
          <label>
            Mật khẩu
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
            />
          </label>

          {error && <p className="message message--error">{error}</p>}

          <button className="primary-button" disabled={loading} type="submit">
            {loading ? "Đang xử lý..." : mode === "login" ? "Đăng nhập" : "Tạo tài khoản"}
          </button>
        </form>
      </section>
    </main>
  );
}
