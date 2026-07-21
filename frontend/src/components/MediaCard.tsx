import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Media } from '../types/media';
import { mediaService } from '../services/mediaService';
import './MediaCard.css';

interface MediaCardProps {
  media: Media;
  onSelect: (media: Media) => void;
  focused?: boolean;
}

export const MediaCard: React.FC<MediaCardProps> = ({ media, onSelect, focused }) => {
  const { t } = useTranslation();
  const [imgError, setImgError] = useState(false);
  const posterUrl = !imgError ? mediaService.getThumbnailUrl(media.id, media.updated_at) : '';

  const formatBytes = (bytes: number) => {
    if (bytes <= 0) return '0.00 GB';
    if (bytes >= 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <div className={`media-card ${focused ? 'focused' : ''}`} onClick={() => onSelect(media)}>
      <div className="card-image-wrapper">
        {posterUrl ? (
          <img 
            src={posterUrl} 
            alt={media.title} 
            className="card-image" 
            onError={() => setImgError(true)}
          />
        ) : (
          <div className="card-image-fallback">
            <span>{media.title.slice(0, 2).toUpperCase()}</span>
          </div>
        )}
        {media.quality && <span className="card-badge quality">{media.quality.toUpperCase()}</span>}
        {media.language && <span className="card-badge lang">{media.language.toUpperCase()}</span>}
        {media.status === 'processing' && (
          <div className="card-processing-overlay">
            <span className="spinner-icon">⚙️</span>
            <span>{t('generatingThumbnails', { defaultValue: 'Processing...' })}</span>
          </div>
        )}
      </div>
      <div className="card-info">
        <h4 className="card-title">{media.title}</h4>
        <div className="card-meta">
          {media.year > 0 && <span className="card-year">{media.year}</span>}
          {focused && <span className="card-size">{formatBytes(media.file_size)}</span>}
        </div>
      </div>
    </div>
  );
};
