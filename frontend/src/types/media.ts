export interface Media {
  id: string;
  title: string;
  original_name: string;
  year: number;
  quality: string;
  genre: string;
  file_path: string;
  file_size: number;
  duration: number;
  mime_type: string;
  thumbnail_path: string;
  status: 'pending' | 'downloading' | 'processing' | 'ready' | 'error';
  source: 'torrent' | 'upload' | 'scan';
  language: string;
  created_at: string;
  updated_at: string;
}

export interface LibraryStats {
  count: number;
  total_size: number;
}

export interface TorrentStatus {
  media_id: string;
  title: string;
  status: string;
  total_bytes: number;
  completed_bytes: number;
  progress_pct: number;
  download_rate_bps: number;
  peers: number;
}

export interface TorrentTarget {
  title: string;
  size: string;
  link: string;
}
