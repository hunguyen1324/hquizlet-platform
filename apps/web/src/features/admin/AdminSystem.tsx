// AdminSystem — Phase 4 (ADM-09)
// Service health from the gateway's /healthz/services fan-out. Import-queue and error
// counters are not exposed yet; the page states that plainly instead of inventing numbers.
import React from "react";
import { useQuery } from "@tanstack/react-query";
import { DataTable, SectionCard, type Column } from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { apiFetch } from "../../lib/api/client";
import { healthApi } from "../../lib/api";
import type { ServiceHealth } from "../../types";

export function AdminSystem() {
 const {token}=useAuth();
 const system=useQuery({queryKey:["admin-study-system"],queryFn:()=>apiFetch<{importQueue: Record<string,number>;recentErrors: Array<{id:number;message:string;at:string}>}>("/v1/admin/study-system",token),enabled:!!token,refetchInterval:15000});
  const query = useQuery({
    queryKey: ["admin-system-health"],
    queryFn: () => healthApi.services(),
    refetchInterval: 15_000,
  });

  const services = query.data?.services ?? [];

  const columns: Array<Column<ServiceHealth>> = [
    { key: "name", header: "Dịch vụ" },
    { key: "url", header: "Địa chỉ" },
    {
      key: "status",
      header: "Trạng thái",
      render: (row) => (
        <span className={row.status === "ok" ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400"}>
          {row.status === "ok" ? "● hoạt động" : "● lỗi"}
        </span>
      ),
    },
  ];

  const down = services.filter((s) => s.status !== "ok").length;

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold">Hệ thống</h1>
        <p className="text-sm text-[var(--muted-foreground)]">
          {services.length} dịch vụ · {down} đang lỗi · tự làm mới mỗi 15 giây
        </p>
      </header>

      {query.isError && (
        <div className="message message--error" role="alert">
          Gateway không phản hồi.
          <button className="ghost-button ml-2" onClick={() => void query.refetch()}>
            Thử lại
          </button>
        </div>
      )}

      <SectionCard title="Trạng thái dịch vụ" description="Nguồn: GET /healthz/services">
        <DataTable columns={columns} rows={services} rowKey={(row) => row.name} loading={query.isLoading} />
      </SectionCard>

      <SectionCard title="Hàng đợi import">
        {system.isError ? <p role="alert">Không tải được hàng đợi <button onClick={()=>void system.refetch()}>Thử lại</button></p> : system.isLoading ? <p>Đang tải…</p> : <p>Chờ: {system.data?.importQueue.pending??0} · Đang chạy: {system.data?.importQueue.running??0} · Hoàn tất: {system.data?.importQueue.done??0} · Lỗi: {system.data?.importQueue.failed??0}</p>}
      </SectionCard>
      <SectionCard title="Lỗi import gần đây"><ul>{system.data?.recentErrors.map(e=><li key={e.id}>#{e.id}: {e.message}</li>)}</ul>{!system.isLoading && !system.isError && !system.data?.recentErrors.length && <p>Chưa có lỗi import</p>}</SectionCard>
    </div>
  );
}
