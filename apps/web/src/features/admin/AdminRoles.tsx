// AdminRoles — Phase 4 (ADM-08)
// Read-only view of the managed roles plus who currently holds them.
// Editing policies is deliberately out of scope (plan non-goal), so this page can only
// inspect existing assignments.
import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { DataTable, EmptyState, SectionCard, formatDateTime, type Column } from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { adminIamApi, type AdminAssignment } from "../../lib/api";

export function AdminRoles() {
  const { token } = useAuth();
  const [roleKey, setRoleKey] = useState("");
  const [userId, setUserId] = useState("");
  const [offset, setOffset] = useState(0);

  const rolesQuery = useQuery({
    queryKey: ["admin-roles"],
    queryFn: () => adminIamApi.listRoles(token),
    enabled: !!token,
    staleTime: 5 * 60_000,
  });

  const assignmentsQuery = useQuery({
    queryKey: ["admin-assignments", roleKey, userId, offset],
    queryFn: () =>
      adminIamApi.listAssignments(token, {
        roleKey: roleKey || undefined,
        userId: userId ? Number(userId) : undefined,
        limit: 25,
        offset,
      }),
    enabled: !!token,
    placeholderData: (prev) => prev,
  });


  const roles = rolesQuery.data?.roles ?? [];
  const assignments = assignmentsQuery.data?.assignments ?? [];

  const columns: Array<Column<AdminAssignment>> = [
    { key: "id", header: "ID" },
    { key: "principalId", header: "User ID" },
    { key: "roleKey", header: "Role" },
    {
      key: "scope",
      header: "Phạm vi",
      render: (row) => `${row.scopeType}${row.scopeId ? ` #${row.scopeId}` : ""}`,
    },
    { key: "createdBy", header: "Cấp bởi" },
    { key: "createdAt", header: "Ngày cấp", render: (row) => formatDateTime(row.createdAt) },

  ];

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold">Vai trò & quyền</h1>
        <p className="text-sm text-[var(--muted-foreground)]">
          Xem vai trò và danh sách tài khoản đang giữ quyền. Quản lý gán quyền ở mục Người dùng.
        </p>
      </header>

      {(rolesQuery.isError || assignmentsQuery.isError) && <div role="alert">Không tải được quyền. <button onClick={() => { void rolesQuery.refetch(); void assignmentsQuery.refetch(); }}>Thử lại</button></div>}

      <SectionCard title="Role dựng sẵn" description={`${roles.length} role`}>
        {rolesQuery.isLoading ? (
          <div className="h-24 animate-pulse rounded bg-[var(--muted)]" />
        ) : (
          <ul className="grid gap-2 sm:grid-cols-2">
            {roles.map((r) => (
              <li key={`${r.key}-${r.orgId ?? 0}`} className="rounded-xl border border-[var(--border)] p-3">
                <p className="text-sm font-semibold">
                  {r.name} <span className="font-normal text-[var(--muted-foreground)]">({r.key})</span>
                </p>
                {r.policy && <details className="mt-2 text-xs"><summary className="cursor-pointer">Xem quyền và điều kiện</summary><ul className="mt-2 space-y-2">{r.policy.Statements.map((statement, index) => <li key={index}><strong>{statement.Effect === "Allow" ? "Cho phép" : "Từ chối"}</strong>: {statement.Actions.join(", ")}<p>Phạm vi: {statement.Resources.join(", ")}</p>{statement.Condition && <p>Điều kiện: {Object.entries(statement.Condition).map(([key, value]) => `${key}: ${value}`).join(" · ")}</p>}</li>)}</ul></details>}
                {r.description && <p className="mt-1 text-xs text-[var(--muted-foreground)]">{r.description}</p>}
                <p className="mt-1 text-xs text-[var(--muted-foreground)]">
                  loại: {r.kind}
                  {r.orgId ? ` · tổ chức #${r.orgId}` : " · toàn hệ thống"}
                </p>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard
        title="Ai đang giữ quyền"
        actions={
          <div className="flex flex-wrap gap-2">
            <select
              value={roleKey}
              onChange={(e) => { setRoleKey(e.target.value); setOffset(0); }}
              aria-label="Lọc theo role"
              className="rounded-lg border border-[var(--border)] bg-[var(--card)] px-2 py-1.5 text-sm"
            >
              <option value="">Mọi role</option>
              {roles.map((r) => (
                <option key={r.key} value={r.key}>
                  {r.key}
                </option>
              ))}
            </select>
            <input
              type="number"
              min={1}
              value={userId}
              onChange={(e) => { setUserId(e.target.value); setOffset(0); }}
              placeholder="User ID"
              aria-label="Lọc theo user id"
              className="w-28 rounded-lg border border-[var(--border)] bg-[var(--card)] px-2 py-1.5 text-sm"
            />
          </div>
        }
      >
        <DataTable
          columns={columns}
          rows={assignments}
          rowKey={(row) => row.id}
          loading={assignmentsQuery.isLoading}
          empty={<EmptyState title="Không có gán quyền nào khớp bộ lọc" />}
        />
        <div className="mt-3 flex gap-3"><button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - 25))}>Trước</button><span>Trang {offset / 25 + 1}</span><button disabled={assignments.length < 25} onClick={() => setOffset(offset + 25)}>Sau</button></div>
      </SectionCard>
    </div>
  );
}
