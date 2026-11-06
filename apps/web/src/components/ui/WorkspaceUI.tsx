import type { ReactNode } from "react";

export type IconName = "book" | "wallet" | "arrow" | "plus" | "activity" | "folder" | "sparkles" | "check" | "clock";

export function WorkspaceIcon({ name, size = 22 }: { name: IconName; size?: number }) {
  const paths: Record<IconName, ReactNode> = {
    book: <><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20M6.5 3H20v19H6.5A2.5 2.5 0 0 1 4 19.5v-14A2.5 2.5 0 0 1 6.5 3Z" /><path d="M8 7h8M8 11h5" /></>,
    wallet: <><path d="M20 8V4H6a3 3 0 0 0-3 3v12a2 2 0 0 0 2 2h15V8H6a2 2 0 0 1 0-4" /><path d="M20 12h-5v5h5" /><path d="M16 14.5h.01" /></>,
    arrow: <><path d="M5 12h14m-6-6 6 6-6 6" /></>,
    plus: <path d="M12 5v14M5 12h14" />,
    activity: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M9 21h6" /><path d="M12 2V1" /></>,
    folder: <path d="M3 7V5a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z" />,
    sparkles: <><path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3Z" /><path d="M20 2v4m-2-2h4" /></>,
    check: <><circle cx="12" cy="12" r="9" /><path d="m8 12 3 3 5-6" /></>,
    clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  };
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

export function WorkspaceHeading({ eyebrow, title, description, action }: { eyebrow?: string; title: string; description?: string; action?: ReactNode }) {
  return <header className="workspace-heading"><div>{eyebrow && <p className="eyebrow">{eyebrow}</p>}<h1>{title}</h1>{description && <p className="workspace-description">{description}</p>}</div>{action && <div className="workspace-heading-actions">{action}</div>}</header>;
}

export function WorkspaceEmpty({ icon = "book", title, description, action }: { icon?: IconName; title: string; description?: string; action?: ReactNode }) {
  return <div className="workspace-empty"><div className="workspace-empty-art" aria-hidden="true"><span /><div><WorkspaceIcon name={icon} size={30} /></div><span /></div><h3>{title}</h3>{description && <p>{description}</p>}{action && <div className="workspace-empty-action">{action}</div>}</div>;
}
