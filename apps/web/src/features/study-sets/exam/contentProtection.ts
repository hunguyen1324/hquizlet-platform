import type { HTMLAttributes } from "react";

// Deterrent for ordinary copying; browser-delivered content remains inspectable.
export const contentProtection: HTMLAttributes<HTMLElement> = {
  style: { userSelect: "none", WebkitUserSelect: "none" },
  onCopy: (event) => event.preventDefault(),
  onCut: (event) => event.preventDefault(),
  onContextMenu: (event) => event.preventDefault(),
  onDragStart: (event) => event.preventDefault(),
};
