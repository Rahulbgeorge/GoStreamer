import React, { useState, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Media, LibraryStats, Category, Clip } from '../types/media';
import { mediaService } from '../services/mediaService';
import { SearchBar } from '../components/SearchBar';
import { HeroBanner } from '../components/HeroBanner';
import { MediaRow } from '../components/MediaRow';
import { MediaCard } from '../components/MediaCard';
import { DetailPage } from './DetailPage';
import { AdminPage } from './AdminPage';
import { CategoryPage } from './CategoryPage';
import { EditModal } from '../components/EditModal';
import { UploadModal } from '../components/UploadModal';
import { VideoPlayer } from '../components/VideoPlayer';
import '../styles/global.css';
import './HomePage.css';

// Build cache breaker to clear CDN cache layers: 2026-07-20 03:55
export const HomePage: React.FC = () => {
  const { t } = useTranslation();
  const [movies, setMovies] = useState<Media[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [clips, setClips] = useState<Clip[]>([]);
  const [stats, setStats] = useState<LibraryStats>({ count: 0, total_size: 0 });
  const [loading, setLoading] = useState(true);
  const [selectedMedia, setSelectedMedia] = useState<Media | null>(null);
  
  const [focusedRow, setFocusedRow] = useState<'header' | 'recent' | 'grid'>('grid');
  const [focusedIndex, setFocusedIndex] = useState(0);
  
  const [isEditing, setIsEditing] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [isAdminPage, setIsAdminPage] = useState(false);
  const [isCategoryPage, setIsCategoryPage] = useState(false);
  const [activeVideo, setActiveVideo] = useState<Media | null>(null);
  const [playingClip, setPlayingClip] = useState<Clip | null>(null);

  // Home Page Category & Language Filter state
  const [activeFilter, setActiveFilter] = useState<string>('all');

  const fetchLibraryData = async () => {
    try {
      const data = await mediaService.getAllMedia();
      setMovies(data);
      const libraryStats = await mediaService.getStats();
      setStats(libraryStats);

      const catsData = await mediaService.getCategories();
      setCategories(catsData);

      const clipsData = await mediaService.getClips();
      setClips(clipsData);
    } catch (err) {
      console.error("Failed to load catalog data", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLibraryData();
    console.log("StreamPlayer Dashboard initialized - v2.0.1");
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
          setFocusedIndex(prev => isTv ? 0 : Math.min(prev + 1, 3));
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
            setIsCategoryPage(true);
          } else if (focusedIndex === 2) {
            e.preventDefault();
            setIsUploading(true);
          } else if (focusedIndex === 3) {
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

  // Extract unique languages dynamically from catalog
  const availableLanguages = useMemo(() => {
    const langs = new Set<string>();
    movies.forEach(m => {
      if (m.language && m.language.trim() !== '') {
        langs.add(m.language.trim().toLowerCase());
      }
    });
    return Array.from(langs);
  }, [movies]);

  // Filter movies based on active category / language filter
  const displayedMovies = useMemo(() => {
    if (activeFilter === 'all' || activeFilter === 'clips') return movies;
    if (activeFilter.startsWith('lang:')) {
      const targetLang = activeFilter.replace('lang:', '').toLowerCase();
      return movies.filter(m => m.language.toLowerCase() === targetLang);
    }
    if (activeFilter.startsWith('cat:')) {
      const catId = activeFilter.replace('cat:', '');
      const catObj = categories.find(c => c.id === catId);
      const catName = catObj ? catObj.name.toLowerCase() : '';
      return movies.filter(m => 
        (m.genre && m.genre.toLowerCase().includes(catName)) ||
        clips.some(clip => clip.media_id === m.id && clip.category_ids?.includes(catId))
      );
    }
    return movies;
  }, [movies, activeFilter, categories, clips]);

  // Filter clips based on active category filter
  const displayedClips = useMemo(() => {
    if (activeFilter === 'clips') return clips;
    if (activeFilter.startsWith('cat:')) {
      const catId = activeFilter.replace('cat:', '');
      return clips.filter(c => c.category_ids?.includes(catId));
    }
    return clips;
  }, [clips, activeFilter]);

  if (isAdminPage) {
    return <AdminPage onBack={() => setIsAdminPage(false)} />;
  }

  if (isCategoryPage) {
    return <CategoryPage onBack={() => setIsCategoryPage(false)} onSelectMedia={(m) => { setSelectedMedia(m); setIsCategoryPage(false); }} />;
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
            className={`btn-action categories-btn ${focusedRow === 'header' && focusedIndex === 1 ? 'focused' : ''}`} 
            onClick={() => setIsCategoryPage(true)}
          >
            📂 Categories
          </button>
          <button 
            className={`btn-action admin ${focusedRow === 'header' && focusedIndex === 2 ? 'focused' : ''}`} 
            onClick={() => setIsUploading(true)}
          >
            📤 {t('admin')}
          </button>
          <button 
            className={`btn-action tasks-dashboard ${focusedRow === 'header' && focusedIndex === 3 ? 'focused' : ''}`} 
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

      {/* Category & Language Navigation Filter Bar */}
      <div className="home-category-bar">
        <div className="filter-group">
          <span className="filter-group-title">🎬 Catalogs:</span>
          <button 
            className={`home-cat-pill ${activeFilter === 'all' ? 'active' : ''}`}
            onClick={() => setActiveFilter('all')}
          >
            All Movies ({movies.length})
          </button>
          {clips.length > 0 && (
            <button 
              className={`home-cat-pill ${activeFilter === 'clips' ? 'active' : ''}`}
              onClick={() => setActiveFilter('clips')}
            >
              ✂️ Clips ({clips.length})
            </button>
          )}
        </div>

        {/* Language Categories */}
        {availableLanguages.length > 0 && (
          <div className="filter-group">
            <span className="filter-group-title">🌐 Languages:</span>
            {availableLanguages.map(lang => {
              const langKey = `lang:${lang}`;
              const count = movies.filter(m => m.language.toLowerCase() === lang).length;
              return (
                <button 
                  key={lang}
                  className={`home-cat-pill ${activeFilter === langKey ? 'active' : ''}`}
                  onClick={() => setActiveFilter(langKey)}
                >
                  {lang.toUpperCase()} ({count})
                </button>
              );
            })}
          </div>
        )}

        {/* Database Categories (Songs, Highlights, Action, etc.) */}
        {categories.length > 0 && (
          <div className="filter-group">
            <span className="filter-group-title">🏷️ Categories:</span>
            {categories.map(cat => {
              const catKey = `cat:${cat.id}`;
              return (
                <button 
                  key={cat.id}
                  className={`home-cat-pill ${activeFilter === catKey ? 'active' : ''}`}
                  onClick={() => setActiveFilter(catKey)}
                >
                  {cat.name}
                </button>
              );
            })}
          </div>
        )}
      </div>

      {loading ? (
        <div className="loading-spinner">Loading catalogs...</div>
      ) : (
        <>
          {/* Main Hero Movie spotlight Banner */}
          {heroMovie && activeFilter === 'all' && (
            <HeroBanner 
              media={heroMovie} 
              onPlay={(m) => setActiveVideo(m)}
              onInfo={(m) => setSelectedMedia(m)}
            />
          )}

          {/* Clips Showcase Section if clips tab or category is active */}
          {(activeFilter === 'clips' || activeFilter.startsWith('cat:')) && displayedClips.length > 0 && (
            <div className="home-clips-section">
              <h3>✂️ Featured Video Clips ({displayedClips.length})</h3>
              <div className="home-clips-grid">
                {displayedClips.map(clip => {
                  const parentMedia = movies.find(m => m.id === clip.media_id);
                  const formatSecs = (secs: number) => {
                    const m = Math.floor(secs / 60);
                    const s = Math.floor(secs % 60);
                    return `${m}:${s.toString().padStart(2, '0')}`;
                  };
                  return (
                    <div key={clip.id} className="home-clip-card" onClick={() => setPlayingClip(clip)}>
                      <div className="home-clip-poster">
                        {clip.thumbnail_path ? (
                          <img src={mediaService.getClipThumbnailUrl(clip.id)} alt={clip.title} />
                        ) : (
                          <div className="fallback-clip-poster">🎬</div>
                        )}
                        <span className="play-overlay-icon">▶</span>
                        <span className="home-clip-duration">
                          {formatSecs(clip.start_time)} - {formatSecs(clip.end_time)}
                        </span>
                      </div>
                      <div className="home-clip-info">
                        <h4>{clip.title}</h4>
                        {parentMedia && <p className="parent-title">🎥 {parentMedia.title}</p>}
                        <div className="home-clip-categories">
                          {clip.categories?.map(c => (
                            <span key={c.id} className="home-cat-badge">{c.name}</span>
                          ))}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {displayedMovies.length === 0 && (activeFilter !== 'clips' || displayedClips.length === 0) ? (
            <div className="no-movies-fallback">
              <p>No video or clip found under this filter category.</p>
              <button className="btn-reset-filter" onClick={() => setActiveFilter('all')}>Show All Movies</button>
            </div>
          ) : (
            activeFilter !== 'clips' && (
              <div className="movie-catalogs">
                {/* Recently added scroll view */}
                {activeFilter === 'all' && (
                  <MediaRow 
                    title={t('recentlyAdded')} 
                    items={displayedMovies} 
                    onSelect={(m) => setSelectedMedia(m)} 
                    focusedIndex={focusedIndex}
                    isFocusedRow={focusedRow === 'recent'}
                  />
                )}
                
                {/* Main movie catalog grids */}
                <div className="all-movies-grid-section">
                  <h3>
                    {activeFilter === 'all' ? t('allMovies') : `Filtered Catalog (${displayedMovies.length})`}
                  </h3>
                  <div className="all-movies-grid">
                    {displayedMovies.map((movie, idx) => (
                      <MediaCard 
                        key={movie.id} 
                        media={movie} 
                        onSelect={(m) => setSelectedMedia(m)} 
                        focused={focusedRow === 'grid' && focusedIndex === idx}
                      />
                    ))}
                  </div>
                </div>
              </div>
            )
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
          startTime={activeVideo.default_start_time || activeVideo.last_position || 0}
          onBack={() => setActiveVideo(null)}
        />
      )}

      {/* Clip player screen overlay */}
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
