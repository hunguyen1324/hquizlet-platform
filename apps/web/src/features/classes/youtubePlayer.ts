export type YouTubePlayer = {
    playVideo: () => void;
    pauseVideo: () => void;
    seekTo: (seconds: number, allowSeekAhead: boolean) => void;
    getCurrentTime: () => number;
    getPlayerState: () => number;
    getDuration: () => number;
    setPlaybackRate: (rate: number) => void;
    getAvailablePlaybackRates: () => number[];
    destroy: () => void;
    getIframe: () => HTMLIFrameElement;
};
type YouTubeEvent = {
    target: YouTubePlayer;
    data: number;
};
export type YouTubeAPI = {
    Player: new (element: HTMLElement, options: {
        host: string;
        videoId: string;
        playerVars: Record<string, string | number>;
        events: {
            onReady: (event: YouTubeEvent) => void;
            onStateChange: (event: YouTubeEvent) => void;
            onError: (event: YouTubeEvent) => void;
        };
    }) => YouTubePlayer;
};
declare global {
    interface Window {
        YT?: YouTubeAPI;
        onYouTubeIframeAPIReady?: () => void;
    }
}
let pending: Promise<YouTubeAPI> | undefined;
export function loadYouTubeAPI(): Promise<YouTubeAPI> {
    if (window.YT?.Player)
        return Promise.resolve(window.YT);
    if (pending)
        return pending;
    pending = new Promise((resolve, reject) => {
        const previous = window.onYouTubeIframeAPIReady;
        let finished = false;
        const cleanup = () => { clearTimeout(timer); if (window.onYouTubeIframeAPIReady === ready)
            window.onYouTubeIframeAPIReady = previous; };
        const ready = () => { if (finished)
            return; finished = true; cleanup(); try {
            previous?.();
        }
        catch { } ; if (window.YT?.Player)
            resolve(window.YT);
        else {
            pending = undefined;
            reject(Error('YouTube chưa sẵn sàng.'));
        } };
        const fail = () => { if (finished)
            return; finished = true; cleanup(); pending = undefined; reject(Error('Không tải được trình phát YouTube. Kiểm tra mạng rồi mở lại video.')); };
        const timer = window.setTimeout(fail, 15000);
        window.onYouTubeIframeAPIReady = ready;
        const existing = document.querySelector<HTMLScriptElement>('script[data-hquizlet-youtube-api]');
        if (existing)
            existing.remove();
        const script = document.createElement('script');
        script.src = 'https://www.youtube.com/iframe_api';
        script.async = true;
        script.dataset.hquizletYoutubeApi = '1';
        script.onerror = fail;
        document.head.appendChild(script);
    });
    return pending;
}
export function describeYouTubeError(code: number): string {
    if (code === 100)
        return 'Video không tồn tại hoặc không còn công khai.';
    if (code === 101 || code === 150)
        return 'Chủ video không cho phép phát nhúng. Bạn có thể mở video trên YouTube.';
    if (code === 153)
        return 'YouTube không xác minh được nguồn phát. Thử tải lại trang hoặc mở video trên YouTube.';
    return 'YouTube chưa phát được video này. Thử lại hoặc mở trên YouTube.';
}
