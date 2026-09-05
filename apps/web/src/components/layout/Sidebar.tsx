// Sidebar — unified design system, hq-* classes
import React from "react";
import "./layout.css";

type SidebarProps = {
  currentView: string;
  onNavigate: (view: string) => void;
  onCreateFolder?: () => void;
};

const primaryNav = [
  {
    id: "home", label: "Trang chủ",
    icon: <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path strokeLinecap="round" strokeLinejoin="round" d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>,
  },
  {
    id: "dashboard", label: "Thư viện",
    icon: <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path strokeLinecap="round" strokeLinejoin="round" d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/></svg>,
  },
  {
    id: "classes", label: "Nhóm học",
    icon: <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/></svg>,
  },
  {
    id: "activity", label: "Thông báo",
    icon: <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path strokeLinecap="round" strokeLinejoin="round" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" /></svg>,
  },
];

const toolsNav = [
  {
    id: "live", label: "Live Quiz",
    icon: <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>,
  },
  {
    id: "folders", label: "Thư mục",
    icon: <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>,
  },
  {
    id: "wallet", label: "Ví của tôi",
    icon: <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path d="M21 12V7H5a2 2 0 010-4h14v4"/><path d="M3 5v14a2 2 0 002 2h16v-5"/><path d="M18 12a2 2 0 000 4h3v-4h-3z"/></svg>,
  },
];

export function Sidebar({ currentView, onNavigate, onCreateFolder }: SidebarProps) {
  return (
    <aside className="hq-sidebar">
      <div className="hq-sidebar-section">
        {primaryNav.map((item) => (
          <button
            key={item.id}
            className={`hq-sidebar-link${currentView === item.id ? " active" : ""}`}
            onClick={() => onNavigate(item.id)}
          >
            {item.icon}
            <span>{item.label}</span>
          </button>
        ))}
      </div>

      <div className="hq-sidebar-divider" />

      <div className="hq-sidebar-section">
        <span className="hq-sidebar-label">Công cụ</span>
        {toolsNav.map((item) => (
          <button
            key={item.id}
            className={`hq-sidebar-link${currentView === item.id ? " active" : ""}`}
            onClick={() => onNavigate(item.id)}
          >
            {item.icon}
            <span>{item.label}</span>
          </button>
        ))}
      </div>

      <div className="hq-sidebar-divider" />

      <button className="hq-sidebar-new-folder" onClick={onCreateFolder}>
        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2.5">
          <path strokeLinecap="round" d="M12 5v14M5 12h14"/>
        </svg>
        Thư mục mới
      </button>
    </aside>
  );
}
