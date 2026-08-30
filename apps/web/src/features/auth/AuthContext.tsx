// Auth context - Dev 3 (FE-CORE-04: Protected layout)
// Uses mock data until Dev 1 backend is ready (FE-CORE-07)

import React, { createContext, useContext, useState, useCallback } from "react";
import type { User } from "../../types";
import { mockLogin, mockRegister } from "../../lib/mock/mockData";

const TOKEN_KEY = "hquizlet.sessionToken";

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

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const login = useCallback(async (email: string, password: string) => {
    setLoading(true);
    setError("");
    try {
      // TODO (FE-CORE-07): swap to real API when Dev 1 ready
      // const res = await apiClient.post<AuthResponse>("/v1/auth/login", { email, password });
      const res = mockLogin(email, password);
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
      // TODO (FE-CORE-07): swap to real API when Dev 1 ready
      const res = mockRegister(name, email, password);
      localStorage.setItem(TOKEN_KEY, res.token);
      setToken(res.token);
      setUser(res.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Đăng ký thất bại.");
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(() => {
    // TODO (FE-CORE-07): call /v1/auth/logout when Dev 1 ready
    localStorage.removeItem(TOKEN_KEY);
    setToken("");
    setUser(null);
  }, []);

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
