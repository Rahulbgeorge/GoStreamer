import React, { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Media, Download, Category, Clip } from '../types/media';
import { mediaService } from '../services/mediaService';
import { VideoPlayer } from '../components/VideoPlayer';
import { MediaPicker } from '../components/MediaPicker';
import './AdminPage.css';

interface AdminPageProps {
  onBack: () => void;
}

export const AdminPage: React.FC<AdminPageProps> = ({ onBack }) => {
  const { t } = useTranslation();
  
  // Navigation tab state
  const [activeTab, setActiveTab] = useState<'tasks' | 'admin-video' | 'clips-studio' | 'clips-library' | 'home-rows'>('admin-video');
  
  // Data states
  const [mediaList, setMediaList] = useState<Media[]>([]);
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [activeTasks, setActiveTasks] = useState<Media[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [clips, setClips] = useState<Clip[]>([]);

  // Home Page Row Order State
  const [rowOrders, setRowOrders] = useState<Array<{ id: string; name: string; type: string; visible: boolean }>>([
    { id: 'hero_banner', name: '🌟 Spotlight Hero Banner', type: 'system', visible: true },
    { id: 'recently_added', name: '🕒 Recently Added Movies', type: 'system', visible: true },
    { id: 'cat_songs', name: '🎵 Category: Songs Row', type: 'category', visible: true },
    { id: 'cat_highlights', name: '⚡ Category: Highlights Row', type: 'category', visible: true },
    { id: 'cat_action', name: '🔥 Category: Action Row', type: 'category', visible: true },
    { id: 'cat_dialogues', name: '💬 Category: Dialogues Row', type: 'category', visible: true },
    { id: 'all_movies', name: '🎬 All Movies Grid', type: 'system', visible: true },
  ]);

  // Video category tagging state
  const [videoCategoryNames, setVideoCategoryNames] = useState<string[]>([]);
  
  // Selection & Video Player ref for Admin operations
  const [selectedMediaId, setSelectedMediaId] = useState<string>('');
  const [currentTime, setCurrentTime] = useState<number>(0);
  const videoRef = useRef<HTMLVideoElement | null>(null);

  // Status & notifications
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' | 'info' } | null>(null);

  // Clip Studio State
  const [clipTitle, setClipTitle] = useState('');
  const [clipStart, setClipStart] = useState<number | null>(null);
  const [clipEnd, setClipEnd] = useState<number | null>(null);
  const [selectedCategoryIds, setSelectedCategoryIds] = useState<string[]>([]);
  const [newCategoryName, setNewCategoryName] = useState('');
  const [savingClip, setSavingClip] = useState(false);

  // Filter state for Clips Library
  const [selectedCategoryFilter, setSelectedCategoryFilter] = useState<string>('all');

  // Active clip playback state
  const [playingClip, setPlayingClip] = useState<Clip | null>(null);

  const showStatus = (text: string, type: 'success' | 'error' | 'info' = 'info') => {
    setStatusMsg({ text, type });
    setTimeout(() => {
      setStatusMsg(null);
    }, 4000);
  };

  const fetchLibraryData = async () => {
    try {
      const allMedia = await mediaService.getAllMedia();
      setMediaList(allMedia);
      setSelectedMediaId(prevId => prevId || (allMedia.length > 0 ? allMedia[0].id : ''));

      const processingMedia = allMedia.filter(m => m.status === 'processing');
      setActiveTasks(processingMedia);

      const dlData = await mediaService.getDownloads();
      setDownloads(dlData);

      const catData = await mediaService.getCategories();
      setCategories(catData);

      const clipsData = await mediaService.getClips();
      setClips(clipsData);
    } catch (err) {
      console.error('Failed to fetch admin dashboard metrics:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLibraryData();
    const interval = setInterval(fetchLibraryData, 3000);
    return () => clearInterval(interval);
  }, []);

  const selectedMedia = mediaList.find(m => m.id === selectedMediaId) || null;

  const handleTimeUpdate = () => {
    if (videoRef.current) {
      setCurrentTime(videoRef.current.currentTime);
    }
  };

  // Video Admin Handlers
  const handleSetFrameAsThumbnail = async () => {
    if (!selectedMedia) return;
    const timeToCapture = videoRef.current ? videoRef.current.currentTime : currentTime;
    try {
      showStatus(`Extracting frame at ${formatTime(timeToCapture)} for poster thumbnail...`, 'info');
      const updatedMedia = await mediaService.setFrameThumbnail(selectedMedia.id, timeToCapture);
      setMediaList(prev => prev.map(m => m.id === updatedMedia.id ? updatedMedia : m));
      showStatus(`Successfully updated poster thumbnail for "${updatedMedia.title}"!`, 'success');
    } catch (err: any) {
      showStatus(`Failed to extract frame thumbnail: ${err.message}`, 'error');
    }
  };

  const handleSetDefaultStartTime = async () => {
    if (!selectedMedia) return;
    const startTimeSec = Math.floor(videoRef.current ? videoRef.current.currentTime : currentTime);
    try {
      const updatedMedia = await mediaService.setDefaultStartTime(selectedMedia.id, startTimeSec);
      setMediaList(prev => prev.map(m => m.id === updatedMedia.id ? updatedMedia : m));
      showStatus(`Default auto-play start time set to ${formatTime(startTimeSec)} for "${updatedMedia.title}"!`, 'success');
    } catch (err: any) {
      showStatus(`Failed to set default start time: ${err.message}`, 'error');
    }
  };

  // Clip Studio Handlers
  const handleMarkStart = () => {
    const timeVal = videoRef.current ? videoRef.current.currentTime : currentTime;
    setClipStart(timeVal);
    showStatus(`Start point marked at ${formatTime(timeVal)}`, 'info');
  };

  const handleMarkEnd = () => {
    const timeVal = videoRef.current ? videoRef.current.currentTime : currentTime;
    setClipEnd(timeVal);
    showStatus(`End point marked at ${formatTime(timeVal)}`, 'info');
  };

  const handleToggleCategory = (catId: string) => {
    setSelectedCategoryIds(prev => 
      prev.includes(catId) ? prev.filter(id => id !== catId) : [...prev, catId]
    );
  };

  const handleAddCategory = async () => {
    if (!newCategoryName.trim()) return;
    try {
      const newCat = await mediaService.createCategory(newCategoryName.trim());
      setCategories(prev => [...prev, newCat]);
      setSelectedCategoryIds(prev => [...prev, newCat.id]);
      setNewCategoryName('');
      showStatus(`Category "${newCat.name}" created!`, 'success');
    } catch (err: any) {
      showStatus(`Failed to create category: ${err.message}`, 'error');
    }
  };

  const handleSaveClip = async () => {
    if (!selectedMedia) {
      showStatus('Please select a video file first', 'error');
      return;
    }
    if (!clipTitle.trim()) {
      showStatus('Please enter a title for the clip', 'error');
      return;
    }
    if (clipStart === null || clipEnd === null) {
      showStatus('Please mark both Start Point and End Point', 'error');
      return;
    }
    if (clipEnd <= clipStart) {
      showStatus('End Point must be greater than Start Point', 'error');
      return;
    }

    setSavingClip(true);
    try {
      const newClip = await mediaService.createClip({
        media_id: selectedMedia.id,
        title: clipTitle.trim(),
        start_time: clipStart,
        end_time: clipEnd,
        category_ids: selectedCategoryIds,
        thumbnail_frame_time: clipStart
      });

      setClips(prev => [newClip, ...prev]);
      showStatus(`Clip "${newClip.title}" saved successfully!`, 'success');

      // Reset form
      setClipTitle('');
      setClipStart(null);
      setClipEnd(null);
      setSelectedCategoryIds([]);
    } catch (err: any) {
      showStatus(`Failed to save clip: ${err.message}`, 'error');
    } finally {
      setSavingClip(false);
    }
  };

  const handleDeleteClip = async (clipId: string) => {
    if (window.confirm('Are you sure you want to delete this clip?')) {
      try {
        await mediaService.deleteClip(clipId);
        setClips(prev => prev.filter(c => c.id !== clipId));
        showStatus('Clip deleted successfully', 'success');
      } catch (err: any) {
        showStatus(`Failed to delete clip: ${err.message}`, 'error');
      }
    }
  };

  const handleDeleteCategory = async (catId: string, catName: string) => {
    if (window.confirm(`Delete category "${catName}"? Clips will remain intact.`)) {
      try {
        await mediaService.deleteCategory(catId);
        setCategories(prev => prev.filter(c => c.id !== catId));
        showStatus(`Category "${catName}" deleted`, 'success');
      } catch (err: any) {
        showStatus(`Failed to delete category: ${err.message}`, 'error');
      }
    }
  };

  const handleTriggerScan = async () => {
    setScanning(true);
    showStatus('Scanning media library directory...', 'info');
    try {
      const res = await mediaService.scanDirectory();
      if (res) {
        showStatus('Scan completed successfully! Updated media catalog.', 'success');
        fetchLibraryData();
      }
    } catch (err: any) {
      showStatus(`Scan failed: ${err.message}`, 'error');
    } finally {
      setScanning(false);
    }
  };

  const handleCancelDownload = async (id: string) => {
    if (window.confirm('Cancel and delete this download record?')) {
      try {
        await mediaService.deleteDownload(id);
        fetchLibraryData();
      } catch (err: any) {
        showStatus(`Failed to cancel download: ${err.message}`, 'error');
      }
    }
  };

  const formatTime = (secs: number) => {
    const mins = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    const ms = Math.floor((secs % 1) * 10);
    return `${mins.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${ms}`;
  };

  const formatDuration = (secs: number) => {
    const mins = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    if (mins === 0) return `${s}s`;
    return `${mins}m ${s}s`;
  };

  const formatSpeed = (bps: number) => {
    if (bps <= 0) return '0 KB/s';
    const kbps = bps / 1000;
    if (kbps >= 1000) return `${(kbps / 1000).toFixed(1)} MB/s`;
    return `${kbps.toFixed(0)} KB/s`;
  };

  const formatSize = (bytes: number) => {
    if (bytes <= 0) return '0 MB';
    const mb = bytes / (1024 * 1024);
    if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GB`;
    return `${mb.toFixed(0)} MB`;
  };

  const filteredClips = selectedCategoryFilter === 'all' 
    ? clips 
    : clips.filter(c => c.category_ids?.includes(selectedCategoryFilter));

  const parentMediaForClip = (mediaId: string) => mediaList.find(m => m.id === mediaId);

  return (
    <div className="admin-page-wrapper">
      {/* Top Header */}
      <header className="admin-header">
        <div className="header-left">
          <button className="btn-back-nav" onClick={onBack}>
            ← {t('back')}
          </button>
          <h2>⚙️ Admin Control Panel</h2>
        </div>
        <div className="header-right">
          {statusMsg && (
            <div className={`status-banner banner-${statusMsg.type}`}>
              {statusMsg.text}
            </div>
          )}
          <button 
            className={`btn-scan-trigger ${scanning ? 'loading' : ''}`} 
            onClick={handleTriggerScan}
            disabled={scanning}
          >
            🔄 {scanning ? 'Scanning...' : 'Trigger Rescan'}
          </button>
        </div>
      </header>

      {/* Admin Sub-Header Tabs */}
      <nav className="admin-tabs-nav">
        <button 
          className={`tab-btn ${activeTab === 'admin-video' ? 'active' : ''}`}
          onClick={() => setActiveTab('admin-video')}
        >
          🎬 Poster Thumbnail & Auto-Play
        </button>
        <button 
          className={`tab-btn ${activeTab === 'clips-studio' ? 'active' : ''}`}
          onClick={() => setActiveTab('clips-studio')}
        >
          ✂️ Clip Studio (Check-in)
        </button>
        <button 
          className={`tab-btn ${activeTab === 'clips-library' ? 'active' : ''}`}
          onClick={() => setActiveTab('clips-library')}
        >
          📁 Clips & Categories ({clips.length})
        </button>
        <button 
          className={`tab-btn ${activeTab === 'home-rows' ? 'active' : ''}`}
          onClick={() => setActiveTab('home-rows')}
        >
          🏠 Home Rows & Category Layout
        </button>
        <button 
          className={`tab-btn ${activeTab === 'tasks' ? 'active' : ''}`}
          onClick={() => setActiveTab('tasks')}
        >
          ⚡ Tasks & Downloads ({downloads.length})
        </button>
      </nav>

      {/* TAB 1: FRAME THUMBNAIL & DEFAULT START TIME */}
      {activeTab === 'admin-video' && (
        <div className="tab-content admin-video-tab">
          <div className="admin-panel-card">
            <MediaPicker 
              mediaList={mediaList}
              selectedMediaId={selectedMediaId}
              onSelectMedia={(id) => setSelectedMediaId(id)}
              title="Select Video for Poster & Auto-Play Settings"
            />

            {selectedMedia ? (
              <div className="video-editor-wrapper">
                <div className="video-player-container">
                  <video 
                    ref={videoRef}
                    src={mediaService.getStreamUrl(selectedMedia.id)}
                    controls 
                    className="admin-preview-video"
                    onTimeUpdate={handleTimeUpdate}
                  />
                  <div className="current-timestamp-badge">
                    Current Frame: <span>{formatTime(currentTime)}</span>
                  </div>
                </div>

                <div className="controls-side-panel">
                  <h3>🎯 Video Frame & Playback Configuration</h3>
                  <p className="panel-subtext">
                    Scrub or play the video to any frame, then assign it as the poster thumbnail or default starting point.
                  </p>

                  <div className="action-card">
                    <h4>📸 Frame Poster Thumbnail</h4>
                    <p>Set current playing frame as the main library poster thumbnail image for this video.</p>
                    <button className="btn-admin-action thumbnail" onClick={handleSetFrameAsThumbnail}>
                      Set Frame ({formatTime(currentTime)}) as Poster Thumbnail
                    </button>
                  </div>

                  <div className="action-card">
                    <h4>⏱️ Default Auto-Play Start Position</h4>
                    <p>Set starting point where the video automatically begins playback when opened.</p>
                    {selectedMedia.default_start_time ? (
                      <div className="current-setting-badge">
                        Current Default Start: <strong>{formatTime(selectedMedia.default_start_time)}</strong>
                      </div>
                    ) : (
                      <div className="current-setting-badge none">Default Start: Beginning (00:00)</div>
                    )}
                    <div className="btn-group-row">
                      <button className="btn-admin-action starttime" onClick={handleSetDefaultStartTime}>
                        Set Position ({formatTime(currentTime)}) as Default Auto-Play
                      </button>
                      {(selectedMedia.default_start_time || 0) > 0 && (
                        <button className="btn-admin-action reset" onClick={() => {
                          mediaService.setDefaultStartTime(selectedMedia.id, 0);
                          setMediaList(prev => prev.map(m => m.id === selectedMedia.id ? { ...m, default_start_time: 0 } : m));
                          showStatus('Reset auto-play start point to 0s', 'info');
                        }}>
                          Reset to 0s
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            ) : (
              <div className="empty-section-card">No video selected or library is empty.</div>
            )}
          </div>
        </div>
      )}

      {/* TAB 2: CLIP STUDIO (CHECKIN START/END POINT) */}
      {activeTab === 'clips-studio' && (
        <div className="tab-content clips-studio-tab">
          <div className="admin-panel-card">
            <MediaPicker 
              mediaList={mediaList}
              selectedMediaId={selectedMediaId}
              onSelectMedia={(id) => {
                setSelectedMediaId(id);
                setClipStart(null);
                setClipEnd(null);
              }}
              title="Select Source Video for Clip Studio"
            />

            {selectedMedia ? (
              <div className="clip-studio-grid">
                {/* Video Player */}
                <div className="studio-video-wrap">
                  <video 
                    ref={videoRef}
                    src={mediaService.getStreamUrl(selectedMedia.id)}
                    controls 
                    className="admin-preview-video"
                    onTimeUpdate={handleTimeUpdate}
                  />
                  <div className="timestamp-toolbar">
                    <span className="live-time">Frame: {formatTime(currentTime)}</span>
                    <button className="btn-mark start" onClick={handleMarkStart}>
                      🚩 Mark Start Point
                    </button>
                    <button className="btn-mark end" onClick={handleMarkEnd}>
                      🛑 Mark End Point
                    </button>
                  </div>
                </div>

                {/* Clip Form */}
                <div className="clip-form-panel">
                  <h3>✂️ Clip Creation Studio</h3>

                  <div className="form-group">
                    <label>Clip Name / Title *</label>
                    <input 
                      type="text" 
                      placeholder="e.g. Romantic Song, Opening Action Scene" 
                      value={clipTitle}
                      onChange={(e) => setClipTitle(e.target.value)}
                      className="clip-input-text"
                    />
                  </div>

                  <div className="timestamps-box">
                    <div className="time-metric">
                      <span className="label">Start Point:</span>
                      <span className={`val ${clipStart !== null ? 'set' : ''}`}>
                        {clipStart !== null ? formatTime(clipStart) : 'Not marked'}
                      </span>
                    </div>
                    <div className="time-metric">
                      <span className="label">End Point:</span>
                      <span className={`val ${clipEnd !== null ? 'set' : ''}`}>
                        {clipEnd !== null ? formatTime(clipEnd) : 'Not marked'}
                      </span>
                    </div>
                    {clipStart !== null && clipEnd !== null && clipEnd > clipStart && (
                      <div className="time-metric duration">
                        <span className="label">Duration:</span>
                        <span className="val highlight">{formatDuration(clipEnd - clipStart)}</span>
                      </div>
                    )}
                  </div>

                  {/* Multi-Category Selection */}
                  <div className="categories-selection-group">
                    <label><strong>Assign Categories:</strong></label>
                    <div className="category-checkboxes-grid">
                      {categories.map(cat => (
                        <label key={cat.id} className="category-checkbox-label">
                          <input 
                            type="checkbox"
                            checked={selectedCategoryIds.includes(cat.id)}
                            onChange={() => handleToggleCategory(cat.id)}
                          />
                          <span className="cat-name">{cat.name}</span>
                        </label>
                      ))}
                    </div>

                    <div className="add-category-inline">
                      <input 
                        type="text"
                        placeholder="Add new category (e.g. Songs)"
                        value={newCategoryName}
                        onChange={(e) => setNewCategoryName(e.target.value)}
                        className="new-cat-input"
                      />
                      <button className="btn-add-cat" onClick={handleAddCategory}>
                        + Add
                      </button>
                    </div>
                  </div>

                  <button 
                    className={`btn-save-clip ${savingClip ? 'saving' : ''}`}
                    onClick={handleSaveClip}
                    disabled={savingClip}
                  >
                    💾 {savingClip ? 'Creating Clip & Generating Thumbnail...' : 'Save Clip'}
                  </button>
                </div>
              </div>
            ) : (
              <div className="empty-section-card">No video selected.</div>
            )}
          </div>
        </div>
      )}

      {/* TAB 3: CLIPS & CATEGORY LIBRARY */}
      {activeTab === 'clips-library' && (
        <div className="tab-content clips-library-tab">
          <div className="admin-panel-card">
            <div className="library-top-bar">
              <h3>📁 Saved Video Clips ({filteredClips.length})</h3>

              {/* Category Filter Pills */}
              <div className="category-pills">
                <button 
                  className={`pill ${selectedCategoryFilter === 'all' ? 'active' : ''}`}
                  onClick={() => setSelectedCategoryFilter('all')}
                >
                  All Clips ({clips.length})
                </button>
                {categories.map(cat => {
                  const catClipsCount = clips.filter(c => c.category_ids?.includes(cat.id)).length;
                  return (
                    <button 
                      key={cat.id}
                      className={`pill ${selectedCategoryFilter === cat.id ? 'active' : ''}`}
                      onClick={() => setSelectedCategoryFilter(cat.id)}
                    >
                      {cat.name} ({catClipsCount})
                    </button>
                  );
                })}
              </div>
            </div>

            {filteredClips.length === 0 ? (
              <div className="empty-section-card">
                <span className="icon">✂️</span>
                <p>No clips found under this category. Switch to Clip Studio tab to create clips!</p>
              </div>
            ) : (
              <div className="clips-grid">
                {filteredClips.map(clip => {
                  const parentMedia = parentMediaForClip(clip.media_id);
                  return (
                    <div key={clip.id} className="clip-card">
                      <div className="clip-poster-wrap">
                        {clip.thumbnail_path ? (
                          <img src={mediaService.getClipThumbnailUrl(clip.id)} alt={clip.title} />
                        ) : (
                          <div className="fallback-clip-poster">🎬</div>
                        )}
                        <span className="clip-duration-badge">
                          {formatDuration(clip.end_time - clip.start_time)}
                        </span>
                      </div>

                      <div className="clip-details-wrap">
                        <h4>{clip.title}</h4>
                        {parentMedia && <p className="parent-media-name">🎥 {parentMedia.title}</p>}
                        <div className="clip-timestamps-row">
                          <span>⏱️ {formatTime(clip.start_time)} → {formatTime(clip.end_time)}</span>
                        </div>

                        {/* Category Badges */}
                        <div className="clip-categories-badges">
                          {clip.categories && clip.categories.length > 0 ? (
                            clip.categories.map(c => (
                              <span key={c.id} className="cat-badge">{c.name}</span>
                            ))
                          ) : (
                            <span className="cat-badge uncategorized">Uncategorized</span>
                          )}
                        </div>
                      </div>

                      <div className="clip-actions-row">
                        <button className="btn-play-clip" onClick={() => setPlayingClip(clip)}>
                          ▶️ Play Clip
                        </button>
                        <button className="btn-delete-clip" onClick={() => handleDeleteClip(clip.id)}>
                          🗑️ Delete
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}

            {/* Category Manager Footer */}
            <div className="category-manager-section">
              <h4>🏷️ Category Manager</h4>
              <div className="existing-categories-tags">
                {categories.map(cat => (
                  <div key={cat.id} className="cat-tag">
                    <span>{cat.name}</span>
                    <button 
                      className="btn-del-cat"
                      onClick={() => handleDeleteCategory(cat.id, cat.name)}
                      title="Delete category"
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 4: TASKS & DOWNLOADS (EXISTING DASHBOARD) */}
      {activeTab === 'tasks' && (
        <div className="tab-content tasks-tab">
          <div className="admin-dashboard-grid">
            <section className="dashboard-section task-queue-section">
              <h3>⚡ Background Metadata & Thumbnail Tasks ({activeTasks.length})</h3>
              {activeTasks.length === 0 ? (
                <div className="empty-section-card">
                  <span className="icon">✓</span>
                  <p>All background metadata and thumbnail generations are complete. System is idle.</p>
                </div>
              ) : (
                <div className="tasks-list">
                  {activeTasks.map(task => (
                    <div key={task.id} className="task-card">
                      <div className="task-info">
                        <span className="task-badge">PROCESSING</span>
                        <h4 className="task-title">{task.title}</h4>
                        <p className="task-path">{task.file_path}</p>
                      </div>
                      <div className="task-progress-wrap">
                        <div className="pulsing-loader-bar"></div>
                        <span className="status-subtext">Generating local thumbnails & probe details...</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className="dashboard-section downloads-section">
              <h3>📥 Video Downloads ({downloads.length})</h3>
              {downloads.length === 0 ? (
                <div className="empty-section-card">
                  <span className="icon">📥</span>
                  <p>No active, pending, or completed downloads found.</p>
                </div>
              ) : (
                <div className="downloads-list">
                  {downloads.map(dl => (
                    <div key={dl.id} className={`download-card status-${dl.status}`}>
                      <div className="dl-header">
                        <div className="dl-title-group">
                          <span className={`type-badge type-${dl.type}`}>
                            {dl.type === 'torrent' ? '🧲 Torrent' : '🎥 YouTube'}
                          </span>
                          <h4 className="dl-title">{dl.title}</h4>
                        </div>
                        <span className={`status-badge status-${dl.status}`}>
                          {dl.status.toUpperCase()}
                        </span>
                      </div>

                      <div className="dl-body">
                        <p className="dl-path"><strong>Destination:</strong> {dl.dest_path}</p>
                        <div className="dl-metrics-row">
                          <span>{dl.progress.toFixed(1)}%</span>
                          {dl.status === 'downloading' && (
                            <>
                              <span><strong>Speed:</strong> {formatSpeed(dl.download_speed)}</span>
                              {dl.eta && <span><strong>ETA:</strong> {dl.eta}</span>}
                            </>
                          )}
                          <span>{formatSize(dl.completed_size)} / {formatSize(dl.total_size)}</span>
                        </div>
                        <div className="progress-bar-container">
                          <div 
                            className={`progress-bar-fill type-${dl.type}`}
                            style={{ width: `${Math.min(100, Math.max(0, dl.progress))}%` }}
                          ></div>
                        </div>
                      </div>

                      <div className="dl-actions">
                        <button 
                          className="btn-cancel-dl"
                          onClick={() => handleCancelDownload(dl.id)}
                        >
                          🗑️ {dl.status === 'downloading' ? 'Cancel & Delete' : 'Remove Record'}
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
        </div>
      )}

      {/* TAB 5: HOME PAGE ROWS & CATEGORIES ORDERING */}
      {activeTab === 'home-rows' && (
        <div className="tab-content home-rows-tab">
          <div className="admin-panel-card">
            <h3>🏠 Home Page Layout & Category Row Configurator</h3>
            <p className="tab-subtitle">Re-order rows, toggle visibility, and display specific categories as dedicated rows on the Home Page.</p>

            <div className="rows-configurator-grid" style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', marginTop: '1rem' }}>
              <div className="row-order-list" style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                {rowOrders.map((row, idx) => (
                  <div 
                    key={row.id} 
                    className="row-order-item"
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid rgba(255,255,255,0.08)',
                      padding: '0.85rem 1.25rem',
                      borderRadius: '10px'
                    }}
                  >
                    <div className="row-item-left" style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                      <span className="row-order-num" style={{ fontWeight: 800, color: '#60a5fa', minWidth: '24px' }}>#{idx + 1}</span>
                      <strong style={{ fontSize: '1rem', color: '#f8fafc' }}>{row.name}</strong>
                    </div>

                    <div className="row-item-actions" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <button 
                        onClick={() => {
                          const updated = rowOrders.map(r => r.id === row.id ? { ...r, visible: !r.visible } : r);
                          setRowOrders(updated);
                        }}
                        style={{
                          background: row.visible ? 'rgba(34, 197, 94, 0.15)' : 'rgba(239, 68, 68, 0.15)',
                          color: row.visible ? '#4ade80' : '#f87171',
                          border: '1px solid currentColor',
                          padding: '0.3rem 0.75rem',
                          borderRadius: '6px',
                          cursor: 'pointer',
                          fontWeight: 600
                        }}
                      >
                        {row.visible ? '👁️ Visible' : '🙈 Hidden'}
                      </button>
                      <button 
                        onClick={() => {
                          if (idx > 0) {
                            const updated = [...rowOrders];
                            const temp = updated[idx];
                            updated[idx] = updated[idx - 1];
                            updated[idx - 1] = temp;
                            setRowOrders(updated);
                          }
                        }}
                        disabled={idx === 0}
                        style={{ background: 'rgba(255,255,255,0.08)', border: 'none', color: '#fff', padding: '0.3rem 0.6rem', borderRadius: '4px', cursor: 'pointer' }}
                      >
                        ⬆️
                      </button>
                      <button 
                        onClick={() => {
                          if (idx < rowOrders.length - 1) {
                            const updated = [...rowOrders];
                            const temp = updated[idx];
                            updated[idx] = updated[idx + 1];
                            updated[idx + 1] = temp;
                            setRowOrders(updated);
                          }
                        }}
                        disabled={idx === rowOrders.length - 1}
                        style={{ background: 'rgba(255,255,255,0.08)', border: 'none', color: '#fff', padding: '0.3rem 0.6rem', borderRadius: '4px', cursor: 'pointer' }}
                      >
                        ⬇️
                      </button>
                    </div>
                  </div>
                ))}
              </div>

              <button 
                onClick={async () => {
                  try {
                    await mediaService.setPreference('home_row_order', JSON.stringify(rowOrders));
                    showStatus('Successfully saved Home Page layout & row ordering!', 'success');
                  } catch (err: any) {
                    showStatus(`Failed to save layout: ${err.message}`, 'error');
                  }
                }}
                style={{
                  alignSelf: 'flex-start',
                  background: 'linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%)',
                  color: '#fff',
                  border: 'none',
                  padding: '0.75rem 1.75rem',
                  borderRadius: '8px',
                  fontWeight: 700,
                  fontSize: '1rem',
                  cursor: 'pointer'
                }}
              >
                💾 Save Home Page Layout
              </button>
            </div>

            <hr style={{ borderColor: 'rgba(255,255,255,0.1)', margin: '2rem 0' }} />

            {/* VIDEO CATEGORY ASSIGNMENT */}
            <h3>🏷️ Assign Entire Video / Movie to Category</h3>
            <p className="tab-subtitle">Select a video and assign it to categories (e.g. Songs, Highlights, Action) to display the video under that category row.</p>

            <MediaPicker 
              mediaList={mediaList}
              selectedMediaId={selectedMediaId}
              onSelectMedia={(id) => {
                setSelectedMediaId(id);
                const media = mediaList.find(m => m.id === id);
                if (media && media.genre) {
                  const currentLangs = media.genre.split(',').map(s => s.trim());
                  setVideoCategoryNames(currentLangs);
                } else {
                  setVideoCategoryNames([]);
                }
              }}
              title="Select Movie / Video to Categorize"
            />

            {selectedMedia && (
              <div className="video-cat-assignment-box" style={{ display: 'flex', flexDirection: 'column', gap: '1rem', background: 'rgba(255,255,255,0.02)', padding: '1.25rem', borderRadius: '10px' }}>
                <h4>Assign Categories for: <strong>{selectedMedia.title}</strong></h4>
                
                <div className="cat-checkbox-grid" style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem' }}>
                  {categories.map(cat => {
                    const isChecked = videoCategoryNames.includes(cat.name);
                    return (
                      <label 
                        key={cat.id}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '0.5rem',
                          background: isChecked ? 'rgba(59, 130, 246, 0.2)' : 'rgba(255,255,255,0.05)',
                          border: isChecked ? '1px solid #3b82f6' : '1px solid rgba(255,255,255,0.1)',
                          padding: '0.5rem 1rem',
                          borderRadius: '8px',
                          cursor: 'pointer',
                          fontWeight: 600
                        }}
                      >
                        <input 
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => {
                            if (isChecked) {
                              setVideoCategoryNames(prev => prev.filter(n => n !== cat.name));
                            } else {
                              setVideoCategoryNames(prev => [...prev, cat.name]);
                            }
                          }}
                        />
                        {cat.name}
                      </label>
                    );
                  })}
                </div>

                <button 
                  onClick={async () => {
                    try {
                      const updatedGenre = videoCategoryNames.join(', ');
                      const updatedMedia = await mediaService.updateMedia(selectedMedia.id, { genre: updatedGenre });
                      setMediaList(prev => prev.map(m => m.id === updatedMedia.id ? updatedMedia : m));
                      showStatus(`Successfully assigned categories [${updatedGenre}] to "${updatedMedia.title}"!`, 'success');
                    } catch (err: any) {
                      showStatus(`Failed to update video categories: ${err.message}`, 'error');
                    }
                  }}
                  style={{
                    alignSelf: 'flex-start',
                    background: '#22c55e',
                    color: '#fff',
                    border: 'none',
                    padding: '0.65rem 1.5rem',
                    borderRadius: '8px',
                    fontWeight: 700,
                    cursor: 'pointer'
                  }}
                >
                  💾 Save Video Categories
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Modal Video Player for Playing Selected Clip */}
      {playingClip && (
        <VideoPlayer 
          mediaId={playingClip.media_id}
          src={mediaService.getStreamUrl(playingClip.media_id)}
          type="video/mp4"
          poster={playingClip.thumbnail_path ? mediaService.getClipThumbnailUrl(playingClip.id) : undefined}
          startTime={playingClip.start_time}
          endTime={playingClip.end_time}
          onBack={() => setPlayingClip(null)}
        />
      )}
    </div>
  );
};
