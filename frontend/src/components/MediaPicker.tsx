import React, { useState, useMemo } from 'react';
import { Media } from '../types/media';
import { mediaService } from '../services/mediaService';
import './MediaPicker.css';

interface MediaPickerProps {
  mediaList: Media[];
  selectedMediaId: string;
  onSelectMedia: (id: string) => void;
  title?: string;
}

export const MediaPicker: React.FC<MediaPickerProps> = ({
  mediaList,
  selectedMediaId,
  onSelectMedia,
  title = "Select Video"
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedLanguage, setSelectedLanguage] = useState<string>('all');

  // Extract unique languages from mediaList
  const languages = useMemo(() => {
    const langs = new Set<string>();
    mediaList.forEach(m => {
      if (m.language) langs.add(m.language.toLowerCase());
    });
    return Array.from(langs);
  }, [mediaList]);

  // Filter media based on search query and language
  const filteredMedia = useMemo(() => {
    return mediaList.filter(m => {
      const matchesSearch = searchQuery === '' || 
        m.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        m.genre.toLowerCase().includes(searchQuery.toLowerCase()) ||
        m.original_name.toLowerCase().includes(searchQuery.toLowerCase());
      
      const matchesLang = selectedLanguage === 'all' || 
        m.language.toLowerCase() === selectedLanguage.toLowerCase();

      return matchesSearch && matchesLang;
    });
  }, [mediaList, searchQuery, selectedLanguage]);

  const selectedMedia = useMemo(() => {
    return mediaList.find(m => m.id === selectedMediaId);
  }, [mediaList, selectedMediaId]);

  const formatDuration = (secs: number) => {
    if (!secs) return '';
    const mins = Math.floor(secs / 60);
    return `${mins}m`;
  };

  return (
    <div className="media-picker-container">
      <div className="picker-header-bar">
        <div className="picker-title-area">
          <span className="picker-icon">🎬</span>
          <h4>{title}</h4>
          {selectedMedia && (
            <span className="active-selected-badge">
              Active: <strong>{selectedMedia.title}</strong>
            </span>
          )}
        </div>

        <div className="picker-controls-row">
          {/* Search Input */}
          <div className="picker-search-wrap">
            <span className="search-icon">🔍</span>
            <input 
              type="text"
              placeholder="Search video by title, genre, language..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="picker-search-input"
            />
            {searchQuery && (
              <button className="clear-search-btn" onClick={() => setSearchQuery('')}>×</button>
            )}
          </div>

          {/* Language filter pills if multiple languages exist */}
          {languages.length > 1 && (
            <div className="picker-lang-pills">
              <button 
                className={`lang-pill ${selectedLanguage === 'all' ? 'active' : ''}`}
                onClick={() => setSelectedLanguage('all')}
              >
                All ({mediaList.length})
              </button>
              {languages.map(lang => (
                <button 
                  key={lang}
                  className={`lang-pill ${selectedLanguage === lang ? 'active' : ''}`}
                  onClick={() => setSelectedLanguage(lang)}
                >
                  {lang.toUpperCase()}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Media Items Scrollable Grid / Carousel */}
      {filteredMedia.length === 0 ? (
        <div className="picker-empty-fallback">
          <span>🔍</span> No video matching "{searchQuery}" found in library.
        </div>
      ) : (
        <div className="picker-cards-grid">
          {filteredMedia.map(item => {
            const isSelected = item.id === selectedMediaId;
            return (
              <div 
                key={item.id}
                className={`picker-media-card ${isSelected ? 'selected' : ''}`}
                onClick={() => onSelectMedia(item.id)}
              >
                <div className="picker-poster-wrap">
                  {item.thumbnail_path ? (
                    <img src={mediaService.getThumbnailUrl(item.id, item.updated_at)} alt={item.title} />
                  ) : (
                    <div className="picker-poster-fallback">
                      <span>{item.title.slice(0, 2).toUpperCase()}</span>
                    </div>
                  )}

                  {item.quality && <span className="picker-quality-tag">{item.quality.toUpperCase()}</span>}
                  {item.language && <span className="picker-lang-tag">{item.language.toUpperCase()}</span>}
                  {isSelected && <div className="selected-checkmark-overlay">✓</div>}
                </div>

                <div className="picker-info-wrap">
                  <h5 className="picker-item-title" title={item.title}>{item.title}</h5>
                  <div className="picker-meta-line">
                    {item.year > 0 && <span>{item.year}</span>}
                    {item.duration > 0 && <span>{formatDuration(item.duration)}</span>}
                    {item.default_start_time ? <span className="start-badge">⏱️ {Math.floor(item.default_start_time / 60)}m</span> : null}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
