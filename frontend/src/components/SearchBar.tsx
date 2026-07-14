import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import './SearchBar.css';

interface SearchBarProps {
  onSearch: (query: string) => void;
  isFocused?: boolean;
}

export const SearchBar: React.FC<SearchBarProps> = ({ onSearch, isFocused }) => {
  const { t } = useTranslation();
  const [val, setVal] = useState('');

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setVal(e.target.value);
    onSearch(e.target.value);
  };

  return (
    <div className={`search-bar ${isFocused ? 'focused' : ''}`}>
      <span className="search-icon">🔍</span>
      <input 
        type="text" 
        value={val}
        placeholder={t('searchPlaceholder')}
        onChange={handleChange}
        tabIndex={-1}
      />
    </div>
  );
};
