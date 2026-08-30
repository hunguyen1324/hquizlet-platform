// AuthContext - Dev 3 (FE-CORE-03, FE-CORE-04)
// Gọi gateway API thật. Mock đã bị xóa — đây là fix regression từ commit trước.

import React, { createContext, useContext, useState, useCallback, useEffect } from "react";
import type { User, AuthResponse } from "../../types";

const TOKEN_KEY = "hquizlet.sessionToken";
const gatewayUrl = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

type AuthContextValue = {
  user: User | null;
  token: string;
  loading: boolean;
  error: string;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

async function apiFetch<T>(path: string, token: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${gatewayUrl}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers,
    },
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error((body as { error?: string }).error ?? `Request failed ${res.status}`);
  return body as T;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  // Restore session on mount
  useEffect(() => {
    if (!token) return;
    apiFetch<{ authenticated: boolean; user: User }>("/v1/auth/me", token)
      .then((res) => { if (res.authenticated) setUser(res.user); else clearSession(); })
      .catch(() => clearSession());
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function clearSession() {
    localStorage.removeItem(TOKEN_KEY);
    setToken("");
    setUser(null);
  }

  const login = useCallback(async (email: string, password: string) => {
    setLoading(true);
    setError("");
    try {
      const res = await apiFetch<AuthResponse>("/v1/auth/login", "", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      localStorage.setItem(TOKEN_KEY, res.token);
      setToken(res.token);
      setUser(res.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Đăng nhập thất bại.");
    } finally {
      setLoading(false);
    }
  }, []);

  const register = useCallback(async (name: string, email: string, password: string) => {
    setLoading(true);
    setError("");
    try {
      const res = await apiFetch<AuthResponse>("/v1/auth/register", "", {
        method: "POST",
        body: JSON.stringify({ name, email, password }),
      });
      localStorage.setItem(TOKEN_KEY, res.token);
      setToken(res.token);
      setUser(res.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Đăng ký thất bại.");
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiFetch("/v1/auth/logout", token, { method: "POST" });
    } catch {
      // best-effort
    } finally {
      clearSession();
    }
  }, [token]);

  return (
    <AuthContext.Provider value={{ user, token, loading, error, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}

// Export apiFetch cho các feature khác dùng (Dashboard, StudySetEditor)
export { apiFetch };
