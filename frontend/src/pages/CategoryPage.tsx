import React, { useState, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Media, Category, Clip } from '../types/media';
import { mediaService } from '../services/mediaService';
import { VideoPlayer } from '../components/VideoPlayer';
import './CategoryPage.css';

interface CategoryPageProps {
  onBack: () => void;
  onSelectMedia: (media: Media) => void;
}

export const CategoryPage: React.FC<CategoryPageProps> = ({ onBack, onSelectMedia }) => {
  const { t } = useTranslation();
  const [movies, setMovies] = useState<Media[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [clips, setClips] = useState<Clip[]>([]);
  const [loading, setLoading] = useState(true);

  // Active category / language selection
  const [selectedFilter, setSelectedFilter] = useState<{
    type: 'all' | 'language' | 'category' | 'clips';
    id?: string;
    name?: string;
  }>({ type: 'all' });

  // Player state for clip playback
  const [activeClip, setActiveClip] = useState<Clip | null>(null);
  const [activeClipIndex, setActiveClipIndex] = useState<number>(0);

  // D-pad spatial focus index
  const [focusedIndex, setFocusedIndex] = useState(0);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const allMedia = await mediaService.getAllMedia();
        setMovies(allMedia);
        const cats = await mediaService.getCategories();
        setCategories(cats);
        const allClips = await mediaService.getClips();
        setClips(allClips);
      } catch (err) {
        console.error("Failed to load category page data", err);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  // Extract unique languages dynamically from catalog
  const languages = useMemo(() => {
    const map = new Map<string, number>();
    movies.forEach(m => {
      if (m.language && m.language.trim() !== '') {
        const lang = m.language.trim().toLowerCase();
        map.set(lang, (map.get(lang) || 0) + 1);
      }
    });
    return Array.from(map.entries()).map(([lang, count]) => ({
      lang,
      count
    }));
  }, [movies]);

  // Keyboard D-pad TV Navigation handler
  useEffect(() => {
    if (activeClip) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' || e.key === 'Backspace' || e.key === 'BrowserBack') {
        if (selectedFilter.type !== 'all') {
          setSelectedFilter({ type: 'all' });
        } else {
          onBack();
        }
        return;
      }

      if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Enter'].includes(e.key)) {
        document.body.classList.add('tv-mode');
      }

      const gridItems = document.querySelectorAll('.tv-focusable');
      if (gridItems.length === 0) return;

      if (e.key === 'ArrowRight') {
        e.preventDefault();
        setFocusedIndex(prev => Math.min(prev + 1, gridItems.length - 1));
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault();
        setFocusedIndex(prev => Math.max(prev - 1, 0));
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        setFocusedIndex(prev => Math.min(prev + 3, gridItems.length - 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setFocusedIndex(prev => Math.max(prev - 3, 0));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        const currentItem = gridItems[focusedIndex] as HTMLElement;
        if (currentItem) currentItem.click();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedFilter, activeClip, focusedIndex, onBack]);

  // Scroll focused element into view
  useEffect(() => {
    const timer = setTimeout(() => {
      const focusedEl = document.querySelector('.tv-focusable.focused');
      if (focusedEl) {
        focusedEl.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' });
      }
    }, 50);
    return () => clearTimeout(timer);
  }, [focusedIndex]);

  // Filter movies matching selected filter
  const filteredMovies = useMemo(() => {
    if (selectedFilter.type === 'language' && selectedFilter.name) {
      return movies.filter(m => m.language.toLowerCase() === selectedFilter.name!.toLowerCase());
    }
    if (selectedFilter.type === 'category' && selectedFilter.id) {
      const catObj = categories.find(c => c.id === selectedFilter.id);
      const catName = catObj ? catObj.name.toLowerCase() : '';
      return movies.filter(m => 
        (m.genre && m.genre.toLowerCase().includes(catName)) ||
        clips.some(clip => clip.media_id === m.id && clip.category_ids?.includes(selectedFilter.id!))
      );
    }
    return movies;
  }, [movies, selectedFilter, categories, clips]);

  // Filter clips matching selected filter
  const filteredClips = useMemo(() => {
    if (selectedFilter.type === 'clips') return clips;
    if (selectedFilter.type === 'category' && selectedFilter.id) {
      return clips.filter(c => c.category_ids?.includes(selectedFilter.id!));
    }
    return [];
  }, [clips, selectedFilter]);

  const parentMediaForClip = (mediaId: string) => movies.find(m => m.id === mediaId);

  const formatDuration = (secs: number) => {
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

  return (
    <div className="category-page-wrapper">
      {/* Top TV-Friendly Header */}
      <header className="category-header">
        <div className="cat-header-left">
          <button 
            className="btn-back-nav tv-focusable" 
            onClick={() => {
              if (selectedFilter.type !== 'all') {
                setSelectedFilter({ type: 'all' });
                setFocusedIndex(0);
              } else {
                onBack();
              }
            }}
          >
            ← {selectedFilter.type !== 'all' ? 'Back to Categories' : t('back')}
          </button>
          <h2>
            {selectedFilter.type === 'all' && '📂 Category & Language Directory'}
            {selectedFilter.type === 'language' && `🌐 ${selectedFilter.name?.toUpperCase()} Movies`}
            {selectedFilter.type === 'category' && `🏷️ ${selectedFilter.name}`}
            {selectedFilter.type === 'clips' && '✂️ All Video Clips'}
          </h2>
        </div>

        {selectedFilter.type !== 'all' && (
          <div className="cat-header-right">
            <span className="results-count-badge">
              {filteredMovies.length} Videos {filteredClips.length > 0 ? `• ${filteredClips.length} Clips` : ''}
            </span>
          </div>
        )}
      </header>

      {loading ? (
        <div className="loading-spinner">Loading categories...</div>
      ) : selectedFilter.type === 'all' ? (
        /* MAIN CATEGORY & LANGUAGE SELECTION OVERVIEW */
        <div className="category-directory-grid">
          {/* SECTION 1: LANGUAGES */}
          <section className="directory-section">
            <div className="section-title-row">
              <h3>🌐 Browse Movies by Language</h3>
            </div>
            <div className="tiles-grid">
              {languages.map((item, idx) => (
                <div 
                  key={item.lang}
                  className={`category-tile language-tile tv-focusable ${focusedIndex === idx ? 'focused' : ''}`}
                  onClick={() => {
                    setSelectedFilter({ type: 'language', name: item.lang });
                    setFocusedIndex(0);
                  }}
                >
                  <div className="tile-icon">🗣️</div>
                  <div className="tile-info">
                    <h4>{item.lang.toUpperCase()}</h4>
                    <span>{item.count} {item.count === 1 ? 'Movie' : 'Movies'}</span>
                  </div>
                  <span className="tile-arrow">→</span>
                </div>
              ))}
            </div>
          </section>

          {/* SECTION 2: CLIP & MEDIA CATEGORIES */}
          <section className="directory-section">
            <div className="section-title-row">
              <h3>🏷️ Browse by Genre & Clip Category</h3>
            </div>
            <div className="tiles-grid">
              {/* Featured Video Clips Tile */}
              {clips.length > 0 && (
                <div 
                  className={`category-tile featured-clips-tile tv-focusable ${focusedIndex === languages.length ? 'focused' : ''}`}
                  onClick={() => {
                    setSelectedFilter({ type: 'clips', name: 'All Clips' });
                    setFocusedIndex(0);
                  }}
                >
                  <div className="tile-icon">✂️</div>
                  <div className="tile-info">
                    <h4>Featured Video Clips</h4>
                    <span>{clips.length} Short Clips</span>
                  </div>
                  <span className="tile-arrow">→</span>
                </div>
              )}

              {categories.map((cat, idx) => {
                const totalIndex = languages.length + (clips.length > 0 ? 1 : 0) + idx;
                const catClips = clips.filter(c => c.category_ids?.includes(cat.id)).length;
                return (
                  <div 
                    key={cat.id}
                    className={`category-tile cat-item-tile tv-focusable ${focusedIndex === totalIndex ? 'focused' : ''}`}
                    onClick={() => {
                      setSelectedFilter({ type: 'category', id: cat.id, name: cat.name });
                      setFocusedIndex(0);
                    }}
                  >
                    <div className="tile-icon">
                      {cat.name.toLowerCase().includes('song') ? '🎵' :
                       cat.name.toLowerCase().includes('highlight') ? '⚡' :
                       cat.name.toLowerCase().includes('action') ? '🔥' :
                       cat.name.toLowerCase().includes('dialogue') ? '💬' : '📁'}
                    </div>
                    <div className="tile-info">
                      <h4>{cat.name}</h4>
                      <span>{catClips} Clips & Movies</span>
                    </div>
                    <span className="tile-arrow">→</span>
                  </div>
                );
              })}
            </div>
          </section>
        </div>
      ) : (
        /* FILTERED CONTENT VIEW */
        <div className="filtered-category-content">
          {/* CLIPS SECTION */}
          {filteredClips.length > 0 && (
            <section className="category-content-block">
              <div className="section-title-row" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h3 className="block-title">✂️ Video Clips ({filteredClips.length})</h3>
                <button 
                  className="btn-autoplay-all tv-focusable"
                  onClick={() => {
                    setActiveClipIndex(0);
                    setActiveClip(filteredClips[0]);
                  }}
                  style={{
                    background: 'linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%)',
                    color: '#fff',
                    border: 'none',
                    padding: '0.5rem 1.2rem',
                    borderRadius: '20px',
                    fontWeight: 700,
                    cursor: 'pointer',
                    fontSize: '0.9rem'
                  }}
                >
                  ▶ Auto-Play All Clips
                </button>
              </div>

              <div className="clips-cards-grid">
                {filteredClips.map((clip, idx) => {
                  const parentMedia = parentMediaForClip(clip.media_id);
                  const isFocused = focusedIndex === idx;
                  return (
                    <div 
                      key={clip.id} 
                      className={`cat-clip-card tv-focusable ${isFocused ? 'focused' : ''}`}
                      onClick={() => {
                        setActiveClipIndex(idx);
                        setActiveClip(clip);
                      }}
                    >
                      <div className="clip-thumb-wrap">
                        {clip.thumbnail_path ? (
                          <img src={mediaService.getClipThumbnailUrl(clip.id)} alt={clip.title} />
                        ) : (
                          <div className="fallback-clip-poster">🎬</div>
                        )}
                        <span className="play-icon-overlay">▶</span>
                        <span className="clip-time-tag">
                          {formatDuration(clip.start_time)} - {formatDuration(clip.end_time)}
                        </span>
                      </div>
                      <div className="clip-card-info">
                        <h4>{clip.title}</h4>
                        {parentMedia && <p className="parent-movie-title">🎥 {parentMedia.title}</p>}
                      </div>
                    </div>
                  );
                })}
              </div>
            </section>
          )}

          {/* MOVIES CATALOG GRID */}
          <section className="category-content-block">
            <h3 className="block-title">🎬 Movie Library ({filteredMovies.length})</h3>
            {filteredMovies.length === 0 ? (
              <div className="empty-section-card">
                <p>No full movies found under this category filter.</p>
              </div>
            ) : (
              <div className="movies-cards-grid">
                {filteredMovies.map((movie, idx) => {
                  const offsetIdx = filteredClips.length + idx;
                  const isFocused = focusedIndex === offsetIdx;
                  return (
                    <div 
                      key={movie.id} 
                      className={`cat-movie-card tv-focusable ${isFocused ? 'focused' : ''}`}
                      onClick={() => onSelectMedia(movie)}
                    >
                      <div className="movie-poster-wrap">
                        {movie.thumbnail_path ? (
                          <img src={mediaService.getThumbnailUrl(movie.id, movie.updated_at)} alt={movie.title} />
                        ) : (
                          <div className="fallback-poster-box">
                            <span>{movie.title.slice(0, 2).toUpperCase()}</span>
                          </div>
                        )}
                        {movie.quality && <span className="quality-badge">{movie.quality.toUpperCase()}</span>}
                        {movie.language && <span className="language-badge">{movie.language.toUpperCase()}</span>}
                      </div>
                      <div className="movie-card-info">
                        <h4>{movie.title}</h4>
                        <div className="movie-card-meta">
                          {movie.year > 0 && <span>{movie.year}</span>}
                          {movie.genre && <span className="genre-tag">{movie.genre}</span>}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        </div>
      )}

      {/* Video Player Modal for Clips */}
      {activeClip && (
        <VideoPlayer 
          mediaId={activeClip.media_id}
          src={mediaService.getStreamUrl(activeClip.media_id)}
          type="video/mp4"
          poster={activeClip.thumbnail_path ? mediaService.getClipThumbnailUrl(activeClip.id) : undefined}
          startTime={activeClip.start_time}
          endTime={activeClip.end_time}
          clipPlaylist={filteredClips}
          initialClipIndex={activeClipIndex}
          allMediaList={movies}
          categoryName={selectedFilter.name || 'Category'}
          onBack={() => setActiveClip(null)}
        />
      )}
    </div>
  );
};
