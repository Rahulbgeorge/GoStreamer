import React from 'react';
import { MediaCard } from './MediaCard';
import { Media } from '../types/media';
import './MediaRow.css';

interface MediaRowProps {
  title: string;
  items: Media[];
  onSelect: (media: Media) => void;
  focusedIndex?: number;
  isFocusedRow?: boolean;
}

export const MediaRow: React.FC<MediaRowProps> = ({ title, items, onSelect, focusedIndex, isFocusedRow }) => {
  if (items.length === 0) return null;

  return (
    <div className="media-row-container">
      <h3 className="row-title">{title}</h3>
      <div className="row-cards-scroll">
        {items.map((item, idx) => (
          <MediaCard 
            key={item.id} 
            media={item} 
            onSelect={onSelect} 
            focused={isFocusedRow && focusedIndex === idx}
          />
        ))}
      </div>
    </div>
  );
};
