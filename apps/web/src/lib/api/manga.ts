import { apiFetch } from './client';
import { gatewayUrl } from './gateway';
export type MangaBlock = {
    box: [
        number,
        number,
        number,
        number
    ];
    vertical: boolean;
    font_size: number;
    lines: string[];
};
export type MangaPage = {
    img_width: number;
    img_height: number;
    blocks: MangaBlock[];
};
export type MangaPageResponse = {
    page: MangaPage;
    revision: number;
};
export type MangaVolume = {
    id: number;
    ownerId: number;
    sourceUuid: string;
    title: string;
    volume: string;
    visibility: 'private' | 'shared';
    status: 'importing' | 'ready';
    pageCount: number;
    uploaded: number;
    canEdit: boolean;
    canDelete: boolean;
    classId: number;
};
export type HTMLInspection = {
    sourceUuid: string;
    title: string;
    volume: string;
    pages: (MangaPage & {
        img_path: string;
    })[];
};
const path = (id: number) => `/v1/manga/${id}`;
const privateBlob = async (token: string, url: string, signal?: AbortSignal) => {
    const response = await fetch(`${gatewayUrl}${url}`, { signal, headers: { Authorization: `Bearer ${token}` } });
    if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        throw Error(error.message || error.error || 'Không tải được file. Vui lòng thử lại.');
    }
    return response.blob();
};
export const mangaApi = {
    move: (token: string,id: number,classId: number) => apiFetch<void>(`${path(id)}/move`,token,{method:"POST",body:JSON.stringify({classId})}),
    list: (token: string, classId?: number) => apiFetch<MangaVolume[]>('/v1/manga', token, {}, {classId}),
    inspect: (token: string, htmlSource: string, sourceName: string) => apiFetch<HTMLInspection>('/v1/manga/inspect', token, { method: 'POST', body: JSON.stringify({ htmlSource, sourceName }) }),
    create: (token: string, input: {
        sourceUuid: string;
        title: string;
        volume: string;
        pageCount: number;
        htmlSource?: string;
        sourceName?: string;
    }, classId?: number) => apiFetch<{
        id: number;
    }>('/v1/manga', token, { method: 'POST', body: JSON.stringify(input) }, {classId}),
    detail: (token: string, id: number) => apiFetch<{
        volume: MangaVolume;
        pages: number[];
    }>(path(id), token),
    page: (token: string, id: number, index: number, signal?: AbortSignal) => apiFetch<MangaPageResponse>(`${path(id)}/pages/${index}`, token, { signal }),
    upload: (token: string, id: number, index: number, page: MangaPage, image: File, signal?: AbortSignal) => { const body = new FormData(); body.append('page', JSON.stringify(page)); body.append('image', image); return apiFetch<void>(`${path(id)}/pages/${index}`, token, { method: 'POST', body, signal }); },
    complete: (token: string, id: number) => apiFetch<void>(`${path(id)}/complete`, token, { method: 'POST' }),
    edit: (token: string, id: number, index: number, revision: number, lines: string[][], restore = false) => apiFetch<MangaPageResponse>(`${path(id)}/pages/${index}`, token, { method: 'PUT', body: JSON.stringify({ revision, lines, restore }) }),
    update: (token: string, v: MangaVolume) => apiFetch<void>(path(v.id), token, { method: 'PUT', body: JSON.stringify({ title: v.title, volume: v.volume, visibility: v.visibility }) }),
    remove: (token: string, id: number) => apiFetch<void>(path(id), token, { method: 'DELETE' }),
    image: (token: string, id: number, index: number, signal?: AbortSignal) => privateBlob(token, `${path(id)}/pages/${index}/image`, signal),
    exportHTML: (token: string, id: number) => privateBlob(token, `${path(id)}/export.html`),
};
