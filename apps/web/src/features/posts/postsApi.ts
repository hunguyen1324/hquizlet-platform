import { apiFetch } from "../../lib/api/client";
import { mediaApi, type MediaItem } from "../../lib/api/media";
export const postsApi = {
  list: (
    token: string,
    search: string,
    mine: boolean,
    offset: number,
    signal?: AbortSignal,
  ) =>
    apiFetch<MediaItem[]>(
      "/v1/media",
      token,
      { signal },
      { kind: "post", search, mine: String(mine), offset, limit: 20 },
    ),
  get: async (token: string, id: number, signal?: AbortSignal) => {
    const post = await apiFetch<MediaItem>(`/v1/media/${id}`, token, {
      signal,
    });
    if (post.kind !== "post") throw Error("Liên kết này không phải bài đăng.");
    return post;
  },
  save: mediaApi.save,
  remove: mediaApi.remove,
};
