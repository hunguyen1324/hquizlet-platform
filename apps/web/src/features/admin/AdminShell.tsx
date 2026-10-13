// AdminShell — Phase 4 (ADM-01)
// YouTube-Studio-style console: fixed left menu, routed content area.
// The gateway already refuses non-admins on /v1/admin/*; this guard only avoids rendering
// a console that could not load any data.
import React from "react";
import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { useIsAdmin, usePlatformRoles } from "../auth/permissions";

const navItems = [
  { to: "/admin", label: "Tổng quan", end: true },
  { to: "/admin/users", label: "Người dùng" },
  { to: "/admin/content", label: "Nội dung" },
  { to: "/admin/classes", label: "Lớp học" },
  { to: "/admin/roles", label: "Vai trò & quyền" },
  { to: "/admin/payments", label: "Thanh toán" },
  { to: "/admin/audit", label: "Nhật ký (Audit)" },
  { to: "/admin/system", label: "Hệ thống" },
];

export function AdminShell() {
  const {permissionsLoading}=useAuth();
  const isAdmin = useIsAdmin();
  const roles = usePlatformRoles();

  if (permissionsLoading) return <div aria-busy="true">Đang tải quyền quản trị…</div>;
  if (!isAdmin) {
    return (
      <div className="mx-auto max-w-2xl p-6">
        <div className="message message--error" role="alert">
          Bạn không có quyền truy cập trang quản trị (dashboard:ViewAdmin).
        </div>
        {roles.length > 0 && (
          <p className="mt-2 text-sm text-[var(--muted-foreground)]">Vai trò hiện tại: {roles.join(", ")}</p>
        )}
      </div>
    );
  }

  return (
    <div className="flex min-h-[70vh] flex-col gap-4 md:flex-row">
      <nav className="shrink-0 border-b border-[var(--border)] pb-2 md:w-52 md:border-b-0 md:border-r md:pr-2" aria-label="Quản trị">
        <p className="mb-2 px-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">
          Quản trị hệ thống
        </p>
        <ul className="space-y-1">
          {navItems.map((item) => (
            <li key={item.to}>
              <NavLink
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `block rounded-lg px-3 py-2 text-sm ${
                    isActive ? "bg-[var(--muted)] font-medium" : "hover:bg-[var(--muted)]"
                  }`
                }
              >
                {item.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <div className="min-w-0 flex-1">
        <Outlet />
      </div>
    </div>
  );
}
