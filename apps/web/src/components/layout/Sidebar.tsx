// Sidebar — left nav: links, folders, quick-start
import React from "react";

type SidebarProps = {
  currentView: string;
  onNavigate: (view: string) => void;
  onCreateFolder?: () => void;
};

const navItems = [
  { id: "home", label: "Trang chủ", icon: "🏠" },
  { id: "dashboard", label: "Thư viện của bạn", icon: "📚" },
  { id: "classes", label: "Nhóm học", icon: "👥" },
  { id: "activity", label: "Thông báo", icon: "🔔" },
  { id: "wallet", label: "Ví của tôi", icon: "💰" },
];

const quickLinks = [
  { id: "editor", label: "Tạo thẻ ghi nhớ", icon: "📝" },
  { id: "live", label: "Live Quiz", icon: "⚡" },
  { id: "folders", label: "Thư mục", icon: "📁" },
];

export function Sidebar({ currentView, onNavigate, onCreateFolder }: SidebarProps) {
  return (
    <aside className="hidden md:flex flex-col w-60 shrink-0 border-r border-[var(--border)] bg-[var(--background)] overflow-y-auto sticky top-16 h-[calc(100vh-4rem)] p-4 gap-1">
      {/* Nav links */}
      <nav className="flex flex-col gap-0.5">
        {navItems.map((item) => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            className={`flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-semibold transition-colors text-left ${
              currentView === item.id
                ? "bg-[var(--primary)] text-white"
                : "hover:bg-[var(--accent)] text-[var(--foreground)]"
            }`}
          >
            <span className="text-base">{item.icon}</span>
            <span>{item.label}</span>
          </button>
        ))}
      </nav>

      {/* Divider */}
      <div className="border-t border-[var(--border)] my-3" />

      {/* Quick links */}
      <div className="flex flex-col gap-0.5">
        <p className="px-3 text-[11px] font-bold uppercase tracking-wider text-[var(--muted-foreground)] mb-1">
          Bắt đầu tại đây
        </p>
        {quickLinks.map((item) => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            className="flex items-center gap-3 px-3 py-2 rounded-xl text-sm font-medium hover:bg-[var(--accent)] transition-colors text-left text-[var(--foreground)]"
          >
            <span className="text-base">{item.icon}</span>
            <span>{item.label}</span>
          </button>
        ))}
      </div>

      {/* Divider */}
      <div className="border-t border-[var(--border)] my-3" />

      {/* Create folder */}
      <button
        onClick={onCreateFolder}
        className="flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium hover:bg-[var(--accent)] transition-colors text-left text-[var(--primary)]"
      >
        <span className="text-lg leading-none">+</span>
        <span>Thư mục mới</span>
      </button>
    </aside>
  );
}
