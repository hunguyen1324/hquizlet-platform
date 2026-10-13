// permissions.tsx — Dev 3 [UI-02]
// Client-side permission helpers. Hiding controls is cosmetic only: every API call is
// re-checked on the server, so these helpers never grant access by themselves.
import React from "react";
import { useAuth } from "./AuthContext";
import { authApi } from "../../lib/api";

const classCache = new Map<string, string[]>();
const cachedAt = new Map<string,number>();
const inflight = new Map<string, Promise<string[]>>();

/** Drop cached class-scope permissions (call after joining/leaving/changing a class). */
export function clearClassPermissionCache(classId?: number) {
  if (classId === undefined) {
    classCache.clear();
    cachedAt.clear();
    inflight.clear();
    return;
  }
  for (const key of Array.from(classCache.keys())) {
    if (key.endsWith(`:${classId}`)) classCache.delete(key);
  }
}

/** Effective actions for a class scope, loaded on demand and cached per token+class. */
export function useClassPermissions(classId?: number): string[] | null {
  const { token } = useAuth();
  const key = token && classId ? `${token}:${classId}` : "";
  const [actions, setActions] = React.useState<string[] | null>(() => (key ? classCache.get(key) ?? null : null));

  React.useEffect(() => {
    if (!key || !classId) {
      setActions(null);
      return;
    }
    const cached = classCache.get(key);
    if (cached && Date.now() - (cachedAt.get(key)??0) < 30000) {
      setActions(cached);
      return;
    }
    setActions(null);
    let alive = true;
    let pending = inflight.get(key);
    if (!pending) {
      pending = authApi
        .permissions(token, classId)
        .then((res) => res.actions ?? [])
        .catch(() => [])
        .finally(() => inflight.delete(key));
      inflight.set(key, pending);
    }
    pending.then((list) => {
      classCache.set(key, list);
      cachedAt.set(key,Date.now());
      if (alive) setActions(list);
    });
    return () => {
      alive = false;
    };
  }, [key, classId, token]);

  return actions;
}

/** useCan("class:Update") or useCan("class:Update", { classId: 7 }). */
export function useCan(action: string, opts?: { classId?: number }): boolean {
  const { permissions } = useAuth();
  const classActions = useClassPermissions(opts?.classId);
  if (opts?.classId) return (classActions ?? []).includes(action);
  return (permissions?.actions ?? []).includes(action);
}

/** useCanAny(["a", "b"]) — true when the caller holds at least one of the actions. */
export function useCanAny(actions: string[], opts?: { classId?: number }): boolean {
  const { permissions } = useAuth();
  const classActions = useClassPermissions(opts?.classId);
  const list = opts?.classId ? classActions ?? [] : permissions?.actions ?? [];
  return actions.some((a) => list.includes(a));
}

export function usePlatformRoles(): string[] {
  return useAuth().permissions?.platformRoles ?? [];
}

export function useIsAdmin(): boolean {
  return usePlatformRoles().includes("PlatformAdmin");
}

type CanProps = {
  action: string;
  classId?: number;
  fallback?: React.ReactNode;
  children: React.ReactNode;
};

/** <Can action="class:Update" classId={id}>…</Can> */
export function Can({ action, classId, fallback = null, children }: CanProps) {
  const allowed = useCan(action, classId ? { classId } : undefined);
  return <>{allowed ? children : fallback}</>;
}
