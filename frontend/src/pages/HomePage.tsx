import React, { useState, useEffect, useMemo, useRef } from 'react';
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
  
  const [focusedRow, setFocusedRow] = useState<'sidenav' | 'header' | 'recent' | 'grid'>('grid');
  const [focusedIndex, setFocusedIndex] = useState(0);
  const lastFocusedMainRowRef = useRef<'recent' | 'grid'>('grid');
  
  const [isEditing, setIsEditing] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [isAdminPage, setIsAdminPage] = useState(false);
  const [isCategoryPage, setIsCategoryPage] = useState(false);
  const [activeVideo, setActiveVideo] = useState<Media | null>(null);
  const [playingClip, setPlayingClip] = useState<Clip | null>(null);

  // Home Page Category & Language Filter state
  const [activeFilter, setActiveFilter] = useState<string>('all');

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

  const handleSaveClipToLibrary = async (clipId: string) => {
    try {
      const res = await mediaService.saveClipToLibrary(clipId);
      window.alert(res.message || "Clip saved successfully to local library!");
      fetchLibraryData();
      if (res.media) {
        if (window.confirm("Would you like to play and loop the saved clip now?")) {
          setActiveVideo(res.media);
        }
      }
    } catch (err: any) {
      window.alert("Failed to save clip to library: " + err.message);
    }
  };

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

      const sideNavItems = getSideNavItems();

      if (e.key === 'ArrowRight') {
        if (focusedRow === 'sidenav') {
          // Leave sidenav, enter main content
          setFocusedRow(lastFocusedMainRowRef.current);
          setFocusedIndex(0);
        } else if (focusedRow === 'recent') {
          setFocusedIndex(prev => Math.min(prev + 1, displayedMovies.length - 1));
        } else if (focusedRow === 'grid') {
          setFocusedIndex(prev => Math.min(prev + 1, displayedMovies.length - 1));
        }
      } else if (e.key === 'ArrowLeft') {
        if (focusedRow === 'sidenav') {
          // Do nothing
        } else if (focusedRow === 'recent' || focusedRow === 'grid') {
          if (focusedIndex === 0) {
            // Go left to open sidenav
            lastFocusedMainRowRef.current = focusedRow;
            setFocusedRow('sidenav');
            const activeFilterIdx = sideNavItems.indexOf(activeFilter);
            setFocusedIndex(activeFilterIdx !== -1 ? activeFilterIdx : 1);
          } else {
            setFocusedIndex(prev => prev - 1);
          }
        }
      } else if (e.key === 'ArrowDown') {
        if (focusedRow === 'sidenav') {
          setFocusedIndex(prev => Math.min(sideNavItems.length - 1, prev + 1));
        } else if (focusedRow === 'recent') {
          setFocusedRow('grid');
          setFocusedIndex(0);
        }
      } else if (e.key === 'ArrowUp') {
        if (focusedRow === 'sidenav') {
          setFocusedIndex(prev => Math.max(0, prev - 1));
        } else if (focusedRow === 'grid') {
          if (activeFilter === 'all' && displayedMovies.length > 0) {
            setFocusedRow('recent');
            setFocusedIndex(0);
          }
        }
      } else if (e.key === 'Enter') {
        if (focusedRow === 'sidenav') {
          const selectedItem = sideNavItems[focusedIndex];
          if (selectedItem === 'search') {
            e.preventDefault();
            (document.querySelector('.search-bar input') as HTMLInputElement)?.focus();
          } else if (selectedItem === 'all') {
            setActiveFilter('all');
          } else if (selectedItem === 'clips') {
            setActiveFilter('clips');
          } else if (selectedItem.startsWith('lang:')) {
            setActiveFilter(selectedItem);
          } else if (selectedItem.startsWith('cat:')) {
            setActiveFilter(selectedItem);
          } else if (selectedItem === 'categories-page') {
            setIsCategoryPage(true);
          } else if (selectedItem === 'upload') {
            setIsUploading(true);
          } else if (selectedItem === 'tasks') {
            setIsAdminPage(true);
          }
        } else if (focusedRow === 'recent') {
          e.preventDefault();
          setSelectedMedia(displayedMovies[focusedIndex]);
        } else if (focusedRow === 'grid') {
          e.preventDefault();
          setSelectedMedia(displayedMovies[focusedIndex]);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [focusedRow, focusedIndex, movies, selectedMedia, activeVideo, isUploading, isEditing, isAdminPage, activeFilter, categories, clips, displayedMovies, availableLanguages]);

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

  if (isCategoryPage) {
    return <CategoryPage onBack={() => setIsCategoryPage(false)} onSelectMedia={(m) => { setSelectedMedia(m); setIsCategoryPage(false); }} />;
  }

  const getSideNavItems = () => {
    const items = ['search', 'all'];
    if (clips.length > 0) {
      items.push('clips');
    }
    availableLanguages.forEach(lang => {
      items.push(`lang:${lang}`);
    });
    categories.forEach(cat => {
      items.push(`cat:${cat.id}`);
    });
    items.push('categories-page', 'upload', 'tasks');
    return items;
  };

  const getSideNavItemInfo = (item: string) => {
    if (item === 'search') return { icon: '🔍', label: t('searchPlaceholder', { defaultValue: 'Search...' }) };
    if (item === 'all') return { icon: '🏠', label: 'All Movies' };
    if (item === 'clips') return { icon: '✂️', label: 'Featured Clips' };
    if (item.startsWith('lang:')) {
      const lang = item.replace('lang:', '').toUpperCase();
      return { icon: '🌐', label: `${lang} Movies` };
    }
    if (item.startsWith('cat:')) {
      const catId = item.replace('cat:', '');
      const cat = categories.find(c => c.id === catId);
      return { icon: '🏷️', label: cat ? cat.name : 'Category' };
    }
    if (item === 'categories-page') return { icon: '📂', label: 'Categories' };
    if (item === 'upload') return { icon: '📤', label: 'Admin Upload' };
    if (item === 'tasks') return { icon: '⚡', label: 'Tasks & Downloads' };
    return { icon: '•', label: item };
  };

  const heroMovie = movies.length > 0 ? movies[0] : null;

  return (
    <div className="homepage-wrapper">
      {/* Left Navigation Bar (Side Drawer) */}
      <aside className={`home-sidenav ${focusedRow === 'sidenav' ? 'expanded' : ''}`}>
        <div className="sidenav-brand">
          <span className="brand-logo">📺</span>
          <span className="brand-name">{t('brand')}</span>
        </div>

        <div className="sidenav-menu">
          {getSideNavItems().map((item, idx) => {
            const info = getSideNavItemInfo(item);
            const isItemFocused = focusedRow === 'sidenav' && focusedIndex === idx;
            const isItemActive = activeFilter === item || 
              (item === 'search' && document.activeElement?.tagName === 'INPUT');

            return (
              <div 
                key={item} 
                className={`sidenav-item ${item}-item ${isItemFocused ? 'focused' : ''} ${isItemActive ? 'active' : ''}`}
                onClick={() => {
                  setFocusedRow('sidenav');
                  setFocusedIndex(idx);
                  if (item === 'search') {
                    (document.querySelector('.search-bar input') as HTMLInputElement)?.focus();
                  } else if (item === 'all') {
                    setActiveFilter('all');
                  } else if (item === 'clips') {
                    setActiveFilter('clips');
                  } else if (item.startsWith('lang:')) {
                    setActiveFilter(item);
                  } else if (item.startsWith('cat:')) {
                    setActiveFilter(item);
                  } else if (item === 'categories-page') {
                    setIsCategoryPage(true);
                  } else if (item === 'upload') {
                    setIsUploading(true);
                  } else if (item === 'tasks') {
                    setIsAdminPage(true);
                  }
                }}
              >
                {item === 'search' ? (
                  <SearchBar 
                    onSearch={handleSearch} 
                    isFocused={isItemFocused} 
                  />
                ) : (
                  <>
                    <span className="sidenav-item-icon">{info.icon}</span>
                    <span className="sidenav-item-label">{info.label}</span>
                  </>
                )}
              </div>
            );
          })}
        </div>

        <div className="sidenav-footer">
          <div className="stats-indicator">
            <span className="footer-icon">📊</span>
            <span className="footer-label">
              {stats.count} Videos ({(stats.total_size / (1024 * 1024 * 1024)).toFixed(2)} GB)
            </span>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="home-main-content">
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
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                          <h4 style={{ margin: 0 }}>{clip.title}</h4>
                          <div style={{ display: 'flex', gap: '8px' }}>
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                handleSaveClipToLibrary(clip.id);
                              }}
                              title="Save Clip to App Library on Device"
                              style={{ border: 'none', cursor: 'pointer', fontSize: '1.1rem', background: 'rgba(255,255,255,0.1)', padding: '4px 8px', borderRadius: '6px' }}
                            >
                              💾
                            </button>
                            <a 
                              href={mediaService.getClipDownloadUrl(clip.id)} 
                              download 
                              onClick={(e) => e.stopPropagation()} 
                              title="Download Clip File (.mp4) to PC"
                              style={{ fontSize: '1.1rem', textDecoration: 'none', background: 'rgba(255,255,255,0.1)', padding: '4px 8px', borderRadius: '6px' }}
                            >
                              ⬇️
                            </a>
                          </div>
                        </div>
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
      </main>

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
