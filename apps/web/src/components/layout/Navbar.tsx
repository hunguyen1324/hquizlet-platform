// Navbar — sticky top bar: logo, search, "+ Tạo", bell, user menu
import React, { useState, useRef, useEffect } from "react";

type NavbarProps = {
  user: { name: string; image?: string; id: string | number };
  onSearch?: (q: string) => void;
  onCreateSet: () => void;
  onLogout: () => void;
  onNavigate?: (view: string) => void;
};

export function Navbar({ user, onSearch, onCreateSet, onLogout, onNavigate }: NavbarProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [showCreateDropdown, setShowCreateDropdown] = useState(false);
  const [showUserMenu, setShowUserMenu] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const createRef = useRef<HTMLDivElement>(null);
  const userRef = useRef<HTMLDivElement>(null);

  // Close dropdowns on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (createRef.current && !createRef.current.contains(e.target as Node)) {
        setShowCreateDropdown(false);
      }
      if (userRef.current && !userRef.current.contains(e.target as Node)) {
        setShowUserMenu(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    onSearch?.(searchQuery.trim());
  }

  const initials = user.name
    .split(" ")
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);

  return (
    <nav className="sticky top-0 z-40 flex items-center h-16 px-4 md:px-6 border-b border-[var(--border)] bg-[var(--background)]/80 backdrop-blur-md">
      {/* Logo */}
      <button
        className="text-xl font-black text-[var(--primary)] mr-4 shrink-0"
        onClick={() => onNavigate?.("home")}
      >
        HQuizlet
      </button>

      {/* Search */}
      <form onSubmit={handleSearch} className="hidden md:flex flex-1 max-w-md">
        <div className="relative w-full">
          <svg
            className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--muted-foreground)]"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="search"
            placeholder="Tìm kiếm"
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              onSearch?.(e.target.value);
            }}
            className="w-full pl-10 pr-4 py-2 rounded-xl bg-[var(--muted)] border border-transparent text-sm focus:border-[var(--primary)] focus:outline-none transition-colors"
          />
        </div>
      </form>

      {/* Spacer */}
      <div className="flex-1 md:hidden" />

      {/* + Tạo button */}
      <div className="relative ml-2" ref={createRef}>
        <button
          onClick={() => {
            setShowCreateDropdown(!showCreateDropdown);
            setShowUserMenu(false);
          }}
          className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-amber-400 hover:bg-amber-500 text-slate-900 font-bold text-sm transition-colors"
        >
          <span className="text-lg leading-none">+</span>
          <span className="hidden sm:inline">Tạo</span>
        </button>
        {showCreateDropdown && (
          <div className="absolute right-0 top-full mt-2 w-52 bg-[var(--card)] border border-[var(--border)] rounded-xl shadow-lg py-1 z-50">
            <button
              className="w-full text-left px-4 py-2.5 hover:bg-[var(--accent)] text-sm font-medium"
              onClick={() => {
                setShowCreateDropdown(false);
                onNavigate?.("create-type");
              }}
            >
              📚 Thẻ học
            </button>
            <button
              className="w-full text-left px-4 py-2.5 hover:bg-[var(--accent)] text-sm font-medium"
              onClick={() => {
                setShowCreateDropdown(false);
                onNavigate?.("classes");
              }}
            >
              👥 Nhóm học
            </button>
          </div>
        )}
      </div>

      {/* Bell icon */}
      <button
        className="ml-2 p-2 rounded-lg hover:bg-[var(--accent)] transition-colors relative"
        onClick={() => onNavigate?.("activity")}
        aria-label="Thông báo"
      >
        <svg className="w-5 h-5 text-[var(--muted-foreground)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
        </svg>
      </button>

      {/* User avatar + dropdown */}
      <div className="relative ml-2" ref={userRef}>
        <button
          onClick={() => {
            setShowUserMenu(!showUserMenu);
            setShowCreateDropdown(false);
          }}
          className="w-9 h-9 rounded-full bg-[var(--primary)] text-white flex items-center justify-center text-sm font-bold hover:opacity-90 transition-opacity overflow-hidden"
        >
          {user.image ? (
            <img src={user.image} alt={user.name} className="w-full h-full object-cover" />
          ) : (
            initials
          )}
        </button>
        {showUserMenu && (
          <div className="absolute right-0 top-full mt-2 w-56 bg-[var(--card)] border border-[var(--border)] rounded-xl shadow-lg py-1 z-50">
            <div className="px-4 py-3 border-b border-[var(--border)]">
              <p className="text-sm font-bold truncate">{user.name}</p>
            </div>
            <button
              className="w-full text-left px-4 py-2.5 hover:bg-[var(--accent)] text-sm"
              onClick={() => {
                setShowUserMenu(false);
                onNavigate?.("profile");
              }}
            >
              👤 Profile
            </button>
            <button
              className="w-full text-left px-4 py-2.5 hover:bg-[var(--accent)] text-sm"
              onClick={() => {
                document.documentElement.classList.toggle("dark");
                setShowUserMenu(false);
              }}
            >
              🌙 Chế độ tối
            </button>
            <div className="border-t border-[var(--border)] my-1" />
            <button
              className="w-full text-left px-4 py-2.5 hover:bg-red-50 dark:hover:bg-red-900/20 text-sm text-red-600"
              onClick={() => {
                setShowUserMenu(false);
                onLogout();
              }}
            >
              🚪 Đăng xuất
            </button>
          </div>
        )}
      </div>

      {/* Mobile hamburger */}
      <button
        className="ml-2 p-2 rounded-lg hover:bg-[var(--accent)] md:hidden"
        onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
        aria-label="Menu"
      >
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>
    </nav>
  );
}
