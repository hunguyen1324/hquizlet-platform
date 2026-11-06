// AppShell — unified layout: Navbar + Sidebar + main
import React, { useEffect, useRef, useState } from "react";
import { Navbar } from "./Navbar";
import { Sidebar } from "./Sidebar";
import { useAuth } from "../../features/auth/AuthContext";
import type { StudySet } from "../../types";
import "./layout.css";

type AppShellProps = {
  currentView: string;
  onNavigate: (view: string) => void;
  onCreateFolder?: () => void;
  onSelectSet?: (set: StudySet) => void;
  children: React.ReactNode;
};

export function AppShell({ currentView, onNavigate, onCreateFolder, onSelectSet, children }: AppShellProps) {
  const { user, logout } = useAuth();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const mainRef = useRef<HTMLElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const toggleRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    mainRef.current?.scrollTo({ top: 0 });
    setNavigationOpen(false);
  }, [currentView]);

  useEffect(() => {
    const desktop = window.matchMedia("(min-width: 769px)");
    const closeOnDesktop = () => { if (desktop.matches) setNavigationOpen(false); };
    desktop.addEventListener("change", closeOnDesktop);
    return () => desktop.removeEventListener("change", closeOnDesktop);
  }, []);

  useEffect(() => {
    if (!navigationOpen) return;
    menuRef.current?.querySelector<HTMLButtonElement>("button")?.focus();
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setNavigationOpen(false); toggleRef.current?.focus(); }
      if (event.key === "Tab") {
        const buttons = menuRef.current?.querySelectorAll<HTMLButtonElement>("button");
        if (!buttons?.length) return;
        const first = buttons[0];
        const last = buttons[buttons.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      }
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [navigationOpen]);
  if (!user) return null;

  const navigate = (view: string) => { setNavigationOpen(false); onNavigate(view); mainRef.current?.focus({ preventScroll: true }); };

  return (
    <div className={`hq-shell${navigationOpen ? " hq-navigation-open" : ""}`}>
      <a className="hq-skip-link" href="#main-content">Đến nội dung chính</a>
      <Navbar
        user={user}
        onSearch={() => onNavigate("dashboard")}
        onCreateSet={() => onNavigate("editor")}
        onLogout={() => void logout()}
        onNavigate={navigate}
        navigationOpen={navigationOpen}
        onToggleNavigation={() => setNavigationOpen(open => !open)}
        navigationToggleRef={toggleRef}
        onSelectSet={onSelectSet}
      />
      <div className="hq-body">
        {navigationOpen && <button className="hq-navigation-backdrop" aria-label="Đóng menu điều hướng" onClick={() => { setNavigationOpen(false); toggleRef.current?.focus(); }} />}
        <div className="hq-sidebar-container" id="primary-navigation" ref={menuRef} role={navigationOpen ? "dialog" : undefined} aria-modal={navigationOpen || undefined} aria-label="Điều hướng chính">
        <button className="hq-navigation-close" onClick={() => { setNavigationOpen(false); toggleRef.current?.focus(); }} aria-label="Đóng menu">Đóng menu <span aria-hidden="true">×</span></button>
        <Sidebar
          currentView={currentView}
          onNavigate={navigate}
          onCreateFolder={() => { setNavigationOpen(false); onCreateFolder?.(); }}
        />
        </div>
        <main className="hq-main" id="main-content" ref={mainRef} tabIndex={-1}>
          <div className="hq-content">{children}</div>
        </main>
      </div>
    </div>
  );
}
