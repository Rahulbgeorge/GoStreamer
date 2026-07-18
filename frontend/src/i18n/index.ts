import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

const resources = {
  en: {
    translation: {
      brand: "StreamingPlayer",
      home: "Home",
      search: "Search",
      admin: "Admin",
      movies: "Movies",
      searchPlaceholder: "Search by title...",
      heroPlay: "Play Now",
      heroInfo: "More Info",
      recentlyAdded: "Recently Added",
      allMovies: "All Movies",
      statsTitle: "Library Overview",
      statsCount: "Total Videos",
      statsSize: "Total Storage",
      noMovies: "No videos cataloged yet. Paste them into the media directory or use the Admin panel to start importing!",
      detailsYear: "Year",
      detailsQuality: "Quality",
      detailsLanguage: "Language",
      detailsDuration: "Duration",
      detailsPath: "Location",
      editBtn: "Edit Info",
      deleteBtn: "Delete",
      saveBtn: "Save Updates",
      cancelBtn: "Cancel",
      editTitle: "Edit Video Information",
      uploadTitle: "Manually Upload Videos",
      uploadHelp: "Choose a file to start chunked streaming upload",
      torrentTitle: "Download via Magnet / Torrent",
      torrentPlaceholder: "Paste magnet URI link here...",
      torrentBtn: "Start Download",
      langLabel: "Language Selection",
      durationFormat: "{{minutes}}m",
      scanTitle: "Scan Local Directory",
      scanHelp: "Trigger a directory scan to automatically catalog new movie files and remove entries for deleted ones.",
      scanBtn: "Scan Folder Now",
      scanSuccess: "Scan initiated! The library is updating in the background.",
      settingsTitle: "Settings & Folders",
      settingsHelp: "Browse and select the active media folder on the host system.",
      settingsSaveBtn: "Select & Save Folder",
      settingsSuccess: "Media folder updated successfully in database!"
    }
  },
  hi: {
    translation: {
      brand: "स्ट्रीमिंग प्लेयर",
      home: "होम",
      search: "खोजें",
      admin: "प्रशासन",
      movies: "फिल्में",
      searchPlaceholder: "शीर्षक से खोजें...",
      heroPlay: "अभी चलाएं",
      heroInfo: "अधिक जानकारी",
      recentlyAdded: "हाल ही में जोड़ी गई",
      allMovies: "सभी फिल्में",
      statsTitle: "लाइब्रेरी अवलोकन",
      statsCount: "कुल वीडियो",
      statsSize: "कुल स्टोरेज",
      noMovies: "अभी तक कोई वीडियो सूचीबद्ध नहीं है। उन्हें मीडिया निर्देशिका में डालें या आयात शुरू करने के लिए व्यवस्थापक पैनल का उपयोग करें!",
      detailsYear: "वर्ष",
      detailsQuality: "गुणवत्ता",
      detailsLanguage: "भाषा",
      detailsDuration: "अवधि",
      detailsPath: "स्थान",
      editBtn: "जानकारी संपादित करें",
      deleteBtn: "हटाएं",
      saveBtn: "सुरक्षित करें",
      cancelBtn: "रद्द करें",
      editTitle: "वीडियो जानकारी संपादित करें",
      uploadTitle: "मैन्युअल रूप से वीडियो अपलोड करें",
      uploadHelp: "चंक स्ट्रीमिंग अपलोड शुरू करने के लिए एक फ़ाइल चुनें",
      torrentTitle: "चुंबक / टोरेंट के माध्यम से डाउनलोड करें",
      torrentPlaceholder: "यहां चुंबक यूआरआई लिंक पेस्ट करें...",
      torrentBtn: "डाउनलोड शुरू करें",
      langLabel: "भाषा चुनें",
      durationFormat: "{{minutes}} मिनट",
      scanTitle: "स्थानीय निर्देशिका स्कैन करें",
      scanHelp: "नए वीडियो खोजने और हटाए गए वीडियो को कैटलॉग से हटाने के लिए फ़ोल्डर स्कैन शुरू करें।",
      scanBtn: "फ़ोल्डर स्कैन करें",
      scanSuccess: "स्कैन सफलतापूर्वक शुरू हुआ! आपका कैटलॉग पृष्ठभूमि में अपडेट हो रहा है।",
      settingsTitle: "सेटिंग्स और फ़ोल्डर्स",
      settingsHelp: "होस्ट सिस्टम पर सक्रिय मीडिया फ़ोल्डर ब्राउज़ करें और चुनें।",
      settingsSaveBtn: "फ़ोल्डर चुनें और सुरक्षित करें",
      settingsSuccess: "डेटाबेस में मीडिया फ़ोल्डर सफलतापूर्वक अपडेट किया गया!"
    }
  }
};

i18n
  .use(initReactI18next)
  .init({
    resources,
    lng: localStorage.getItem('lang') || 'en',
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false
    }
  });

export default i18n;
