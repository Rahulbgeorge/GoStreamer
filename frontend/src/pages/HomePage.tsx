import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Media, LibraryStats } from '../types/media';
import { mediaService } from '../services/mediaService';
import { SearchBar } from '../components/SearchBar';
import { HeroBanner } from '../components/HeroBanner';
import { MediaRow } from '../components/MediaRow';
import { DetailPage } from './DetailPage';
import { AdminPage } from './AdminPage';
import { EditModal } from '../components/EditModal';
import { UploadModal } from '../components/UploadModal';
import { VideoPlayer } from '../components/VideoPlayer';
import '../styles/global.css';
import './HomePage.css';

export const HomePage: React.FC = () => {
  const { t } = useTranslation();
  const [movies, setMovies] = useState<Media[]>([]);
  const [stats, setStats] = useState<LibraryStats>({ count: 0, total_size: 0 });
  const [loading, setLoading] = useState(true);
  const [selectedMedia, setSelectedMedia] = useState<Media | null>(null);
  
  const [focusedRow, setFocusedRow] = useState<'header' | 'recent' | 'grid'>('grid');
  const [focusedIndex, setFocusedIndex] = useState(0);
  
  const [isEditing, setIsEditing] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [isAdminPage, setIsAdminPage] = useState(false);
  const [activeVideo, setActiveVideo] = useState<Media | null>(null);

  const fetchLibraryData = async () => {
    try {
      const data = await mediaService.getAllMedia();
      setMovies(data);
      const libraryStats = await mediaService.getStats();
      setStats(libraryStats);
    } catch (err) {
      console.error("Failed to load catalog data", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLibraryData();
  }, []);

  useEffect(() => {
    const handleMouseMove = () => {
      document.body.classList.remove('tv-mode');
    };
    window.addEventListener('mousemove', handleMouseMove);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
    };
  }, []);

  useEffect(() => {
    // Smoothly scroll the currently focused element (grid item, list item, button, etc.) into view
    const timer = setTimeout(() => {
      const focusedEl = document.querySelector('.focused');
      if (focusedEl) {
        focusedEl.scrollIntoView({
          behavior: 'smooth',
          block: 'nearest',
          inline: 'nearest'
        });
      }
    }, 50);
    return () => clearTimeout(timer);
  }, [focusedRow, focusedIndex]);

  useEffect(() => {
    // Disable main page navigation if any modal or video player is active
    if (selectedMedia || activeVideo || isUploading || isEditing) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      // If user is currently typing in a text field, let those inputs handle D-pad natively
      if (document.activeElement?.tagName === 'INPUT' || document.activeElement?.tagName === 'TEXTAREA') {
        if (e.key === 'Enter' || e.key === 'Escape') {
          (document.activeElement as HTMLElement).blur();
        }
        return;
      }

      if (movies.length === 0) return;

      // Add tv-mode class when navigating with keyboard D-pad
      if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Enter'].includes(e.key)) {
        document.body.classList.add('tv-mode');
      }

      // Prevent default scrolling and browser spatial focus moves on D-pad navigation
      if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(e.key)) {
        e.preventDefault();
      }

      const isTv = document.body.classList.contains('tv-mode');

      if (e.key === 'ArrowRight') {
        if (focusedRow === 'header') {
          setFocusedIndex(prev => isTv ? 0 : Math.min(prev + 1, 2));
        } else if (focusedRow === 'recent' || focusedRow === 'grid') {
          setFocusedIndex(prev => Math.min(prev + 1, movies.length - 1));
        }
      } else if (e.key === 'ArrowLeft') {
        setFocusedIndex(prev => Math.max(prev - 1, 0));
      } else if (e.key === 'ArrowDown') {
        if (focusedRow === 'header') {
          setFocusedRow('recent');
          setFocusedIndex(0);
        } else if (focusedRow === 'recent') {
          setFocusedRow('grid');
          setFocusedIndex(0);
        }
      } else if (e.key === 'ArrowUp') {
        if (focusedRow === 'grid') {
          setFocusedRow('recent');
          setFocusedIndex(0);
        } else if (focusedRow === 'recent') {
          setFocusedRow('header');
          setFocusedIndex(isTv ? 0 : 1);
        }
      } else if (e.key === 'Enter') {
        if (focusedRow === 'header') {
          if (focusedIndex === 0) {
            e.preventDefault();
            (document.querySelector('.search-bar input') as HTMLInputElement)?.focus();
          } else if (focusedIndex === 1) {
            e.preventDefault();
            setIsUploading(true);
          } else if (focusedIndex === 2) {
            e.preventDefault();
            setIsAdminPage(true);
          }
        } else if (focusedRow === 'recent') {
          e.preventDefault();
          setSelectedMedia(movies[focusedIndex]);
        } else if (focusedRow === 'grid') {
          e.preventDefault();
          setSelectedMedia(movies[focusedIndex]);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [focusedRow, focusedIndex, movies, selectedMedia, activeVideo, isUploading, isEditing, isAdminPage]);

  const handleSearch = async (query: string) => {
    if (query.trim() === '') {
      fetchLibraryData();
      return;
    }
    const results = await mediaService.search(query);
    setMovies(results);
  };

  const handleUpdate = async (updates: Partial<Media>) => {
    if (!selectedMedia) return;
    try {
      const updated = await mediaService.updateMedia(selectedMedia.id, updates);
      setSelectedMedia(updated);
      setIsEditing(false);
      fetchLibraryData();
    } catch (err) {
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!selectedMedia) return;
    if (window.confirm("Are you sure you want to delete this media file from disk?")) {
      try {
        await mediaService.deleteMedia(selectedMedia.id);
        setSelectedMedia(null);
        fetchLibraryData();
      } catch (err) {
        console.error(err);
      }
    }
  };

  if (isAdminPage) {
    return <AdminPage onBack={() => setIsAdminPage(false)} />;
  }

  const heroMovie = movies.length > 0 ? movies[0] : null;

  return (
    <div className="homepage-wrapper">
      {/* Header Bar */}
      <header className="home-header">
        <div className="header-brand">
          <span className="brand-logo">📺</span>
          <h1>{t('brand')}</h1>
        </div>
        <div className="header-actions">
          <SearchBar 
            onSearch={handleSearch} 
            isFocused={focusedRow === 'header' && focusedIndex === 0} 
          />
          <button 
            className={`btn-action admin ${focusedRow === 'header' && focusedIndex === 1 ? 'focused' : ''}`} 
            onClick={() => setIsUploading(true)}
          >
            📤 {t('admin')}
          </button>
          <button 
            className={`btn-action tasks-dashboard ${focusedRow === 'header' && focusedIndex === 2 ? 'focused' : ''}`} 
            onClick={() => setIsAdminPage(true)}
          >
            ⚡ Tasks & Downloads
          </button>
        </div>
      </header>

      {/* Library Stats Dashboard Banner */}
      <div className="stats-row">
        <span><strong>{t('statsCount')}:</strong> {stats.count}</span>
        <span><strong>{t('statsSize')}:</strong> {(stats.total_size / (1024 * 1024 * 1024)).toFixed(2)} GB</span>
      </div>

      {loading ? (
        <div className="loading-spinner">Loading catalogs...</div>
      ) : (
        <>
          {/* Main Hero Movie spotlight Banner */}
          {heroMovie && (
            <HeroBanner 
              media={heroMovie} 
              onPlay={(m) => setActiveVideo(m)}
              onInfo={(m) => setSelectedMedia(m)}
            />
          )}

          {movies.length === 0 ? (
            <div className="no-movies-fallback">
              <p>{t('noMovies')}</p>
            </div>
          ) : (
            <div className="movie-catalogs">
              {/* Recently added scroll view */}
              <MediaRow 
                title={t('recentlyAdded')} 
                items={movies} 
                onSelect={(m) => setSelectedMedia(m)} 
                focusedIndex={focusedIndex}
                isFocusedRow={focusedRow === 'recent'}
              />
              
              {/* Main movie catalog grids */}
              <div className="all-movies-grid-section">
                <h3>{t('allMovies')}</h3>
                <div className="all-movies-grid">
                  {movies.map((movie, idx) => {
                    const isFocused = focusedRow === 'grid' && focusedIndex === idx;
                    const formatBytes = (bytes: number) => {
                      if (bytes <= 0) return '0.00 GB';
                      if (bytes >= 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
                      return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
                    };
                    return (
                      <div 
                        key={movie.id} 
                        onClick={() => setSelectedMedia(movie)} 
                        className={`grid-movie-card ${isFocused ? 'focused' : ''}`}
                      >
                        <div className="grid-poster-wrap">
                          {movie.thumbnail_path ? (
                            <img src={mediaService.getThumbnailUrl(movie.id)} alt={movie.title} />
                          ) : (
                            <div className="fallback-grid-poster">
                              <span>{movie.title.slice(0, 2).toUpperCase()}</span>
                            </div>
                          )}
                          {movie.quality && <span className="quality-badge">{movie.quality.toUpperCase()}</span>}
                        </div>
                        <div className="grid-info-wrap">
                          <h4>{movie.title}</h4>
                          <div className="grid-meta-wrap">
                            {movie.year > 0 && <span className="grid-year">{movie.year}</span>}
                            {isFocused && <span className="grid-size">{formatBytes(movie.file_size)}</span>}
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          )}
        </>
      )}

      {/* Full Movie player screen overlay */}
      {activeVideo && (
        <VideoPlayer 
          mediaId={activeVideo.id}
          src={mediaService.getStreamUrl(activeVideo.id)}
          type={activeVideo.mime_type}
          poster={activeVideo.thumbnail_path ? mediaService.getThumbnailUrl(activeVideo.id) : undefined}
          onBack={() => setActiveVideo(null)}
        />
      )}

      {/* Detail info Modal Overlay */}
      {selectedMedia && !isEditing && (
        <DetailPage 
          media={selectedMedia}
          onPlay={(m) => setActiveVideo(m)}
          onClose={() => setSelectedMedia(null)}
          onEdit={() => setIsEditing(true)}
          onDelete={handleDelete}
        />
      )}

      {/* Edit Details Modal Overlay */}
      {selectedMedia && isEditing && (
        <EditModal 
          media={selectedMedia}
          onSave={handleUpdate}
          onClose={() => setIsEditing(false)}
        />
      )}

      {/* Upload files Modal Overlay */}
      {isUploading && (
        <UploadModal 
          onClose={() => setIsUploading(false)}
          onUploadSuccess={() => {
            fetchLibraryData();
          }}
        />
      )}
    </div>
  );
};
