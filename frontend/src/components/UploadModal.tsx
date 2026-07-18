import React, { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { API_BASE, mediaService } from '../services/mediaService';
import { TorrentStatus, TorrentTarget, DirectoryItem } from '../types/media';
import './UploadModal.css';

interface UploadModalProps {
  onClose: () => void;
  onUploadSuccess: () => void;
}

export const UploadModal: React.FC<UploadModalProps> = ({ onClose, onUploadSuccess }) => {
  const { t } = useTranslation();
  const [file, setFile] = useState<File | null>(null);
  const [progress, setProgress] = useState<number>(0);
  const [status, setStatus] = useState<'idle' | 'uploading' | 'complete' | 'error'>('idle');

  const [activeTab, setActiveTab] = useState<'upload' | 'torrent' | 'scan' | 'youtube' | 'settings'>('upload');
  const [magnetUrl, setMagnetUrl] = useState('');
  const [torrentStatus, setTorrentStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [torrentError, setTorrentError] = useState('');

  const [scanStatus, setScanStatus] = useState<'idle' | 'scanning' | 'success' | 'error'>('idle');

  // YouTube tab state
  const [youtubeUrl, setYoutubeUrl] = useState('');
  const [youtubeTitle, setYoutubeTitle] = useState('');
  const [youtubeFormats, setYoutubeFormats] = useState<any[]>([]);
  const [selectedVideoItag, setSelectedVideoItag] = useState<number | ''>('');
  const [selectedAudioItag, setSelectedAudioItag] = useState<number | ''>('');
  const [youtubeStatus, setYoutubeStatus] = useState<'idle' | 'loading' | 'fetched' | 'downloading' | 'success' | 'error'>('idle');
  const [youtubeError, setYoutubeError] = useState('');

  const handleFetchYoutubeFormats = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!youtubeUrl.trim()) return;

    setYoutubeStatus('loading');
    setYoutubeError('');
    try {
      const data = await mediaService.listYoutubeFormats(youtubeUrl);
      setYoutubeTitle(data.title);
      setYoutubeFormats(data.formats || []);
      setYoutubeStatus('fetched');

      const videoFormats = (data.formats || []).filter((f: any) => f.height > 0);
      const audioFormats = (data.formats || []).filter((f: any) => f.height === 0 && (f.audio_quality || f.bitrate));

      // Find 1080p video if available, or fallback to first video format
      const v1080 = videoFormats.find((f: any) => f.height === 1080);
      const defaultVideo = v1080 || videoFormats[0];
      if (defaultVideo) {
        setSelectedVideoItag(defaultVideo.itag);
      } else {
        setSelectedVideoItag('');
      }

      // Find highest bitrate audio format
      let defaultAudio = audioFormats[0];
      for (const audio of audioFormats) {
        if (!defaultAudio || audio.bitrate > defaultAudio.bitrate) {
          defaultAudio = audio;
        }
      }
      if (defaultAudio) {
        setSelectedAudioItag(defaultAudio.itag);
      } else {
        setSelectedAudioItag('');
      }

    } catch (err: any) {
      setYoutubeStatus('error');
      setYoutubeError(err.message || 'Failed to fetch YouTube formats');
    }
  };

  const handleYoutubeDownload = async () => {
    setYoutubeStatus('downloading');
    setYoutubeError('');
    try {
      await mediaService.downloadYoutubeVideo(
        youtubeUrl,
        selectedVideoItag === '' ? undefined : selectedVideoItag,
        selectedAudioItag === '' ? undefined : selectedAudioItag
      );
      setYoutubeStatus('success');
      setTimeout(() => {
        onUploadSuccess();
        onClose();
      }, 2000);
    } catch (err: any) {
      setYoutubeStatus('error');
      setYoutubeError(err.message || 'Failed to start YouTube download');
    }
  };

  const handleScanDirectory = async () => {
    setScanStatus('scanning');
    try {
      await mediaService.scanDirectory();
      setScanStatus('success');
      setTimeout(() => {
        onUploadSuccess();
      }, 1500);
    } catch (err) {
      console.error(err);
      setScanStatus('error');
    }
  };

  // Active torrent download tracker
  const [activeTorrents, setActiveTorrents] = useState<TorrentStatus[]>([]);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Poll for active torrent downloads
  useEffect(() => {
    const fetchTorrents = async () => {
      try {
        const statuses = await mediaService.listActiveTorrents();
        setActiveTorrents(statuses);
      } catch {
        // silently handle
      }
    };

    fetchTorrents();
    pollRef.current = setInterval(fetchTorrents, 3000);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  // Settings tab state
  const [currentPath, setCurrentPath] = useState<string>('');
  const [parentPath, setParentPath] = useState<string>('');
  const [directories, setDirectories] = useState<DirectoryItem[]>([]);
  const [settingsStatus, setSettingsStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [settingsMsg, setSettingsMsg] = useState<string>('');

  // Scan folder browse state
  const [scanPath, setScanPath] = useState<string>('');
  const [scanDirectories, setScanDirectories] = useState<DirectoryItem[]>([]);
  const [scanParentPath, setScanParentPath] = useState<string>('');
  const [showScanBrowser, setShowScanBrowser] = useState<boolean>(false);
  const [scanBrowseStatus, setScanBrowseStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [scanBrowseMsg, setScanBrowseMsg] = useState<string>('');

  const fetchSettings = async () => {
    setSettingsStatus('loading');
    setSettingsMsg('');
    try {
      const prefs = await mediaService.getPreferences();
      const homedirPref = prefs.find(p => p.key === 'homedir');
      const startPath = homedirPref ? homedirPref.value : '';
      
      const browseData = await mediaService.browseDirectory(startPath || undefined);
      setCurrentPath(browseData.current_path);
      setParentPath(browseData.parent_path);
      setDirectories(browseData.directories || []);
      setSettingsStatus('idle');
    } catch (err: any) {
      setSettingsStatus('error');
      setSettingsMsg(err.message || 'Failed to fetch settings');
    }
  };

  const handleBrowse = async (path: string) => {
    setSettingsStatus('loading');
    setSettingsMsg('');
    try {
      const browseData = await mediaService.browseDirectory(path);
      setCurrentPath(browseData.current_path);
      setParentPath(browseData.parent_path);
      setDirectories(browseData.directories || []);
      setSettingsStatus('idle');
    } catch (err: any) {
      setSettingsStatus('error');
      setSettingsMsg(err.message || 'Failed to browse directory');
    }
  };

  const handleSaveSettings = async () => {
    setSettingsStatus('loading');
    setSettingsMsg('');
    try {
      await mediaService.setPreference('homedir', currentPath);
      setSettingsStatus('success');
      setSettingsMsg(t('settingsSuccess'));
      setTimeout(() => {
        setSettingsStatus('idle');
      }, 3000);
    } catch (err: any) {
      setSettingsStatus('error');
      setSettingsMsg(err.message || 'Failed to save settings');
    }
  };

  const fetchScanPath = async () => {
    try {
      const prefs = await mediaService.getPreferences();
      const homedirPref = prefs.find(p => p.key === 'homedir');
      const startPath = homedirPref ? homedirPref.value : '';
      setScanPath(startPath);
      return startPath;
    } catch (err) {
      console.error('Failed to fetch scan path preferences:', err);
    }
  };

  const handleBrowseScan = async (path: string) => {
    setScanBrowseStatus('loading');
    setScanBrowseMsg('');
    try {
      const browseData = await mediaService.browseDirectory(path);
      setScanPath(browseData.current_path);
      setScanParentPath(browseData.parent_path);
      setScanDirectories(browseData.directories || []);
      setScanBrowseStatus('idle');
    } catch (err: any) {
      setScanBrowseStatus('error');
      setScanBrowseMsg(err.message || 'Failed to browse directory');
    }
  };

  const handleSaveScanPath = async (pathToSave?: string) => {
    const targetPath = pathToSave || scanPath;
    setScanBrowseStatus('loading');
    setScanBrowseMsg('');
    try {
      await mediaService.setPreference('homedir', targetPath);
      setScanPath(targetPath);
      setScanBrowseStatus('success');
      setScanBrowseMsg(t('settingsSuccess'));
      setShowScanBrowser(false);
      setTimeout(() => {
        setScanBrowseStatus('idle');
      }, 3000);
    } catch (err: any) {
      setScanBrowseStatus('error');
      setScanBrowseMsg(err.message || 'Failed to save scan folder');
    }
  };

  const handleOpenScanBrowser = async () => {
    const current = await fetchScanPath();
    if (current !== undefined) {
      handleBrowseScan(current);
    }
    setShowScanBrowser(true);
  };

  useEffect(() => {
    if (activeTab === 'settings') {
      fetchSettings();
    } else if (activeTab === 'scan') {
      fetchScanPath();
    }
  }, [activeTab]);

  const [scannedLinks, setScannedLinks] = useState<TorrentTarget[]>([]);
  const [isScanningUrl, setIsScanningUrl] = useState(false);

  const handleTorrentDownload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!magnetUrl.trim()) return;

    const isUrl = magnetUrl.startsWith('http://') || magnetUrl.startsWith('https://');

    if (isUrl) {
      setIsScanningUrl(true);
      setTorrentStatus('idle');
      setTorrentError('');
      setScannedLinks([]);
      try {
        const links = await mediaService.scanTorrentURL(magnetUrl);
        setScannedLinks(links);
        if (links.length === 0) {
          setTorrentError('No magnet links found on this webpage.');
        }
      } catch (err: any) {
        setTorrentError(err.message || 'Failed to scan webpage');
      } finally {
        setIsScanningUrl(false);
      }
      return;
    }

    setTorrentStatus('loading');
    setTorrentError('');
    try {
      await mediaService.downloadTorrent(magnetUrl);
      setTorrentStatus('success');
      setMagnetUrl('');
      setScannedLinks([]);
      onUploadSuccess();
    } catch (err: any) {
      setTorrentStatus('error');
      setTorrentError(err.message || 'Failed to start download');
    }
  };

  const handleDownloadScannedLink = async (link: string) => {
    setTorrentStatus('loading');
    setTorrentError('');
    try {
      await mediaService.downloadTorrent(link);
      setTorrentStatus('success');
      setMagnetUrl('');
      setScannedLinks([]);
      onUploadSuccess();
    } catch (err: any) {
      setTorrentStatus('error');
      setTorrentError(err.message || 'Failed to start download');
    }
  };

  const handleCancelTorrent = async (mediaId: string) => {
    try {
      await mediaService.cancelTorrent(mediaId);
      setActiveTorrents(prev => prev.filter(t => t.media_id !== mediaId));
      onUploadSuccess();
    } catch (err: any) {
      console.error('Failed to cancel torrent:', err);
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      setFile(e.target.files[0]);
    }
  };

  const startUpload = async () => {
    if (!file) return;

    setStatus('uploading');
    setProgress(0);

    const CHUNK_SIZE = Math.round(1024 * 1024 * 2.5); // 2.5MB Chunks (faster/more reliable internet upload)
    const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
    
    try {
      // Step 1: Initialize Upload
      const initRes = await fetch(`${API_BASE}/upload/init`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filename: file.name, total_size: file.size })
      });
      const initJson = await initRes.json();
      const uploadID = initJson.data.upload_id;

      // Step 2: Upload Chunks Sequentially
      for (let chunkIdx = 0; chunkIdx < totalChunks; chunkIdx++) {
        const start = chunkIdx * CHUNK_SIZE;
        const end = Math.min(file.size, start + CHUNK_SIZE);
        const chunkBlob = file.slice(start, end);

        const formData = new FormData();
        formData.append('chunk', chunkBlob);
        formData.append('index', chunkIdx.toString());

        const chunkRes = await fetch(`${API_BASE}/upload/${uploadID}/chunk`, {
          method: 'POST',
          body: formData
        });

        if (!chunkRes.ok) throw new Error("Chunk upload failed");

        setProgress(Math.round(((chunkIdx + 1) / totalChunks) * 100));
      }

      // Step 3: Finalize Upload
      const completeRes = await fetch(`${API_BASE}/upload/${uploadID}/complete`, {
        method: 'POST'
      });
      if (!completeRes.ok) throw new Error("Assembly finalization failed");

      setStatus('complete');
      onUploadSuccess();
    } catch (err) {
      console.error(err);
      setStatus('error');
    }
  };

  const formatBytes = (bytes: number) => {
    if (bytes >= 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
    if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / 1024).toFixed(1)} KB`;
  };

  return (
    <div className="modal-backdrop">
      <div className="modal-content">
        <div className="modal-tabs">
          <button 
            className={`tab-btn ${activeTab === 'upload' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('upload');
              setStatus('idle');
            }}
          >
            {t('uploadTitle')}
          </button>
          <button 
            className={`tab-btn ${activeTab === 'torrent' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('torrent');
              setTorrentStatus('idle');
            }}
          >
            {t('torrentTitle')}
          </button>
          <button 
            className={`tab-btn ${activeTab === 'scan' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('scan');
              setScanStatus('idle');
            }}
          >
            {t('scanTitle')}
          </button>
          <button 
            className={`tab-btn ${activeTab === 'youtube' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('youtube');
              setYoutubeStatus('idle');
            }}
          >
            YouTube
          </button>
          <button 
            className={`tab-btn ${activeTab === 'settings' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('settings');
              setSettingsStatus('idle');
              setSettingsMsg('');
            }}
          >
            ⚙️ {t('settingsTitle') || 'Settings'}
          </button>
        </div>

        {activeTab === 'upload' && (
          <div className="tab-pane">
            {status === 'idle' && (
              <div className="upload-form">
                <p>{t('uploadHelp')}</p>
                <input type="file" accept="video/*" onChange={handleFileChange} />
                {file && (
                  <button className="btn-upload" onClick={startUpload}>
                    Upload {(file.size / (1024 * 1024)).toFixed(1)} MB
                  </button>
                )}
              </div>
            )}

            {status === 'uploading' && (
              <div className="progress-container">
                <p>Uploading... {progress}%</p>
                <div className="progress-bar">
                  <div className="progress-fill" style={{ width: `${progress}%` }}></div>
                </div>
              </div>
            )}

            {status === 'complete' && (
              <div className="upload-result">
                <p style={{ color: 'var(--success)' }}>Upload complete! Processing catalog updates.</p>
              </div>
            )}

            {status === 'error' && (
              <div className="upload-result">
                <p style={{ color: 'var(--accent)' }}>Failed to upload. Try checking network connection.</p>
              </div>
            )}
          </div>
        )}

        {activeTab === 'torrent' && (
          <div className="tab-pane">
            <form onSubmit={handleTorrentDownload} className="torrent-form">
              <p>
                {magnetUrl.startsWith('http://') || magnetUrl.startsWith('https://') 
                  ? 'Enter MovieRulz or other movie page URL to scan for magnet links' 
                  : t('torrentHelp') || 'Paste a magnet link to start background downloading'}
              </p>
              <textarea 
                className="torrent-input"
                placeholder="Paste magnet URI or webpage URL here..."
                value={magnetUrl}
                onChange={(e) => setMagnetUrl(e.target.value)}
                disabled={torrentStatus === 'loading' || isScanningUrl}
                required
              />
              
              {isScanningUrl && (
                <p className="status-msg loading" style={{ color: 'var(--accent)' }}>Scanning webpage for magnet links...</p>
              )}
              {torrentStatus === 'loading' && (
                <p className="status-msg loading">Adding torrent and initiating download...</p>
              )}
              {torrentStatus === 'success' && (
                <p className="status-msg success">Torrent download successfully started in the background!</p>
              )}
              {torrentStatus === 'error' && (
                <p className="status-msg error">{torrentError}</p>
              )}

              <button 
                type="submit" 
                className="btn-upload" 
                disabled={torrentStatus === 'loading' || isScanningUrl || !magnetUrl.trim()}
              >
                {magnetUrl.startsWith('http://') || magnetUrl.startsWith('https://') 
                  ? '🔍 Scan Webpage' 
                  : t('torrentBtn')}
              </button>
            </form>

            {scannedLinks.length > 0 && (
              <div className="scanned-torrents" style={{ marginTop: '20px', borderTop: '1px solid #333', paddingTop: '15px' }}>
                <h4 style={{ margin: '0 0 10px', color: '#ffa600' }}>🧲 Scanned Torrent Links:</h4>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', maxHeight: '200px', overflowY: 'auto' }}>
                  {scannedLinks.map((target, idx) => (
                    <div key={idx} style={{ 
                      display: 'flex', 
                      justifyContent: 'space-between', 
                      alignItems: 'center', 
                      background: '#151525', 
                      padding: '8px 12px', 
                      borderRadius: '6px',
                      border: '1px solid #334'
                    }}>
                      <div style={{ flex: 1, paddingRight: '10px', textAlign: 'left' }}>
                        <div style={{ fontSize: '12px', fontWeight: 'bold', color: '#fff', wordBreak: 'break-all' }}>{target.title}</div>
                        <span style={{ fontSize: '10px', background: '#333', padding: '2px 6px', borderRadius: '4px', marginTop: '4px', display: 'inline-block' }}>
                          {target.size}
                        </span>
                      </div>
                      <button 
                        onClick={() => handleDownloadScannedLink(target.link)}
                        className="btn-upload"
                        style={{ padding: '6px 12px', fontSize: '11px', margin: 0, width: 'auto' }}
                      >
                        📥 Download
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Active Torrent Downloads */}
            {activeTorrents.length > 0 && (
              <div className="active-torrents">
                <h4 style={{ margin: '16px 0 8px', color: '#ccc' }}>📥 Active Downloads</h4>
                {activeTorrents.map((torrent) => (
                  <div key={torrent.media_id} className="torrent-item">
                    <div className="torrent-item-header">
                      <span className="torrent-item-title">{torrent.title}</span>
                      <button 
                        className="torrent-cancel-btn"
                        onClick={() => handleCancelTorrent(torrent.media_id)}
                        title="Cancel download"
                      >
                        ✕
                      </button>
                    </div>
                    <div className="torrent-progress-bar">
                      <div 
                        className="torrent-progress-fill" 
                        style={{ width: `${Math.min(torrent.progress_pct, 100)}%` }}
                      ></div>
                    </div>
                    <div className="torrent-item-stats">
                      <span>{torrent.progress_pct.toFixed(1)}%</span>
                      <span>{formatBytes(torrent.completed_bytes)} / {formatBytes(torrent.total_bytes)}</span>
                      <span>{torrent.peers} peers</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {activeTab === 'scan' && (
          <div className="tab-pane">
            <div className="scan-form" style={{ display: 'flex', flexDirection: 'column', gap: '16px', padding: '16px 0' }}>
              <p>{t('scanHelp')}</p>
              
              <div className="scan-folder-selection" style={{ background: '#111122', padding: '14px', borderRadius: '8px', border: '1px solid #223', textAlign: 'left' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                  <span style={{ fontSize: '13px', color: '#888', fontWeight: 'bold' }}>Target Directory to Scan:</span>
                </div>
                
                <div style={{ display: 'flex', gap: '8px', marginBottom: '10px' }}>
                  <input 
                    type="text" 
                    className="path-input" 
                    value={scanPath || 'Default media folder'} 
                    onChange={(e) => setScanPath(e.target.value)} 
                    placeholder="Folder path..."
                    style={{ 
                      flex: 1, 
                      padding: '10px 12px', 
                      background: '#0a0a16', 
                      border: '1px solid #334', 
                      borderRadius: '6px', 
                      color: '#fff',
                      fontSize: '14px' 
                    }}
                  />
                  <button 
                    type="button"
                    className="btn-upload"
                    onClick={() => {
                      if (showScanBrowser) {
                        setShowScanBrowser(false);
                      } else {
                        handleOpenScanBrowser();
                      }
                    }}
                    style={{ 
                      margin: 0, 
                      padding: '10px 16px', 
                      width: 'auto', 
                      fontSize: '13px', 
                      display: 'flex', 
                      alignItems: 'center', 
                      gap: '6px',
                      background: 'var(--accent)'
                    }}
                  >
                    📁 {showScanBrowser ? 'Close' : 'Browse Server'}
                  </button>
                </div>

                {showScanBrowser && (
                  <div className="scan-browser-container" style={{ marginTop: '14px', borderTop: '1px solid #223', paddingTop: '14px' }}>
                    <div style={{ display: 'flex', gap: '8px', marginBottom: '10px' }}>
                      <input 
                        type="text" 
                        className="path-input" 
                        value={scanPath} 
                        onChange={(e) => setScanPath(e.target.value)} 
                        placeholder="Go to folder path..."
                        style={{ 
                          flex: 1,
                          padding: '8px 12px', 
                          background: '#0a0a16', 
                          border: '1px solid #334', 
                          borderRadius: '6px', 
                          color: '#fff',
                          fontSize: '13px'
                        }}
                      />
                      <button 
                        className="btn-browse-refresh"
                        onClick={() => handleBrowseScan(scanPath)}
                        style={{ padding: '8px 12px', background: '#223', border: '1px solid #334', borderRadius: '6px', color: '#fff', cursor: 'pointer' }}
                        title="Browse manually entered path"
                      >
                        🔄
                      </button>
                    </div>

                    {scanBrowseStatus === 'loading' && (
                      <div className="settings-loading" style={{ fontSize: '13px', color: '#888', margin: '8px 0' }}>Loading directory...</div>
                    )}

                    {scanBrowseStatus === 'error' && (
                      <p className="status-msg error" style={{ margin: '8px 0', fontSize: '13px' }}>{scanBrowseMsg}</p>
                    )}

                    {scanBrowseStatus === 'success' && (
                      <p className="status-msg success" style={{ margin: '8px 0', fontSize: '13px' }}>{scanBrowseMsg}</p>
                    )}

                    <div className="directory-browser" style={{ maxHeight: '200px', overflowY: 'auto', background: '#0a0a16', borderRadius: '6px', border: '1px solid #223', marginBottom: '12px' }}>
                      {scanParentPath && (
                        <div 
                          className="dir-item parent-dir" 
                          onClick={() => handleBrowseScan(scanParentPath)} 
                          style={{ padding: '10px 14px', cursor: 'pointer', display: 'flex', alignItems: 'center', background: 'rgba(255, 193, 7, 0.03)', borderBottom: '1px solid #223' }}
                        >
                          <span style={{ color: 'var(--warning, #ffc107)', fontWeight: 'bold' }}>⬆️ .. (Go Up)</span>
                        </div>
                      )}

                      <div className="dir-list">
                        {scanDirectories.length === 0 ? (
                          <div className="no-subdirs" style={{ padding: '16px', color: '#666', fontSize: '13px', textAlign: 'center' }}>No subdirectories found</div>
                        ) : (
                          scanDirectories.map((dir) => (
                            <div 
                              key={dir.path} 
                              className="dir-item folder-dir" 
                              style={{ 
                                display: 'flex', 
                                justifyContent: 'space-between', 
                                alignItems: 'center', 
                                padding: '10px 14px', 
                                borderBottom: '1px solid rgba(255, 255, 255, 0.03)'
                              }}
                            >
                              <span 
                                className="dir-name"
                                onClick={() => handleBrowseScan(dir.path)}
                                style={{ flex: 1, display: 'flex', alignItems: 'center', gap: '8px', color: '#fff', cursor: 'pointer' }}
                              >
                                📁 {dir.name}
                              </span>
                              <button
                                type="button"
                                onClick={() => handleSaveScanPath(dir.path)}
                                style={{ 
                                  background: 'rgba(255, 166, 0, 0.1)', 
                                  border: '1px solid rgba(255, 166, 0, 0.3)', 
                                  color: '#ffa600', 
                                  borderRadius: '4px', 
                                  padding: '4px 10px', 
                                  fontSize: '11px', 
                                  cursor: 'pointer',
                                  fontWeight: 'bold'
                                }}
                              >
                                Select
                              </button>
                            </div>
                          ))
                        )}
                      </div>
                    </div>

                    <div style={{ display: 'flex', gap: '8px' }}>
                      <button 
                        className="btn-upload" 
                        onClick={() => handleSaveScanPath(scanPath)}
                        disabled={scanBrowseStatus === 'loading' || !scanPath.trim()}
                        style={{ padding: '8px 14px', width: 'auto', fontSize: '12px' }}
                      >
                        💾 Use Current Folder
                      </button>
                      <button 
                        type="button" 
                        className="btn-close" 
                        onClick={() => setShowScanBrowser(false)}
                        style={{ padding: '8px 14px', fontSize: '12px', background: '#223', color: '#ccc', border: '1px solid #334', alignSelf: 'center', margin: 0 }}
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                )}
              </div>

              {scanStatus === 'idle' && (
                <button className="btn-upload" onClick={handleScanDirectory}>
                  🔍 {t('scanBtn')}
                </button>
              )}

              {scanStatus === 'scanning' && (
                <p className="status-msg loading" style={{ color: 'var(--accent)' }}>Scanning media folder and syncing database records...</p>
              )}

              {scanStatus === 'success' && (
                <p className="status-msg success" style={{ color: 'var(--success)' }}>{t('scanSuccess')}</p>
              )}

              {scanStatus === 'error' && (
                <p className="status-msg error" style={{ color: 'var(--accent)' }}>Failed to complete folder scan. Check backend logs.</p>
              )}
            </div>
          </div>
        )}

        {activeTab === 'youtube' && (
          <div className="tab-pane">
            <form onSubmit={handleFetchYoutubeFormats} className="torrent-form" style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <p>Paste a YouTube URL below to list resolutions and download the movie.</p>
              <textarea 
                className="torrent-input"
                placeholder="Paste YouTube video URL here (e.g. https://www.youtube.com/watch?v=...)"
                value={youtubeUrl}
                onChange={(e) => setYoutubeUrl(e.target.value)}
                disabled={youtubeStatus === 'loading' || youtubeStatus === 'downloading'}
                required
              />
              
              {youtubeStatus === 'idle' && (
                <button type="submit" className="btn-upload" disabled={!youtubeUrl.trim()}>
                  🔍 Fetch Available Qualities
                </button>
              )}
            </form>

            {youtubeStatus === 'loading' && (
              <p className="status-msg loading">Retrieving video stream details...</p>
            )}

            {youtubeStatus === 'error' && (
              <p className="status-msg error" style={{ marginTop: '10px' }}>{youtubeError}</p>
            )}

            {youtubeStatus === 'downloading' && (
              <p className="status-msg loading" style={{ color: 'var(--accent)' }}>YouTube download started in background! Video and audio streams are being merged...</p>
            )}

            {youtubeStatus === 'success' && (
              <p className="status-msg success">YouTube download successfully started! The file will appear in your library shortly.</p>
            )}

            {(youtubeStatus === 'fetched' || youtubeStatus === 'downloading' || youtubeStatus === 'success') && (
              <div className="youtube-options-panel" style={{ marginTop: '20px', borderTop: '1px solid #333', paddingTop: '15px', textAlign: 'left' }}>
                <h4 style={{ margin: '0 0 12px', color: '#ffa600' }}>🎬 {youtubeTitle}</h4>
                
                <div style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
                  <div>
                    <label style={{ display: 'block', marginBottom: '6px', fontSize: '13px', fontWeight: 'bold' }}>Resolution (Video Format):</label>
                    <select 
                      value={selectedVideoItag} 
                      onChange={(e) => setSelectedVideoItag(e.target.value ? Number(e.target.value) : '')}
                      style={{ 
                        width: '100%', 
                        padding: '10px', 
                        background: '#151525', 
                        color: '#fff', 
                        border: '1px solid #334', 
                        borderRadius: '6px',
                        fontSize: '13px'
                      }}
                      disabled={youtubeStatus === 'downloading' || youtubeStatus === 'success'}
                    >
                      <option value="">Default (1080p Fallback Chain)</option>
                      {youtubeFormats
                        .filter((f: any) => f.height > 0)
                        // Deduplicate resolutions to show them cleanly
                        .filter((f: any, idx: number, self: any[]) => self.findIndex((x: any) => x.quality_label === f.quality_label) === idx)
                        .map((f: any) => (
                          <option key={f.itag} value={f.itag}>
                            {f.quality_label || `${f.height}p`} ({f.mime_type.split(';')[0]}) - Itag {f.itag}
                          </option>
                        ))}
                    </select>
                  </div>

                  <div>
                    <label style={{ display: 'block', marginBottom: '6px', fontSize: '13px', fontWeight: 'bold' }}>Audio Quality:</label>
                    <select 
                      value={selectedAudioItag} 
                      onChange={(e) => setSelectedAudioItag(e.target.value ? Number(e.target.value) : '')}
                      style={{ 
                        width: '100%', 
                        padding: '10px', 
                        background: '#151525', 
                        color: '#fff', 
                        border: '1px solid #334', 
                        borderRadius: '6px',
                        fontSize: '13px'
                      }}
                      disabled={youtubeStatus === 'downloading' || youtubeStatus === 'success'}
                    >
                      <option value="">Default (Best Available Audio)</option>
                      {youtubeFormats
                        .filter((f: any) => f.height === 0 && (f.audio_quality || f.bitrate))
                        .map((f: any) => (
                          <option key={f.itag} value={f.itag}>
                            {f.audio_quality || 'AUDIO'} ({Math.round(f.bitrate / 1000)} kbps) - Itag {f.itag}
                          </option>
                        ))}
                    </select>
                  </div>

                  {youtubeStatus === 'fetched' && (
                    <button 
                      onClick={handleYoutubeDownload} 
                      className="btn-upload"
                      style={{ marginTop: '10px' }}
                    >
                      📥 Start Adaptive Download
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'settings' && (
          <div className="tab-pane settings-pane">
            <p className="settings-help">{t('settingsHelp')}</p>
            
            <div className="current-path-container">
              <span className="folder-icon">📁</span>
              <input 
                type="text" 
                className="path-input" 
                value={currentPath} 
                onChange={(e) => setCurrentPath(e.target.value)} 
                placeholder="Folder path..."
              />
              <button 
                className="btn-browse-refresh"
                onClick={() => handleBrowse(currentPath)}
                title="Browse manually entered path"
              >
                🔄
              </button>
            </div>

            {settingsStatus === 'loading' && (
              <div className="settings-loading">Loading directory...</div>
            )}

            {settingsStatus === 'error' && (
              <p className="status-msg error" style={{ margin: '8px 0' }}>{settingsMsg}</p>
            )}

            {settingsStatus === 'success' && (
              <p className="status-msg success" style={{ margin: '8px 0' }}>{settingsMsg}</p>
            )}

            <div className="directory-browser">
              {parentPath && (
                <div className="dir-item parent-dir" onClick={() => handleBrowse(parentPath)}>
                  <span>⬆️ .. (Go Up)</span>
                </div>
              )}

              <div className="dir-list">
                {directories.length === 0 ? (
                  <div className="no-subdirs">No subdirectories found</div>
                ) : (
                  directories.map((dir) => (
                    <div 
                      key={dir.path} 
                      className="dir-item folder-dir" 
                      onClick={() => handleBrowse(dir.path)}
                    >
                      <span className="dir-name">📁 {dir.name}</span>
                    </div>
                  ))
                )}
              </div>
            </div>

            <button 
              className="btn-upload btn-save-settings" 
              onClick={handleSaveSettings}
              disabled={settingsStatus === 'loading' || !currentPath.trim()}
              style={{ marginTop: '15px' }}
            >
              💾 {t('settingsSaveBtn') || 'Save Folder'}
            </button>
          </div>
        )}

        <button className="btn-close" onClick={onClose}>{t('cancelBtn')}</button>
      </div>
    </div>
  );
};
