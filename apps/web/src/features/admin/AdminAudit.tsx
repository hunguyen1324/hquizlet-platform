// AdminAudit — Phase 4 (ADM-08)
// Audit trail with actor/action filters and pagination.
import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { DataTable, EmptyState, SectionCard, formatDateTime, type Column } from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { adminIamApi, type AdminAuditEntry } from "../../lib/api";

const PAGE_SIZE = 50;
const COMMON_ACTIONS = [
  "iam:AssignRole",
  "iam:RevokeRole",
  "iam:ListAssignments",
  "user:Disable",
  "user:List",
  "dashboard:ViewAdmin",
  "audit:Read",
  "wallet:Credit",
  "order:ListAll",
];

export function AdminAudit() {
  const { token } = useAuth();
  const [action, setAction] = useState("");
  const [actorId, setActorId] = useState("");
  const [offset, setOffset] = useState(0);

  const query = useQuery({
    queryKey: ["admin-audit", action, actorId, offset],
    queryFn: () =>
      adminIamApi.listAudit(token, {
        action: action || undefined,
        actorId: actorId ? Number(actorId) : undefined,
        limit: PAGE_SIZE,
        offset,
      }),
    enabled: !!token,
    placeholderData: (prev) => prev,
  });

  const entries = query.data?.entries ?? [];

  const columns: Array<Column<AdminAuditEntry>> = [
    { key: "id", header: "ID" },
    { key: "at", header: "Thời điểm", render: (row) => formatDateTime(row.at) },
    { key: "actorId", header: "Actor" },
    { key: "action", header: "Hành động" },
    { key: "resource", header: "Đối tượng" },
    {
      key: "result",
      header: "Kết quả",
      render: (row) => (
        <span className={row.result === "deny" ? "text-red-600 dark:text-red-400" : "text-green-600 dark:text-green-400"}>
          {row.result}
        </span>
      ),
    },
    { key: "ip", header: "IP", render: (row) => row.ip || "—" },
    { key: "requestId", header: "Request ID", render: (row) => row.requestId || "—" },
  ];

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold">Nhật ký (Audit)</h1>
        <p className="text-sm text-[var(--muted-foreground)]">Ai làm gì, lúc nào, kết quả ra sao.</p>
      </header>

      {query.isError && (
        <div className="message message--error" role="alert">
          Không tải được nhật ký.{" "}
          <button className="ghost-button" onClick={() => void query.refetch()}>
            Thử lại
          </button>
        </div>
      )}

      <SectionCard
        title="Bản ghi"
        description={`${entries.length} bản ghi gần nhất trong trang này`}
        actions={
          <div className="flex flex-wrap gap-2">
            <select
              value={action}
              onChange={(e) => {
                setAction(e.target.value);
                setOffset(0);
              }}
              aria-label="Lọc theo hành động"
              className="rounded-lg border border-[var(--border)] bg-[var(--card)] px-2 py-1.5 text-sm"
            >
              <option value="">Mọi hành động</option>
              {COMMON_ACTIONS.map((a) => (
                <option key={a} value={a}>
                  {a}
                </option>
              ))}
            </select>
            <input
              type="number"
              min={1}
              value={actorId}
              onChange={(e) => {
                setActorId(e.target.value);
                setOffset(0);
              }}
              placeholder="Actor ID"
              aria-label="Lọc theo actor id"
              className="w-28 rounded-lg border border-[var(--border)] bg-[var(--card)] px-2 py-1.5 text-sm"
            />
          </div>
        }
      >
        <DataTable
          columns={columns}
          rows={entries}
          rowKey={(row) => row.id}
          loading={query.isLoading}
          empty={<EmptyState title="Không có bản ghi nào khớp bộ lọc" />}
        />
        <div className="mt-3 flex items-center justify-between text-sm">
          <button className="ghost-button" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
            ← Trước
          </button>
          <span className="text-[var(--muted-foreground)]">Trang {Math.floor(offset / PAGE_SIZE) + 1}</span>
          <button className="ghost-button" disabled={entries.length < PAGE_SIZE} onClick={() => setOffset(offset + PAGE_SIZE)}>
            Sau →
          </button>
        </div>
      </SectionCard>
    </div>
  );
}
