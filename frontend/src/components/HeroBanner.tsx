import React from 'react';
import { useTranslation } from 'react-i18next';
import { Media } from '../types/media';
import { mediaService } from '../services/mediaService';
import './HeroBanner.css';

interface HeroBannerProps {
  media: Media;
  onPlay: (media: Media) => void;
  onInfo: (media: Media) => void;
}

export const HeroBanner: React.FC<HeroBannerProps> = ({ media, onPlay, onInfo }) => {
  const { t } = useTranslation();
  const backdropUrl = media.thumbnail_path ? mediaService.getThumbnailUrl(media.id, media.updated_at) : '';

  return (
    <div 
      className="hero-banner"
      style={{
        backgroundImage: backdropUrl ? `linear-gradient(to top, var(--bg-primary) 5%, rgba(11,11,15,0.3) 60%, rgba(11,11,15,0.7) 100%), url(${backdropUrl})` : 'none'
      }}
    >
      <div className="hero-content">
        <h1 className="hero-title">{media.title}</h1>
        <div className="hero-meta">
          {media.year > 0 && <span>{media.year}</span>}
          {media.quality && <span className="badge">{media.quality.toUpperCase()}</span>}
          {media.language && <span className="badge lang">{media.language.toUpperCase()}</span>}
        </div>
        <p className="hero-description">
          Filename: {media.original_name}<br/>
          Format: {media.mime_type} • File size: {(media.file_size / (1024 * 1024 * 1024)).toFixed(2)} GB
        </p>
        <div className="hero-buttons">
          <button className="btn-hero play" onClick={() => onPlay(media)}>
            <span className="icon">▶</span> {t('heroPlay')}
          </button>
          <button className="btn-hero info" onClick={() => onInfo(media)}>
            ℹ {t('heroInfo')}
          </button>
        </div>
      </div>
    </div>
  );
};
