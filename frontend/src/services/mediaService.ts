import type { Media, LibraryStats, TorrentStatus, TorrentTarget } from '../types/media';

export let API_BASE = (import.meta.env.VITE_API_BASE as string) || `/api/v1`;

export function setApiBase(url: string) {
  API_BASE = url;
}

export async function initializeApiBase() {
  // If VITE_API_BASE is set during build/runtime, use it directly in production
  if (import.meta.env.VITE_API_BASE) {
    setApiBase(import.meta.env.VITE_API_BASE);
    console.log(`API_BASE set from build-time VITE_API_BASE: ${API_BASE}`);
    return;
  }

  const ip = `192.168.29.142`;
  
  const bases = [
    `http://${ip}:8080/api/v1`,
    `http://${ip}:80/api/v1`
  ];

  if (window.location.origin && window.location.origin.startsWith('http') && !window.location.origin.includes('localhost') && !window.location.origin.includes('127.0.0.1')) {
    bases.unshift(`${window.location.origin}/api/v1`);
  }

  for (const base of bases) {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 1500);

      const res = await fetch(`${base}/local-ip`, { signal: controller.signal });
      clearTimeout(timeoutId);

      if (res.ok) {
        const json = await res.json();
        
        // If we are connecting to a public production endpoint, do not override it with private local_url
        if (base.startsWith('https://') || (window.location.origin && !window.location.origin.includes('localhost') && !window.location.origin.includes('127.0.0.1') && !window.location.origin.includes('192.168.'))) {
          console.log(`Successfully reached production/public server at ${base}. Keeping API_BASE as is.`);
          setApiBase(base);
          return;
        }

        const localUrl = json.local_url;
        if (localUrl) {
          console.log(`Successfully reached local server at ${localUrl}. Setting API_BASE.`);
          setApiBase(`${localUrl}/api/v1`);
          return;
        }
      }
    } catch (err) {
      // try next base URL
    }
  }

  // Fallback to the first base in the list
  setApiBase(bases[0]);
}

export const mediaService = {
  async getAllMedia(): Promise<Media[]> {
    const res = await fetch(`${API_BASE}/media`);
    const json = await res.json();
    return json.data || [];
  },

  async getMediaByID(id: string): Promise<Media | null> {
    const res = await fetch(`${API_BASE}/media/${id}`);
    if (res.status === 404) return null;
    const json = await res.json();
    return json.data;
  },

  async updateMedia(id: string, updates: Partial<Media>): Promise<Media> {
    const res = await fetch(`${API_BASE}/media/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updates)
    });
    const json = await res.json();
    return json.data;
  },

  async deleteMedia(id: string): Promise<boolean> {
    const res = await fetch(`${API_BASE}/media/${id}`, {
      method: 'DELETE'
    });
    const json = await res.json();
    return !!json.data;
  },

  async getStats(): Promise<LibraryStats> {
    const res = await fetch(`${API_BASE}/media/stats`);
    const json = await res.json();
    return json.data || { count: 0, total_size: 0 };
  },

  async search(query: string): Promise<Media[]> {
    const res = await fetch(`${API_BASE}/media/search?q=${encodeURIComponent(query)}`);
    const json = await res.json();
    return json.data || [];
  },

  async downloadTorrent(magnetUrl: string): Promise<Media> {
    const res = await fetch(`${API_BASE}/torrent/download`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ magnet_url: magnetUrl })
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error || 'Failed to start torrent download');
    }
    return json.data;
  },

  async listActiveTorrents(): Promise<TorrentStatus[]> {
    const res = await fetch(`${API_BASE}/torrent/status`);
    const json = await res.json();
    return json.data || [];
  },

  async getTorrentStatus(mediaId: string): Promise<TorrentStatus> {
    const res = await fetch(`${API_BASE}/torrent/status/${mediaId}`);
    const json = await res.json();
    return json.data;
  },

  async cancelTorrent(mediaId: string): Promise<boolean> {
    const res = await fetch(`${API_BASE}/torrent/cancel/${mediaId}`, {
      method: 'POST'
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error || 'Failed to cancel torrent');
    }
    return !!json.data;
  },

  getStreamUrl(id: string): string {
    return `${API_BASE}/stream/${id}`;
  },

  getThumbnailUrl(id: string): string {
    return `${API_BASE}/media/${id}/thumbnail`;
  },

  async getScrubberStatus(id: string): Promise<{ ready: boolean; interval: number; count: number }> {
    const res = await fetch(`${API_BASE}/media/${id}/scrubber`);
    const json = await res.json();
    return json.data || { ready: false, interval: 10, count: 0 };
  },

  getScrubberImageUrl(id: string, frame: number): string {
    return `${API_BASE}/media/${id}/scrubber/image/${frame}`;
  },

  async scanDirectory(): Promise<boolean> {
    const res = await fetch(`${API_BASE}/media/scan`, {
      method: 'POST'
    });
    const json = await res.json();
    return !!json.data;
  },

  async scanTorrentURL(pageURL: string): Promise<TorrentTarget[]> {
    const res = await fetch(`${API_BASE}/torrent/scan-url`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: pageURL })
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error || 'Failed to scan URL for magnets');
    }
    return json.data || [];
  },

  async listYoutubeFormats(url: string): Promise<{ title: string, formats: any[] }> {
    const rootBase = API_BASE.replace('/api/v1', '');
    const res = await fetch(`${rootBase}/youtube/list?url=${encodeURIComponent(url)}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error || 'Failed to list YouTube formats');
    }
    return json;
  },

  async downloadYoutubeVideo(url: string, videoItag?: number, audioItag?: number): Promise<any> {
    const rootBase = API_BASE.replace('/api/v1', '');
    const query = new URLSearchParams({ url });
    if (videoItag !== undefined) query.append('videoItag', videoItag.toString());
    if (audioItag !== undefined) query.append('audioItag', audioItag.toString());
    const res = await fetch(`${rootBase}/youtube/download?${query.toString()}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error || 'Failed to start YouTube download');
    }
    return json;
  }
};
