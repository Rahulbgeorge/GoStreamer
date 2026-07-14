import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Media } from '../types/media';
import './EditModal.css';

interface EditModalProps {
  media: Media;
  onSave: (updates: Partial<Media>) => void;
  onClose: () => void;
}

export const EditModal: React.FC<EditModalProps> = ({ media, onSave, onClose }) => {
  const { t } = useTranslation();
  const [title, setTitle] = useState(media.title);
  const [year, setYear] = useState(media.year.toString());
  const [quality, setQuality] = useState(media.quality);
  const [genre, setGenre] = useState(media.genre || '');
  const [language, setLanguage] = useState(media.language);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSave({
      title,
      year: parseInt(year) || 0,
      quality,
      genre,
      language
    });
  };

  return (
    <div className="modal-backdrop">
      <form className="modal-content" onSubmit={handleSubmit}>
        <h2>{t('editTitle')}</h2>
        
        <div className="form-group">
          <label>Title</label>
          <input 
            type="text" 
            value={title} 
            onChange={(e) => setTitle(e.target.value)} 
            required 
          />
        </div>

        <div className="form-row">
          <div className="form-group">
            <label>Year</label>
            <input 
              type="number" 
              value={year} 
              onChange={(e) => setYear(e.target.value)} 
            />
          </div>

          <div className="form-group">
            <label>Quality</label>
            <input 
              type="text" 
              value={quality} 
              onChange={(e) => setQuality(e.target.value)} 
              placeholder="e.g. 1080p, 4k"
            />
          </div>
        </div>

        <div className="form-group">
          <label>Genre</label>
          <input 
            type="text" 
            value={genre} 
            onChange={(e) => setGenre(e.target.value)} 
            placeholder="e.g. Action, Drama, Comedy"
          />
        </div>

        <div className="form-group">
          <label>Language</label>
          <select value={language} onChange={(e) => setLanguage(e.target.value)}>
            <option value="en">English (EN)</option>
            <option value="hi">Hindi (HI)</option>
            <option value="malayalam">Malayalam</option>
            <option value="telugu">Telugu</option>
            <option value="tamil">Tamil</option>
            <option value="kannada">Kannada</option>
            <option value="bengali">Bengali</option>
            <option value="marathi">Marathi</option>
            <option value="punjabi">Punjabi</option>
            <option value="es">Spanish (ES)</option>
            <option value="fr">French (FR)</option>
            <option value="korean">Korean</option>
            <option value="japanese">Japanese</option>
          </select>
        </div>

        <div className="modal-actions">
          <button type="button" className="btn-cancel" onClick={onClose}>
            {t('cancelBtn')}
          </button>
          <button type="submit" className="btn-save">
            {t('saveBtn')}
          </button>
        </div>
      </form>
    </div>
  );
};
