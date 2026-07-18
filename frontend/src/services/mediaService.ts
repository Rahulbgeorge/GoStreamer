import type { Media, LibraryStats, TorrentStatus, TorrentTarget, Preference, BrowseData } from '../types/media';

export let API_BASE = (import.meta.env.VITE_API_BASE as string) || `/api/v1`;

export function setApiBase(url: string) {
  API_BASE = url;
}

export async function initializeApiBase() {
  // Determine the default server URL based on build config or current page origin
  let defaultBase = '/api/v1'; // fallback relative path
  if (import.meta.env.VITE_API_BASE) {
    defaultBase = import.meta.env.VITE_API_BASE;
  } else if (window.location.origin && window.location.origin.startsWith('http')) {
    // If running in development/local on localhost:3000, default base to port 8000
    if (window.location.origin.includes('localhost') || window.location.origin.includes('127.0.0.1')) {
      defaultBase = `${window.location.protocol}//${window.location.hostname}:8000/api/v1`;
    } else {
      defaultBase = `${window.location.origin}/api/v1`;
    }
  }

  // Set default first so that we have a working fallback
  setApiBase(defaultBase);
  console.log(`Setting initial default API base to: ${defaultBase}`);

  // Fetch local IP info from the current default server
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 2000);
    const res = await fetch(`${defaultBase}/local-ip`, { signal: controller.signal });
    clearTimeout(timeoutId);

    if (res.ok) {
      const json = await res.json();
      const localUrl = json.local_url; // e.g. "http://192.168.1.5:8000"
      
      if (localUrl) {
        const localApiBase = `${localUrl}/api/v1`;
        
        // If they are identical (e.g. we connected directly via local IP), no switch needed
        if (localApiBase.replace(/\/$/, '') === defaultBase.replace(/\/$/, '')) {
          console.log(`Already connected directly via local IP: ${localApiBase}`);
          return;
        }

        console.log(`Discovered server local IP URL: ${localApiBase}. Probing connection speed...`);

        // Test if the local IP base is directly accessible by the client (Ping probe)
        try {
          const pingCtrl = new AbortController();
          const pingTimeout = setTimeout(() => pingCtrl.abort(), 1500); // quick timeout
          const pingRes = await fetch(`${localApiBase}/ping`, { signal: pingCtrl.signal });
          clearTimeout(pingTimeout);

          if (pingRes.ok) {
            console.log(`Ping to local IP base succeeded. Switching API base to local IP: ${localApiBase} for faster performance.`);
            setApiBase(localApiBase);
            return;
          }
        } catch (pingErr) {
          console.log(`Ping to local IP base failed. Client is likely remote. Continuing with default base: ${defaultBase}`);
        }
      }
    }
  } catch (err) {
    console.log(`Failed to fetch local IP info from default base: ${defaultBase}. Falling back to default.`);
  }

  // Capacitor / Mobile fallback probe if default Base is localhost and failed
  if (defaultBase.includes('localhost') || defaultBase.includes('127.0.0.1')) {
    const fallbackIp = `192.168.29.142`;
    const fallbackBases = [
      `http://${fallbackIp}:8000/api/v1`,
      `http://${fallbackIp}:8080/api/v1`,
      `http://${fallbackIp}:80/api/v1`
    ];
    for (const base of fallbackBases) {
      try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 1500);
        const res = await fetch(`${base}/ping`, { signal: controller.signal });
        clearTimeout(timeoutId);
        if (res.ok) {
          console.log(`Reached fallback local server at ${base}. Setting API_BASE.`);
          setApiBase(base);
          return;
        }
      } catch (err) {
        // try next
      }
    }
  }
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
  },

  async getPreferences(): Promise<Preference[]> {
    const res = await fetch(`${API_BASE}/preferences`);
    const json = await res.json();
    return json.data || [];
  },

  async setPreference(key: string, value: string): Promise<Preference> {
    const res = await fetch(`${API_BASE}/preferences`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key, value })
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error || 'Failed to update preference');
    }
    return json.data;
  },

  async browseDirectory(path?: string): Promise<BrowseData> {
    const query = path ? `?path=${encodeURIComponent(path)}` : '';
    const res = await fetch(`${API_BASE}/system/browse${query}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error || 'Failed to browse directory');
    }
    return json.data;
  }
};
