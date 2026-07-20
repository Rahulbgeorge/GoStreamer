import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Media, Download } from '../types/media';
import { mediaService } from '../services/mediaService';
import './AdminPage.css';

interface AdminPageProps {
  onBack: () => void;
}

export const AdminPage: React.FC<AdminPageProps> = ({ onBack }) => {
  const { t } = useTranslation();
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [activeTasks, setActiveTasks] = useState<Media[]>([]);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [scanMessage, setScanMessage] = useState('');

  const fetchData = async () => {
    try {
      // 1. Fetch unified downloads list
      const dlData = await mediaService.getDownloads();
      setDownloads(dlData);

      // 2. Fetch all media and filter for 'processing' status (active cataloging tasks)
      const allMedia = await mediaService.getAllMedia();
      const processingMedia = allMedia.filter(m => m.status === 'processing');
      setActiveTasks(processingMedia);
    } catch (err) {
      console.error('Failed to poll admin dashboard metrics:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    // Poll data every 2 seconds for live progress tracking
    const interval = setInterval(fetchData, 2000);
    return () => clearInterval(interval);
  }, []);

  const handleCancelDownload = async (id: string) => {
    if (window.confirm('Are you sure you want to cancel and delete this download record?')) {
      try {
        await mediaService.deleteDownload(id);
        fetchData();
      } catch (err) {
        alert('Failed to cancel download: ' + err);
      }
    }
  };

  const handleTriggerScan = async () => {
    setScanning(true);
    setScanMessage('Scan initiated in background...');
    try {
      const res = await mediaService.scanDirectory();
      if (res) {
        setScanMessage('Scan completed successfully! Loading changes...');
        setTimeout(() => setScanMessage(''), 3000);
        fetchData();
      } else {
        setScanMessage('Scan completed with issues.');
      }
    } catch (err) {
      setScanMessage('Failed to trigger scan: ' + err);
    } finally {
      setScanning(false);
    }
  };

  const formatSpeed = (bps: number) => {
    if (bps <= 0) return '0 KB/s';
    const kbps = bps / 1000;
    if (kbps >= 1000) {
      return `${(kbps / 1000).toFixed(1)} MB/s`;
    }
    return `${kbps.toFixed(0)} KB/s`;
  };

  const formatSize = (bytes: number) => {
    if (bytes <= 0) return '0 MB';
    const mb = bytes / (1024 * 1024);
    if (mb >= 1024) {
      return `${(mb / 1024).toFixed(2)} GB`;
    }
    return `${mb.toFixed(0)} MB`;
  };

  return (
    <div className="admin-page-wrapper">
      {/* Top Header */}
      <header className="admin-header">
        <div className="header-left">
          <button className="btn-back-nav" onClick={onBack}>
            ← {t('back')}
          </button>
          <h2>System Operations & Tasks</h2>
        </div>
        <div className="header-right">
          {scanMessage && <span className="scan-status-msg">{scanMessage}</span>}
          <button 
            className={`btn-scan-trigger ${scanning ? 'loading' : ''}`} 
            onClick={handleTriggerScan}
            disabled={scanning}
          >
            🔄 {scanning ? 'Scanning...' : 'Trigger Rescan'}
          </button>
        </div>
      </header>

      <div className="admin-dashboard-grid">
        {/* Active System Tasks */}
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

        {/* Video Downloads */}
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

                    {/* Progress details */}
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

                    {/* Progress Bar */}
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
  );
};
