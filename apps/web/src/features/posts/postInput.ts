import type { MediaInput } from "../../lib/api/media";
export function postTitle(body: string) {
  return Array.from(body.trim().split(/\r?\n/)[0]).slice(0, 200).join("");
}
export function postInput(
  body: string,
  visibility: MediaInput["visibility"],
  viewers: string,
  existingTitle?: string,
): MediaInput {
  const text = body.trim();
  if (!text) throw Error("Viết nội dung trước khi đăng bài.");
  if (Array.from(text).length > 10000)
    throw Error("Bài đăng tối đa 10.000 ký tự.");
  const ids =
    visibility === "selected" && viewers.trim()
      ? viewers
          .trim()
          .split(/[\s,]+/)
          .map(Number)
      : [];
  if (ids.some((id) => !Number.isSafeInteger(id) || id < 1))
    throw Error("Mã người xem phải là số nguyên dương.");
  if (visibility === "selected" && !ids.length)
    throw Error("Thêm ít nhất một người xem hoặc chọn Chỉ mình tôi.");
  return {
    kind: "post",
    title: existingTitle || postTitle(text),
    body: text,
    visibility,
    viewerIds: [...new Set(ids)],
  };
}
