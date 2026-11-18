import type { Visibility } from "../../types";

export type PersonalContentKind = "study" | "post" | "video";
export const personalContentLabels: Record<PersonalContentKind, string> = {
  study: "Học phần",
  post: "Bài đăng",
  video: "Video cá nhân",
};

function storageKey(userId: number, kind: PersonalContentKind) {
  return `hquizlet:visibility:${userId}:${kind}`;
}

export function readVisibilityDefault(userId: number | undefined, kind: PersonalContentKind): Visibility {
  try {
    return userId !== undefined && localStorage.getItem(storageKey(userId, kind)) === "private"
      ? "private"
      : "public";
  } catch {
    return "public";
  }
}

export function saveVisibilityDefault(userId: number, kind: PersonalContentKind, visibility: Visibility) {
  localStorage.setItem(storageKey(userId, kind), visibility);
}
