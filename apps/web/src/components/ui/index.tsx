// Shared UI components - Dev 3 (CORE-FB-04)
// Reusable: Button, EmptyState, LoadingSkeleton, ErrorAlert + Card, Badge, ProgressBar, Carousel

import React, { useRef, useState, useCallback } from "react";

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
    <div className="empty-panel">
      <h2>{title}</h2>
      {description && <p>{description}</p>}
      {action && (
        <button className="primary-button" onClick={action.onClick}>
          {action.label}
        </button>
      )}
    </div>
  );
}

export function LoadingSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div className="loading-skeleton" aria-busy="true">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="skeleton-row" style={{ opacity: 1 - i * 0.2 }} />
      ))}
    </div>
  );
}

export function ErrorAlert({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="error-alert" role="alert">
      <span>{message}</span>
      {onRetry && (
        <button className="ghost-button" onClick={onRetry}>
          Thử lại
        </button>
      )}
    </div>
  );
}

// ── Card ──────────────────────────────────────────────────────────────
export function Card({
  children,
  className = "",
  onClick,
}: {
  children: React.ReactNode;
  className?: string;
  onClick?: () => void;
}) {
  return (
    <div
      onClick={onClick}
      className={`bg-[var(--card)] text-[var(--card-foreground)] border border-[var(--border)] rounded-2xl shadow-sm ${onClick ? "cursor-pointer" : ""} ${className}`}
    >
      {children}
    </div>
  );
}

// ── Badge ──────────────────────────────────────────────────────────────
export type BadgeVariant = "sky" | "blue" | "green" | "amber" | "muted";

const badgeStyles: Record<BadgeVariant, string> = {
  sky: "bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300",
  blue: "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300",
  green: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300",
  amber: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300",
  muted: "bg-[var(--muted)] text-[var(--muted-foreground)]",
};

export function Badge({
  children,
  variant = "sky",
  className = "",
}: {
  children: React.ReactNode;
  variant?: BadgeVariant;
  className?: string;
}) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold ${badgeStyles[variant]} ${className}`}
    >
      {children}
    </span>
  );
}

// ── ProgressBar ────────────────────────────────────────────────────────
export function ProgressBar({
  value,
  className = "",
}: {
  value: number;
  className?: string;
}) {
  const clamped = Math.max(0, Math.min(100, value));
  return (
    <div className={`h-2 w-full rounded-full bg-[var(--muted)] overflow-hidden ${className}`}>
      <div
        className="h-full rounded-full bg-sky-400 transition-all duration-300"
        style={{ width: `${clamped}%` }}
      />
    </div>
  );
}

// ── Carousel ───────────────────────────────────────────────────────────
export function Carousel({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(true);

  const checkScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 2);
    setCanScrollRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 2);
  }, []);

  const scroll = (dir: "left" | "right") => {
    const el = scrollRef.current;
    if (!el) return;
    const amount = el.clientWidth * 0.7;
    el.scrollBy({ left: dir === "left" ? -amount : amount, behavior: "smooth" });
  };

  return (
    <div className={`relative group ${className}`}>
      {canScrollLeft && (
        <button
          onClick={() => scroll("left")}
          className="absolute left-0 top-1/2 -translate-y-1/2 z-10 w-9 h-9 rounded-full bg-white/90 dark:bg-slate-800/90 border border-[var(--border)] shadow-md flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity hover:bg-white dark:hover:bg-slate-700"
          aria-label="Scroll left"
        >
          ‹
        </button>
      )}
      <div
        ref={scrollRef}
        onScroll={checkScroll}
        className="flex gap-4 overflow-x-auto scroll-smooth snap-x snap-mandatory pb-2 scrollbar-none"
        style={{ scrollbarWidth: "none", msOverflowStyle: "none" }}
      >
        {children}
      </div>
      {canScrollRight && (
        <button
          onClick={() => scroll("right")}
          className="absolute right-0 top-1/2 -translate-y-1/2 z-10 w-9 h-9 rounded-full bg-white/90 dark:bg-slate-800/90 border border-[var(--border)] shadow-md flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity hover:bg-white dark:hover:bg-slate-700"
          aria-label="Scroll right"
        >
          ›
        </button>
      )}
    </div>
  );
}
