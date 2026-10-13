// AdminUsers — Phase 4 (ADM-05)
// Search/filter users, inspect a user, disable/enable, assign and revoke roles.
// Every mutation is dangerous enough to need an explicit confirmation dialog, and the
// server writes an audit_log row for each one.
import React, { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DataTable, SectionCard, EmptyState, formatDateTime, type Column } from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { adminIamApi, type AdminAssignment, type AdminRole, type AdminUserRow } from "../../lib/api";

const PAGE_SIZE = 25;

export function AdminUsers() {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [role, setRole] = useState("");
 const [status,setStatus]=useState("");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<AdminUserRow | null>(null);
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);

  const usersQuery = useQuery({
    queryKey: ["admin-users", search, role, status, offset],
    queryFn: () => adminIamApi.listUsers(token, { search: search || undefined, role: role || undefined,status:status||undefined, limit: PAGE_SIZE, offset }),
    enabled: !!token,
    placeholderData: (prev) => prev,
  });

  const rolesQuery = useQuery({
    queryKey: ["admin-roles"],
    queryFn: () => adminIamApi.listRoles(token),
    enabled: !!token,
    staleTime: 5 * 60_000,
  });

  const assignmentsQuery = useQuery({
    queryKey: ["admin-assignments", selected?.id],
    queryFn: () => adminIamApi.listAssignments(token, { userId: selected!.id, limit: 100 }),
    enabled: !!token && !!selected,
  });

  const refreshUsers = () => {
    void queryClient.invalidateQueries({ queryKey: ["admin-users"] });
    void queryClient.invalidateQueries({ queryKey: ["admin-assignments"] });
  };

  const toggleDisabled = useMutation({
    mutationFn: (user: AdminUserRow) => (user.disabled ? adminIamApi.enableUser(token, user.id) : adminIamApi.disableUser(token, user.id)),
    onSuccess: (updated) => {
      setNotice({ kind: "ok", text: updated.disabled ? `Đã vô hiệu hoá ${updated.email}` : `Đã mở lại ${updated.email}` });
      setSelected((cur) => (cur && cur.id === updated.id ? updated : cur));
      refreshUsers();
    },
    onError: (err: unknown) => setNotice({ kind: "error", text: err instanceof Error ? err.message : "Thao tác thất bại" }),
  });

  const assignRole = useMutation({
    mutationFn: (input: { userId: number; roleKey: string; scopeType: string; scopeId: number | null; password?: string }) => adminIamApi.assignRole(token, input),
    onSuccess: (created: AdminAssignment) => {
      setNotice({ kind: "ok", text: `Đã gán ${created.roleKey} (${created.scopeType})` });
      refreshUsers();
    },
    onError: (err: unknown) => setNotice({ kind: "error", text: err instanceof Error ? err.message : "Không gán được role" }),
  });

  const revokeRole = useMutation({
    mutationFn: (assignmentId: number) => adminIamApi.revokeRole(token, assignmentId),
    onSuccess: () => {
      setNotice({ kind: "ok", text: "Đã thu hồi quyền" });
      refreshUsers();
    },
    onError: (err: unknown) => setNotice({ kind: "error", text: err instanceof Error ? err.message : "Không thu hồi được quyền" }),
  });

  const users = usersQuery.data?.users ?? [];
  const total = usersQuery.data?.total ?? 0;
  const roles = (rolesQuery.data?.roles ?? []).filter(r => ["PlatformAdmin","PlatformSupport","Teacher","Student"].includes(r.key));

  const columns: Array<Column<AdminUserRow>> = [
    { key: "id", header: "ID" },
    { key: "name", header: "Tên" },
    { key: "email", header: "Email" },
    { key: "role", header: "Vai trò", render: row => row.platformRoles?.join(", ") || row.role },
    {
      key: "status",
      header: "Trạng thái",
      render: (row) => (
        <span className={row.disabled ? "text-red-600 dark:text-red-400" : "text-green-600 dark:text-green-400"}>
          {row.disabled ? "Đã vô hiệu hoá" : "Đang hoạt động"}
        </span>
      ),
    },
    { key: "createdAt", header: "Ngày tạo", render: (row) => formatDateTime(row.createdAt) },
  ];

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-xl font-semibold">Người dùng</h1>
          <p className="text-sm text-[var(--muted-foreground)]">{total} tài khoản</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="search"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setOffset(0);
            }}
            placeholder="Tìm theo tên hoặc email…"
            aria-label="Tìm người dùng"
            className="rounded-lg border border-[var(--border)] bg-[var(--card)] px-3 py-1.5 text-sm"
          />
          <select
            value={role}
            onChange={(e) => {
              setRole(e.target.value);
              setOffset(0);
            }}
            aria-label="Lọc theo role"
            className="rounded-lg border border-[var(--border)] bg-[var(--card)] px-3 py-1.5 text-sm"
          >
            <option value="">Mọi role</option>
            {["PlatformAdmin","PlatformSupport","Teacher","Student"].map(role=><option key={role} value={role}>{role}</option>)}
          </select>
          <select aria-label="Lọc trạng thái" value={status} onChange={e=>{setStatus(e.target.value);setOffset(0);}}><option value="">Mọi trạng thái</option><option value="active">Đang hoạt động</option><option value="disabled">Đã vô hiệu hoá</option></select>
        </div>
      </header>

      {notice && (
        <div className={notice.kind === "ok" ? "message message--success" : "message message--error"} role="status">
          {notice.text}
        </div>
      )}
      {usersQuery.isError && (
        <div className="message message--error" role="alert">
          Không tải được danh sách người dùng.
          <button className="ghost-button ml-2" onClick={() => void usersQuery.refetch()}>
            Thử lại
          </button>
        </div>
      )}

      <SectionCard title="Danh sách" description={`Trang ${Math.floor(offset / PAGE_SIZE) + 1}`}>
        <DataTable
          columns={columns}
          rows={users}
          rowKey={(row) => row.id}
          loading={usersQuery.isLoading}
          onRowClick={(row) => setSelected(row)}
          empty={<EmptyState title="Không có người dùng nào khớp bộ lọc" />}
        />
        <div className="mt-3 flex items-center justify-between text-sm">
          <button className="ghost-button" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
            ← Trước
          </button>
          <span className="text-[var(--muted-foreground)]">
            {total === 0 ? "0" : `${offset + 1}–${Math.min(offset + PAGE_SIZE, total)}`} / {total}
          </span>
          <button className="ghost-button" disabled={offset + PAGE_SIZE >= total} onClick={() => setOffset(offset + PAGE_SIZE)}>
            Sau →
          </button>
        </div>
      </SectionCard>

      {selected && (
        <SectionCard
          title={`Chi tiết: ${selected.name}`}
          description={`#${selected.id} · ${selected.email}`}
          actions={
            <button className="ghost-button" onClick={() => setSelected(null)}>
              Đóng
            </button>
          }
        >
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <button
              className={selected.disabled ? "primary-button" : "danger-button"}
              disabled={toggleDisabled.isPending}
              onClick={() => {
                const verb = selected.disabled ? "mở lại" : "vô hiệu hoá";
                if (window.confirm(`Bạn chắc chắn muốn ${verb} tài khoản ${selected.email}?`)) {
                  toggleDisabled.mutate(selected);
                }
              }}
            >
              {selected.disabled ? "Mở lại tài khoản" : "Vô hiệu hoá tài khoản"}
            </button>
          </div>
          <AssignRoleForm
            roles={roles}
            busy={assignRole.isPending}
            onSubmit={(input) => {
              if (window.confirm(`Gán role ${input.roleKey} (${input.scopeType}) cho ${selected.email}?`)) {
                assignRole.mutate({ userId: selected.id, ...input });
              }
            }}
          />
          <h3 className="mb-2 mt-4 text-sm font-semibold">Quyền đang giữ</h3>
          {assignmentsQuery.isLoading && <div className="h-16 animate-pulse rounded bg-[var(--muted)]" />}
          {!assignmentsQuery.isLoading && (assignmentsQuery.data?.assignments ?? []).length === 0 && (
            <p className="text-sm text-[var(--muted-foreground)]">Chưa có gán quyền nào.</p>
          )}
          <ul className="space-y-1">
            {(assignmentsQuery.data?.assignments ?? []).map((a) => (
              <li key={a.id} className="flex items-center justify-between rounded-lg border border-[var(--border)] px-3 py-2 text-sm">
                <span>
                  <strong>{a.roleKey}</strong> · {a.scopeType}
                  {a.scopeId ? ` #${a.scopeId}` : ""} · cấp {formatDateTime(a.createdAt)}
                  {a.expiresAt ? ` · hết hạn ${formatDateTime(a.expiresAt)}` : ""}
                </span>
                <button
                  className="danger-button"
                  disabled={revokeRole.isPending}
                  onClick={() => {
                    if (window.confirm(`Thu hồi ${a.roleKey} khỏi ${selected.email}?`)) revokeRole.mutate(a.id);
                  }}
                >
                  Thu hồi
                </button>
              </li>
            ))}
          </ul>
        </SectionCard>
      )}
    </div>
  );
}

function AssignRoleForm({
  roles,
  busy,
  onSubmit,
}: {
  roles: AdminRole[];
  busy: boolean;
  onSubmit: (input: { roleKey: string; scopeType: string; scopeId: number | null; password?: string }) => void;
}) {
  const [roleKey, setRoleKey] = useState("");
  const [scopeType, setScopeType] = useState("platform");
  const [scopeId, setScopeId] = useState("");
  const [password, setPassword] = useState("");
  const needsScope = scopeType !== "platform";

  return (
    <div className="flex flex-wrap items-end gap-2">
      <label className="text-xs">
        <span className="mb-1 block text-[var(--muted-foreground)]">Role</span>
        <select value={roleKey} onChange={(e) => setRoleKey(e.target.value)} className="rounded-lg border border-[var(--border)] bg-[var(--card)] px-2 py-1.5 text-sm">
          <option value="">— chọn role —</option>
          {roles.map((r) => (
            <option key={`${r.key}-${r.orgId ?? 0}`} value={r.key}>
              {r.name} ({r.key})
            </option>
          ))}
        </select>
      </label>
      <label className="text-xs">
        <span className="mb-1 block text-[var(--muted-foreground)]">Phạm vi</span>
        <select value={scopeType} onChange={(e) => setScopeType(e.target.value)} className="rounded-lg border border-[var(--border)] bg-[var(--card)] px-2 py-1.5 text-sm">
          <option value="platform">platform</option>

        </select>
      </label>
      {needsScope && (
        <label className="text-xs">
          <span className="mb-1 block text-[var(--muted-foreground)]">ID phạm vi</span>
          <input
            type="number"
            min={1}
            value={scopeId}
            onChange={(e) => setScopeId(e.target.value)}
            className="w-28 rounded-lg border border-[var(--border)] bg-[var(--card)] px-2 py-1.5 text-sm"
          />
        </label>
      )}
      {roleKey === "PlatformAdmin" && <label>Mật khẩu xác nhận<input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} /></label>}
      <button
        className="primary-button"
        disabled={busy || !roleKey || (needsScope && !scopeId) || (roleKey === "PlatformAdmin" && !password)}
        onClick={() => { onSubmit({ roleKey, scopeType, scopeId: needsScope ? Number(scopeId) : null, password }); setPassword(""); }}
      >
        Gán role
      </button>
    </div>
  );
}
