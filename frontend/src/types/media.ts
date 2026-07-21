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
  last_position?: number;
  default_start_time?: number;
  created_at: string;
  updated_at: string;
}

export interface Category {
  id: string;
  name: string;
  created_at: string;
}

export interface Clip {
  id: string;
  media_id: string;
  title: string;
  start_time: number;
  end_time: number;
  thumbnail_path: string;
  category_ids?: string[];
  categories?: Category[];
  created_at: string;
  updated_at: string;
}

export interface CreateClipPayload {
  media_id: string;
  title: string;
  start_time: number;
  end_time: number;
  category_ids?: string[];
  thumbnail_frame_time?: number;
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

export interface Preference {
  key: string;
  value: string;
}

export interface DirectoryItem {
  name: string;
  path: string;
}

export interface BrowseData {
  current_path: string;
  parent_path: string;
  directories: DirectoryItem[];
}

export interface Download {
  id: string;
  title: string;
  status: 'downloading' | 'completed' | 'failed' | 'cancelled';
  type: 'torrent' | 'youtube';
  progress: number;
  total_size: number;
  completed_size: number;
  download_speed: number;
  eta: string;
  dest_path: string;
  created_at: string;
  updated_at: string;
}
