import { gatewayUrl } from "./gateway";
import { apiFetch } from "./client";
export type MediaItem = {
  id: number;
  ownerId: number;
  kind: "video" | "post" | "playlist";
  title: string;
  body: string;
  source: string;
  url: string;
  embedUrl: string;
  thumbnailUrl: string;
  visibility: "public" | "private" | "selected";
  viewerIds: number[];
  createdAt: string;
};
export type MediaInput = Pick<
  MediaItem,
  "kind" | "title" | "body" | "visibility" | "viewerIds"
> & { url?: string };
export const mediaApi = {
  changePlaylistItem: (
    token: string,
    id: number,
    videoId: number,
    remove: boolean,
  ) =>
    apiFetch<void>(`/v1/media/${id}/items/${videoId}`, token, {
      method: remove ? "DELETE" : "POST",
    }),
  ownContent: (token: string, kind: "post" | "video", offset: number, limit: number, signal?: AbortSignal) =>
    apiFetch<MediaItem[]>("/v1/media", token, { signal }, { kind, mine: "true", offset, limit }),
  ownPlaylists: (token: string, signal?: AbortSignal) =>
    apiFetch<MediaItem[]>(
      "/v1/media",
      token,
      { signal },
      { kind: "playlist", mine: "true", limit: 100 },
    ),
  list: (
    token: string,
    kind: MediaItem["kind"],
    search = "",
    playlistId = 0,
    offset = 0,
    signal?: AbortSignal,
  ) =>
    apiFetch<MediaItem[]>(
      "/v1/media",
      token,
      { signal },
      { kind, search, playlistId, offset, limit: 60 },
    ),
  save: (token: string, input: MediaInput, id?: number) =>
    apiFetch<{ id: number }>(`/v1/media${id ? `/${id}` : ""}`, token, {
      method: id ? "PUT" : "POST",
      body: JSON.stringify(input),
    }),
  remove: (token: string, id: number) =>
    apiFetch<void>(`/v1/media/${id}`, token, { method: "DELETE" }),
  playlist: (token: string, id: number, videoIds: number[]) =>
    apiFetch<void>(`/v1/media/${id}/items`, token, {
      method: "PUT",
      body: JSON.stringify({ videoIds }),
    }),
  playback: (token: string, id: number) =>
    apiFetch<{ url: string }>(`/v1/media/${id}/playback`, token, {
      method: "POST",
    }),
  upload: (
    token: string,
    input: MediaInput,
    file: File,
    onProgress: (value: number) => void,
  ) =>
    new Promise<{ id: number }>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open(
        "POST",
        new URL("/v1/media/upload", gatewayUrl || location.origin).toString(),
      );
      xhr.setRequestHeader("Authorization", `Bearer ${token}`);
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable)
          onProgress(Math.round((e.loaded / e.total) * 100));
      };
      xhr.onerror = () => reject(Error("Mất kết nối khi tải video."));
      xhr.onload = () => {
        let data;
        try {
          data = JSON.parse(xhr.responseText);
        } catch {
          reject(Error("Không đọc được phản hồi tải lên."));
          return;
        }
        if (xhr.status < 200 || xhr.status >= 300)
          reject(Error(data.message || "Không tải được video."));
        else resolve(data);
      };
      const body = new FormData();
      body.append("metadata", JSON.stringify(input));
      body.append("file", file);
      xhr.send(body);
    }),
};
