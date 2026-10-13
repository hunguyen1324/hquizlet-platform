// Shared dashboard components - Phase 2 (UI-03)
// KpiCard, TimeSeriesChart, Sparkline, DataTable, DateRangePicker, SectionCard.
import React from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

// ── KpiCard ───────────────────────────────────────────────────────────

export function KpiCard({
  label,
  value,
  delta,
  hint,
  loading = false,
}: {
  label: string;
  value: React.ReactNode;
  delta?: number;
  hint?: string;
  loading?: boolean;
}) {
  return (
    <div className="rounded-2xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
      <p className="text-xs font-medium uppercase tracking-wide text-[var(--muted-foreground)]">{label}</p>
      {loading ? (
        <div className="mt-2 h-8 w-20 animate-pulse rounded bg-[var(--muted)]" />
      ) : (
        <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
      )}
      {delta !== undefined && !loading && <DeltaBadge delta={delta} />}
      {hint && <p className="mt-1 text-xs text-[var(--muted-foreground)]">{hint}</p>}
    </div>
  );
}

export function DeltaBadge({ delta }: { delta: number }) {
  const up = delta >= 0;
  return (
    <p className={`mt-1 text-xs font-medium ${up ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400"}`}>
      {up ? "▲" : "▼"} {Math.abs(delta)}% so với kỳ trước
    </p>
  );
}

// ── Charts ────────────────────────────────────────────────────────────

export type TimePoint = { date: string; [key: string]: string | number };

function shortDate(value: string) {
  const parts = value.split("-");
  return parts.length === 3 ? `${parts[2]}/${parts[1]}` : value;
}

export function TimeSeriesChart({
  data,
  series,
  height = 220,
  variant = "area",
}: {
  data: TimePoint[];
  series: Array<{ key: string; label: string; color: string }>;
  height?: number;
  variant?: "area" | "line";
}) {
  if (data.length === 0) {
    return (
      <div className="flex h-[220px] items-center justify-center text-sm text-[var(--muted-foreground)]">
        Chưa có dữ liệu trong khoảng thời gian này
      </div>
    );
  }
  const common = (
    <>
      <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
      <XAxis dataKey="date" tickFormatter={shortDate} tick={{ fontSize: 11 }} stroke="var(--muted-foreground)" />
      <YAxis tick={{ fontSize: 11 }} stroke="var(--muted-foreground)" width={32} allowDecimals={false} />
      <Tooltip
        contentStyle={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 12, fontSize: 12 }}
        labelFormatter={(v) => shortDate(String(v))}
      />
    </>
  );
  return (
    <div style={{ width: "100%", height }}>
      <ResponsiveContainer>
        {variant === "area" ? (
          <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
            <defs>
              {series.map((s) => (
                <linearGradient key={s.key} id={`grad-${s.key}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={s.color} stopOpacity={0.35} />
                  <stop offset="100%" stopColor={s.color} stopOpacity={0.02} />
                </linearGradient>
              ))}
            </defs>
            {common}
            {series.map((s) => (
              <Area
                key={s.key}
                type="monotone"
                dataKey={s.key}
                name={s.label}
                stroke={s.color}
                strokeWidth={2}
                fill={`url(#grad-${s.key})`}
              />
            ))}
          </AreaChart>
        ) : (
          <LineChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
            {common}
            {series.map((s) => (
              <Line key={s.key} type="monotone" dataKey={s.key} name={s.label} stroke={s.color} strokeWidth={2} dot={false} />
            ))}
          </LineChart>
        )}
      </ResponsiveContainer>
    </div>
  );
}

export function Sparkline({ data, color = "#0ea5e9", height = 40 }: { data: number[]; color?: string; height?: number }) {
  if (data.length === 0) return <div style={{ height }} />;
  const points = data.map((value, index) => ({ index, value }));
  return (
    <div style={{ width: "100%", height }}>
      <ResponsiveContainer>
        <LineChart data={points} margin={{ top: 4, right: 0, bottom: 0, left: 0 }}>
          <Line type="monotone" dataKey="value" stroke={color} strokeWidth={2} dot={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

// ── Layout ────────────────────────────────────────────────────────────

export function SectionCard({
  title,
  description,
  actions,
  children,
  className = "",
}: {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={`rounded-2xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm ${className}`}>
      <header className="mb-3 flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold">{title}</h2>
          {description && <p className="mt-0.5 text-xs text-[var(--muted-foreground)]">{description}</p>}
        </div>
        {actions}
      </header>
      {children}
    </section>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: { label: string; onClick: () => void };
}) {
  return (
    <div className="rounded-xl border border-dashed border-[var(--border)] p-6 text-center">
      <p className="text-sm font-medium">{title}</p>
      {description && <p className="mt-1 text-xs text-[var(--muted-foreground)]">{description}</p>}
      {action && (
        <button className="primary-button mt-3" onClick={action.onClick}>
          {action.label}
        </button>
      )}
    </div>
  );
}

export const RANGE_OPTIONS = [7, 28, 90] as const;
export type RangeDays = (typeof RANGE_OPTIONS)[number];

export function DateRangePicker({ value, onChange }: { value: number; onChange: (days: RangeDays) => void }) {
  return (
    <div className="inline-flex overflow-hidden rounded-lg border border-[var(--border)] text-xs">
      {RANGE_OPTIONS.map((days) => (
        <button
          key={days}
          type="button"
          onClick={() => onChange(days)}
          aria-pressed={value === days}
          className={`px-3 py-1.5 font-medium transition-colors ${
            value === days ? "bg-[var(--primary,#0ea5e9)] text-white" : "bg-[var(--card)] hover:bg-[var(--muted)]"
          }`}
        >
          {days} ngày
        </button>
      ))}
    </div>
  );
}

// ── DataTable ─────────────────────────────────────────────────────────

export type Column<T> = {
  key: string;
  header: string;
  align?: "left" | "right";
  render?: (row: T) => React.ReactNode;
};

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading = false,
  empty,
  onRowClick,
}: {
  columns: Array<Column<T>>;
  rows: T[];
  rowKey: (row: T) => string | number;
  loading?: boolean;
  empty?: React.ReactNode;
  onRowClick?: (row: T) => void;
}) {
  if (loading) {
    return (
      <div className="space-y-2" aria-busy="true">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-9 animate-pulse rounded bg-[var(--muted)]" />
        ))}
      </div>
    );
  }
  if (rows.length === 0) {
    return <>{empty ?? <p className="p-4 text-sm text-[var(--muted-foreground)]">Không có dữ liệu.</p>}</>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-[var(--border)] text-xs uppercase tracking-wide text-[var(--muted-foreground)]">
            {columns.map((c) => (
              <th key={c.key} className={`px-3 py-2 font-medium ${c.align === "right" ? "text-right" : ""}`}>
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={rowKey(row)}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
              className={`border-b border-[var(--border)] last:border-0 ${onRowClick ? "cursor-pointer hover:bg-[var(--muted)]" : ""}`}
            >
              {columns.map((c) => (
                <td key={c.key} className={`px-3 py-2 ${c.align === "right" ? "text-right tabular-nums" : ""}`}>
                  {c.render ? c.render(row) : String((row as Record<string, unknown>)[c.key] ?? "")}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ── formatting helpers ────────────────────────────────────────────────

export function formatNumber(value: number | undefined | null) {
  return new Intl.NumberFormat("vi-VN").format(value ?? 0);
}

export function formatVnd(value: number | undefined | null) {
  return new Intl.NumberFormat("vi-VN", { style: "currency", currency: "VND", maximumFractionDigits: 0 }).format(value ?? 0);
}

export function formatDateTime(value?: string | null) {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("vi-VN", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}
