import { apiFetch } from './client';
export type VideoCue = {
    start: number;
    end: number;
    text: string;
};
export type VideoSubtitleTrack = {
    language: string;
    label: string;
    source: 'uploaded' | 'youtube' | 'local';
    cues: VideoCue[];
};
export type VideoSubtitleResult = {
    original: VideoSubtitleTrack | null;
    translation: VideoSubtitleTrack | null;
    availableTracks: {
        language: string;
        label: string;
        automatic: boolean;
    }[];
    message?: string;
};
export const videoSubtitleApi = {
    get: (token: string, classId: number, videoId: number, options: {
        language?: string;
        translation?: string;
        youtube?: number;
    } = {}, signal?: AbortSignal) => apiFetch<VideoSubtitleResult>(`/v1/classes/${classId}/videos/${videoId}/subtitles`, token, { signal }, options),
    save: (token: string, classId: number, videoId: number, kind: 'original' | 'translation', track: VideoSubtitleTrack) => apiFetch<VideoSubtitleTrack>(`/v1/classes/${classId}/videos/${videoId}/subtitles`, token, { method: 'PUT', body: JSON.stringify({ kind, language: track.language, label: track.label, cues: track.cues }) }),
    remove: (token: string, classId: number, videoId: number, kind: 'original' | 'translation') => apiFetch<void>(`/v1/classes/${classId}/videos/${videoId}/subtitles`, token, { method: 'DELETE' }, { kind }),
};
