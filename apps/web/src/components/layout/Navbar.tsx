// Navbar — unified design system, Quizlet-style
// PM: thống nhất với hq-* CSS classes, không dùng Tailwind inline
import React, { useState, useRef, useEffect } from "react";
import "./layout.css";

type NavbarProps = {
  user: { name: string; image?: string; id: string | number };
  onSearch?: (q: string) => void;
  onCreateSet: () => void;
  onLogout: () => void;
  onNavigate?: (view: string) => void;
};

export function Navbar({ user, onSearch, onCreateSet, onLogout, onNavigate }: NavbarProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [showUser, setShowUser] = useState(false);
  const createRef = useRef<HTMLDivElement>(null);
  const userRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (createRef.current && !createRef.current.contains(e.target as Node)) setShowCreate(false);
      if (userRef.current && !userRef.current.contains(e.target as Node)) setShowUser(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const initials = user.name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2);

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    onSearch?.(searchQuery.trim());
  }

  return (
    <nav className="hq-nav">
      {/* Logo */}
      <button className="hq-nav-logo" onClick={() => onNavigate?.("home")}>
        <div className="hq-nav-logo-mark">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <rect x="2" y="3" width="20" height="14" rx="2"/>
            <path d="M8 21h8M12 17v4"/>
          </svg>
        </div>
        <span className="hq-nav-logo-text">HQuizlet</span>
      </button>

      {/* Search */}
      <form className="hq-nav-search" onSubmit={handleSearch}>
        <div className="hq-nav-search-wrap">
          <svg className="hq-nav-search-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            className="hq-nav-search-input"
            type="search"
            placeholder="Tìm kiếm học phần, bài học…"
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              onSearch?.(e.target.value);
            }}
          />
        </div>
      </form>

      <div className="hq-nav-spacer" />

      <div className="hq-nav-actions">
        {/* + Tạo */}
        <div className="hq-dropdown-wrap" ref={createRef}>
          <button
            className="hq-nav-create"
            onClick={() => { setShowCreate((v) => !v); setShowUser(false); }}
          >
            <svg width="14" height="14" fill="none" stroke="currentColor" strokeWidth="3" viewBox="0 0 24 24">
              <path strokeLinecap="round" d="M12 5v14M5 12h14"/>
            </svg>
            <span>Tạo</span>
          </button>
          {showCreate && (
            <div className="hq-dropdown">
              <button className="hq-dropdown-item" onClick={() => { setShowCreate(false); onNavigate?.("create-type"); }}>
                <svg className="hq-di-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/></svg>
                Thẻ ghi nhớ mới
              </button>
              <button className="hq-dropdown-item" onClick={() => { setShowCreate(false); onNavigate?.("classes"); }}>
                <svg className="hq-di-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/></svg>
                Nhóm học mới
              </button>
            </div>
          )}
        </div>

        {/* Bell */}
        <button
          className="hq-nav-icon-btn"
          onClick={() => onNavigate?.("activity")}
          aria-label="Thông báo"
        >
          <svg width="18" height="18" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
          </svg>
        </button>

        {/* Avatar + user menu */}
        <div className="hq-dropdown-wrap" ref={userRef}>
          <button
            className="hq-nav-avatar-btn"
            onClick={() => { setShowUser((v) => !v); setShowCreate(false); }}
            aria-label="Tài khoản"
          >
            {user.image ? <img src={user.image} alt={user.name} /> : initials}
          </button>
          {showUser && (
            <div className="hq-dropdown">
              <div className="hq-dropdown-header">
                <p>{user.name}</p>
              </div>
              <button className="hq-dropdown-item" onClick={() => { setShowUser(false); onNavigate?.("profile"); }}>
                <svg className="hq-di-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><circle cx="12" cy="8" r="4"/><path d="M4 20c0-4 3.6-7 8-7s8 3 8 7"/></svg>
                Profile
              </button>
              <button className="hq-dropdown-item" onClick={() => { setShowUser(false); onNavigate?.("wallet"); }}>
                <svg className="hq-di-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path d="M21 12V7H5a2 2 0 010-4h14v4"/><path d="M3 5v14a2 2 0 002 2h16v-5"/><path d="M18 12a2 2 0 000 4h3v-4h-3z"/></svg>
                Ví của tôi
              </button>
              <button className="hq-dropdown-item" onClick={() => { document.documentElement.classList.toggle("dark"); setShowUser(false); }}>
                <svg className="hq-di-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path d="M21 12.79A9 9 0 1111.21 3 7 7 0 0021 12.79z"/></svg>
                Chế độ tối
              </button>
              <div className="hq-dropdown-divider" />
              <button className="hq-dropdown-item hq-dropdown-item--danger" onClick={() => { setShowUser(false); onLogout(); }}>
                <svg className="hq-di-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4M16 17l5-5-5-5M21 12H9"/></svg>
                Đăng xuất
              </button>
            </div>
          )}
        </div>

        {/* Mobile hamburger */}
        <button className="hq-nav-hamburger" aria-label="Menu">
          <svg width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2">
            <path strokeLinecap="round" d="M4 6h16M4 12h16M4 18h16"/>
          </svg>
        </button>
      </div>
    </nav>
  );
}
