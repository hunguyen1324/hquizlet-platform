// AdminOverview — Phase 4 (ADM-03/ADM-04)
import React, { useState } from "react";
import { useQuery, useQueries } from "@tanstack/react-query";
import { DataTable, DateRangePicker, KpiCard, SectionCard, TimeSeriesChart, type RangeDays } from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { apiFetch } from "../../lib/api/client";
import { adminIamApi } from "../../lib/api";

export function AdminOverview() {
  const { token } = useAuth();
  const [days, setDays] = useState<number>(28);
  const query = useQuery({
    queryKey: ["admin-overview", days],
    queryFn: () => adminIamApi.overview(token, days),
    enabled: !!token,
    staleTime: 60_000,
  });

  type Metrics = {totals: Record<string,number>;previous: Record<string,number>;series:Array<{date:string;name:string;value:number}>};
  const metrics = useQueries({queries:["study-stats","class-stats","payments/stats"].map(path=>({queryKey:["admin-metrics",path,days],queryFn:()=>apiFetch<Metrics>(`/v1/admin/${path}`,token,{}, {range:`${days}d`}),enabled:!!token,staleTime:60000}))});
  const top=useQuery({queryKey:["admin-top-content"],queryFn:()=>apiFetch<{items:Array<{id:number;title:string;sessions:number;avgScorePct:number}>}>("/v1/admin/top-content",token),enabled:!!token,staleTime:60000});
  const audit=useQuery({queryKey:["admin-recent-audit"],queryFn:()=>adminIamApi.listAudit(token,{limit:5}),enabled:!!token,staleTime:60000});
  const kpis = query.data?.kpis;

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-xl font-semibold">Tổng quan</h1>
          <p className="text-sm text-[var(--muted-foreground)]">Số liệu toàn hệ thống so với kỳ trước</p>
        </div>
        <DateRangePicker value={days} onChange={(d: RangeDays) => setDays(d)} />
      </header>

      {query.isError && (
        <div className="message message--error" role="alert">
          Không tải được số liệu tổng quan.{" "}
          <button className="ghost-button" onClick={() => void query.refetch()}>Thử lại</button>
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard label="Tổng người dùng" value={kpis?.totalUsers ?? 0} loading={query.isLoading} />
        <KpiCard label="Người dùng mới" value={kpis?.newUsers ?? 0} delta={kpis?.newUsersDeltaPct} loading={query.isLoading} />
        <KpiCard
          label="Người dùng hoạt động"
          value={kpis?.activeUsers ?? 0}
          delta={kpis?.activeUsersDeltaPct}
          hint="Có phiên đăng nhập trong kỳ"
          loading={query.isLoading}
        />
        <KpiCard label="Gán quyền đang hiệu lực" value={kpis?.assignments ?? 0} loading={query.isLoading} />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard label="Sự kiện audit" value={kpis?.auditEvents ?? 0} loading={query.isLoading} />
        <KpiCard
          label="Lần bị từ chối"
          value={kpis?.deniedAttempts ?? 0}
          hint="Deny trong audit log"
          loading={query.isLoading}
        />
      </div>

      <SectionCard title="Người dùng mới & hoạt động" description={`Theo ngày trong ${days} ngày qua`}>
        <TimeSeriesChart
          data={(query.data?.series ?? []).map((p) => ({ date: p.date, newUsers: p.newUsers, activeUsers: p.activeUsers }))}
          series={[
            { key: "newUsers", label: "Người dùng mới", color: "#0ea5e9" },
            { key: "activeUsers", label: "Hoạt động", color: "#22c55e" },
          ]}
        />
      </SectionCard>

      <div className="grid gap-3 md:grid-cols-3">{metrics.map((m,i)=>{const key=["studySessions","newClasses","revenueVnd"][i];const current=m.data?.totals[key]??0, previous=m.data?.previous[key]??0;return <SectionCard key={key} title={["Phiên học","Lớp mới","Doanh thu học phần"][i]}>{m.isError?<p role="alert">Không tải được dữ liệu <button onClick={()=>void m.refetch()}>Thử lại</button></p>:<><KpiCard label={["Hoàn thành","Đã tạo","VND (sau hoàn tiền)"][i]} value={current} delta={previous?Math.round((current-previous)*100/previous):current?100:0} loading={m.isLoading}/><TimeSeriesChart height={130} data={(m.data?.series??[]).map(p=>({date:p.date,value:p.value}))} series={[{key:"value",label:["Phiên học","Lớp","VND"][i],color:"#6366f1"}]} /></>}</SectionCard>})}</div>
      <SectionCard title="Nội dung nổi bật (28 ngày)">{top.isError?<p role="alert">Không tải được nội dung nổi bật</p>:<DataTable columns={[{key:"title",header:"Học phần"},{key:"sessions",header:"Phiên học"},{key:"avgScorePct",header:"Điểm trung bình (%)"}]} rows={top.data?.items??[]} rowKey={r=>r.id} loading={top.isLoading}/>}</SectionCard>
      <SectionCard title="Hoạt động quản trị mới nhất">{audit.isError?<p role="alert">Không tải được hoạt động</p>:<ul>{audit.data?.entries.map(e=><li key={e.id}>#{e.actorId} · {e.action} · {e.resource} · {new Date(e.at).toLocaleString("vi-VN")}</li>)}</ul>}</SectionCard>
    </div>
  );
}
