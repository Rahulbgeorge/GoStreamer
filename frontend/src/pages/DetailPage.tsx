import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Media } from '../types/media';
import { mediaService } from '../services/mediaService';
import './DetailPage.css';

interface DetailPageProps {
  media: Media;
  onPlay: (media: Media) => void;
  onClose: () => void;
  onEdit: () => void;
  onDelete: () => void;
}

export const DetailPage: React.FC<DetailPageProps> = ({ media, onPlay, onClose, onEdit, onDelete }) => {
  const { t } = useTranslation();
  const [focusedBtn, setFocusedBtn] = useState<'play' | 'edit' | 'delete'>('play');
  const posterUrl = media.thumbnail_path ? mediaService.getThumbnailUrl(media.id) : '';

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (['ArrowLeft', 'ArrowRight', 'Enter'].includes(e.key)) {
        e.preventDefault();
      }

      const isTv = document.body.classList.contains('tv-mode');

      if (e.key === 'ArrowRight') {
        if (isTv) return;
        setFocusedBtn(prev => {
          if (prev === 'play') return 'edit';
          if (prev === 'edit') return 'delete';
          return prev;
        });
      } else if (e.key === 'ArrowLeft') {
        if (isTv) return;
        setFocusedBtn(prev => {
          if (prev === 'delete') return 'edit';
          if (prev === 'edit') return 'play';
          return prev;
        });
      } else if (e.key === 'Enter') {
        if (focusedBtn === 'play') {
          onPlay(media);
        } else if (focusedBtn === 'edit' && !isTv) {
          onEdit();
        } else if (focusedBtn === 'delete' && !isTv) {
          onDelete();
        }
      } else if (e.key === 'Escape' || e.key === 'Backspace' || e.key === 'BrowserBack') {
        e.preventDefault();
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [focusedBtn, media, onPlay, onEdit, onDelete, onClose]);

  const formatDuration = (secs: number) => {
    if (secs <= 0) return 'Unknown';
    const mins = Math.round(secs / 60);
    return t('durationFormat', { minutes: mins });
  };

  return (
    <div className="detail-page-overlay">
      <div className="detail-modal">
        <button className="detail-close-btn" onClick={onClose}>✕</button>
        
        <div className="detail-layout">
          <div className="detail-poster-sec">
            {posterUrl ? (
              <img src={posterUrl} alt={media.title} className="detail-poster" />
            ) : (
              <div className="detail-poster-fallback">
                <span>{media.title.slice(0, 2).toUpperCase()}</span>
              </div>
            )}
          </div>

          <div className="detail-info-sec">
            <h1 className="detail-title">{media.title}</h1>
            
            {media.status === 'processing' && (
              <div className="detail-processing-banner">
                <span className="banner-icon">⚙️</span>
                <span>
                  {t('bgProcessingNotice', { defaultValue: 'Scrubber preview & thumbnails are generating in the background. You can start playing the video now!' })}
                </span>
              </div>
            )}
            
            <div className="detail-meta-row">
              {media.year > 0 && (
                <div className="meta-item">
                  <span className="label">{t('detailsYear')}</span>
                  <span className="val">{media.year}</span>
                </div>
              )}
              {media.quality && (
                <div className="meta-item">
                  <span className="label">{t('detailsQuality')}</span>
                  <span className="val badge">{media.quality.toUpperCase()}</span>
                </div>
              )}
              {media.genre && (
                <div className="meta-item">
                  <span className="label">Genre</span>
                  <span className="val">{media.genre}</span>
                </div>
              )}
              {media.language && (
                <div className="meta-item">
                  <span className="label">{t('detailsLanguage')}</span>
                  <span className="val">{media.language.toUpperCase()}</span>
                </div>
              )}
              {media.duration > 0 && (
                <div className="meta-item">
                  <span className="label">{t('detailsDuration')}</span>
                  <span className="val">{formatDuration(media.duration)}</span>
                </div>
              )}
            </div>

            <div className="detail-specs">
              <p><strong>{t('detailsPath')}:</strong> <code className="path-code">{media.file_path}</code></p>
              <p><strong>Size:</strong> {(media.file_size / (1024 * 1024 * 1024)).toFixed(2)} GB</p>
              <p><strong>MIME Type:</strong> {media.mime_type}</p>
              <p><strong>Source:</strong> {media.source.toUpperCase()}</p>
            </div>

            <div className="detail-action-buttons">
              <button 
                className={`btn-detail play ${focusedBtn === 'play' ? 'focused' : ''}`} 
                onClick={() => onPlay(media)}
              >
                ▶ {t('heroPlay')}
              </button>
              {!document.body.classList.contains('tv-mode') && (
                <>
                  <button 
                    className={`btn-detail edit ${focusedBtn === 'edit' ? 'focused' : ''}`} 
                    onClick={onEdit}
                  >
                    ✏ {t('editBtn')}
                  </button>
                  <button 
                    className={`btn-detail delete ${focusedBtn === 'delete' ? 'focused' : ''}`} 
                    onClick={onDelete}
                  >
                    🗑 {t('deleteBtn')}
                  </button>
                </>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
