// AuthScreen - Dev 3 (FE-CORE-03: Login/Register UI)
// Clean auth form with loading/error states

import React, { useState } from "react";
import { useAuth } from "./AuthContext";
import type { AuthMode, HealthStatus } from "../../types";
import { WorkspaceIcon } from "../../components/ui/WorkspaceUI";

type Props = {
  healthStatus: HealthStatus;
  liveCount: number;
  serviceCount: number;
};

export function AuthScreen({ healthStatus, liveCount, serviceCount }: Props) {
  const { login, register, loading, error } = useAuth();
  const [mode, setMode] = useState<AuthMode>("login");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [accountType, setAccountType] = useState<"teacher" | "student">("student");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (mode === "login") {
      await login(email, password);
    } else {
      await register(name, email, password, accountType);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-hero">
        <p className="eyebrow">HQuizlet Platform</p>
        <h1>Học thông minh hơn.<br />Tiến bộ mỗi ngày.</h1>
        <p>Biến kiến thức thành thói quen. Tạo thẻ ghi nhớ, thử sức với quiz và cùng bạn bè chinh phục mục tiêu học tập.</p>
        <div className="auth-benefits"><span><WorkspaceIcon name="book" size={16} /> Ghi nhớ chủ động</span><span><WorkspaceIcon name="sparkles" size={16} /> Học cùng bạn bè</span><span><WorkspaceIcon name="check" size={16} /> Theo dõi tiến bộ</span></div>
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
            <>
              <label>
                Tên
                <input
                  autoComplete="name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Nguyen Van A"
                  required
                />
              </label>
              <fieldset className="account-type-fieldset">
                <legend>Loại tài khoản</legend>
                <label className="inline-choice">
                  <input
                    type="radio"
                    name="accountType"
                    value="student"
                    checked={accountType === "student"}
                    onChange={() => setAccountType("student")}
                  />
                  Học sinh
                </label>
                <label className="inline-choice">
                  <input
                    type="radio"
                    name="accountType"
                    value="teacher"
                    checked={accountType === "teacher"}
                    onChange={() => setAccountType("teacher")}
                  />
                  Giáo viên
                </label>
              </fieldset>
            </>
          )}
          <label>
            Email
            <input
              type="email"
              autoComplete="email"
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
              autoComplete={mode === "login" ? "current-password" : "new-password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
            />
          </label>

          {error && <p className="message message--error" role="alert">{error}</p>}

          <button className="primary-button" disabled={loading} type="submit">
            {loading ? "Đang xử lý..." : mode === "login" ? "Đăng nhập" : "Tạo tài khoản"}
          </button>
        </form>
      </section>
    </main>
  );
}
