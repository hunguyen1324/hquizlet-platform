// AuthContext — Dev 3 [P2-WEB-01]
// Auth state: login, register, logout, session restore.
// Dùng authApi từ lib/api — không gọi fetch trực tiếp.

import React, { createContext, useContext, useState, useCallback, useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { User, AuthResponse } from "../../types";
import { authApi, apiFetch } from "../../lib/api";

const TOKEN_KEY = "hquizlet.sessionToken";
const USER_KEY = "hquizlet.user";

export type Permissions = { platformRoles: string[]; legacyRole: string; actions: string[] };

type AuthContextValue = {
  user: User | null;
  token: string;
  loading: boolean;
  error: string;
  /** Effective platform-scope permissions from GET /v1/me/permissions. */
  permissions: Permissions | null;
  permissionsLoading: boolean;
  refreshPermissions: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string, accountType?: "teacher" | "student") => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
 const queryClient=useQueryClient();
  const [user, setUser] = useState<User | null>(() => {
    try { return JSON.parse(localStorage.getItem(USER_KEY) ?? "null"); }
    catch { return null; }
  });
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [permissions, setPermissions] = useState<Permissions | null>(null);
  const [permissionsLoading, setPermissionsLoading] = useState(!!token);

  // UI-02: load effective permissions so the UI can hide controls. The server always
  // re-checks; hiding is cosmetic only.
  const refreshPermissions = useCallback(async () => {
    const current = localStorage.getItem(TOKEN_KEY) ?? "";
    if (!current) {
      setPermissions(null);
      return;
    }
    setPermissionsLoading(true);
    try {
      const res = await authApi.permissions(current);
      if(localStorage.getItem(TOKEN_KEY)!==current) return;
      setPermissions({ platformRoles: res.platformRoles ?? [], legacyRole: res.legacyRole ?? "user", actions: res.actions ?? [] });
    } catch {
      if(localStorage.getItem(TOKEN_KEY)===current) setPermissions(null);
    } finally {
      setPermissionsLoading(false);
    }
  }, []);

  // Restore session on mount
  useEffect(() => {
    if (!token) return;
    authApi
      .me(token)
      .then((res) => {
        if(localStorage.getItem(TOKEN_KEY)!==token) return;
        if (res.authenticated) {
          setUser(res.user);
          localStorage.setItem(USER_KEY, JSON.stringify({
            id: res.user.id,
            name: res.user.name,
            image: res.user.image,
            role: res.user.role,
          }));
        } else {
          clearSession();
        }
      })
      .catch(() => {if(localStorage.getItem(TOKEN_KEY)===token) clearSession();});
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function clearSession() {
 queryClient.clear();
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    setToken("");
    setUser(null);
    setPermissions(null);
  }

  function persistSession(res: AuthResponse) {
 queryClient.clear();
 setPermissions(null);
    localStorage.setItem(TOKEN_KEY, res.token);
    localStorage.setItem(USER_KEY, JSON.stringify({
      id: res.user.id,
      name: res.user.name,
      image: res.user.image,
      role: res.user.role,
    }));
    setToken(res.token);
    setUser(res.user);
  }

  const login = useCallback(async (email: string, password: string) => {
    setLoading(true);
    setError("");
    try {
      persistSession(await authApi.login(email, password));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Đăng nhập thất bại.");
    } finally {
      setLoading(false);
    }
  }, []);

  const register = useCallback(async (name: string, email: string, password: string, accountType: "teacher" | "student" = "student") => {
    setLoading(true);
    setError("");
    try {
      persistSession(await authApi.register(name, email, password, accountType));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Đăng ký thất bại.");
    } finally {
      setLoading(false);
    }
  }, []);

  // Refresh permissions whenever the session token changes.
  useEffect(() => {
    if (token) void refreshPermissions();
    else setPermissions(null);
  }, [token, refreshPermissions]);

  const logout = useCallback(async () => {
    try {
      await authApi.logout(token);
    } catch {
      // best-effort
    } finally {
      clearSession();
    }
  }, [token]);

  return (
    <AuthContext.Provider value={{ user, token, loading, error, permissions, permissionsLoading, refreshPermissions, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}

// Re-export apiFetch cho các file cũ còn dùng trực tiếp (sẽ migrate dần)
export { apiFetch };
