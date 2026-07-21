import React, { useEffect, useRef, useState } from 'react';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import { Media, Clip } from '../types/media';
import { mediaService } from '../services/mediaService';
import './VideoPlayer.css';

interface VideoPlayerProps {
  src: string;
  type: string;
  poster?: string;
  onBack?: () => void;
  mediaId: string;
  startTime?: number;
  endTime?: number;

  // Clip Playlist & Sound-Only Mode Extensions
  clipPlaylist?: Clip[];
  initialClipIndex?: number;
  allMediaList?: Media[];
  categoryName?: string;
}

export const VideoPlayer: React.FC<VideoPlayerProps> = ({
  src,
  type,
  poster,
  onBack,
  mediaId,
  startTime,
  endTime,
  clipPlaylist = [],
  initialClipIndex = 0,
  allMediaList = [],
  categoryName
}) => {
  const videoRef = useRef<HTMLDivElement | null>(null);
  const playerRef = useRef<any>(null);

  // Playlist state
  const isPlaylist = clipPlaylist.length > 0;
  const [clipIndex, setClipIndex] = useState<number>(initialClipIndex);
  const currentClip = isPlaylist ? clipPlaylist[clipIndex] : null;

  // Player Settings
  const [repeatMode, setRepeatMode] = useState<'off' | 'all' | 'one'>('all');
  const [soundOnly, setSoundOnly] = useState<boolean>(false);
  const [isPlaying, setIsPlaying] = useState<boolean>(true);
  const [currentTimeSec, setCurrentTimeSec] = useState<number>(0);
  const [durationSec, setDurationSec] = useState<number>(0);

  // Scrubber preview state
  const [scrubberReady, setScrubberReady] = useState(false);
  const [previewState, setPreviewState] = useState<{
    show: boolean;
    x: number;
    time: string;
    imageUrl: string;
  }>({
    show: false,
    x: 0,
    time: '',
    imageUrl: '',
  });

  const scrubberReadyRef = useRef(false);
  const scrubberIntervalRef = useRef(10);
  const activeMediaId = currentClip ? currentClip.media_id : mediaId;
  const activeSrc = currentClip ? mediaService.getStreamUrl(currentClip.media_id) : src;
  const activeStartTime = currentClip ? currentClip.start_time : (startTime || 0);
  const activeEndTime = currentClip ? currentClip.end_time : endTime;

  const parentMedia = currentClip ? allMediaList.find(m => m.id === currentClip.media_id) : null;
  const activePoster = currentClip && currentClip.thumbnail_path
    ? mediaService.getClipThumbnailUrl(currentClip.id)
    : (parentMedia?.thumbnail_path ? mediaService.getThumbnailUrl(parentMedia.id) : poster);

  // Poll scrubber status for active media
  useEffect(() => {
    let active = true;
    const checkScrubber = async () => {
      try {
        const status = await mediaService.getScrubberStatus(activeMediaId);
        if (active && status.ready) {
          setScrubberReady(true);
          scrubberReadyRef.current = true;
          scrubberIntervalRef.current = status.interval;
        }
      } catch (err) {
        console.error("Failed to check scrubber status", err);
      }
    };
    checkScrubber();
    return () => {
      active = false;
    };
  }, [activeMediaId]);

  // Video.js Initialization & Re-sync on clip index change
  useEffect(() => {
    let player = playerRef.current;

    if (!player && videoRef.current) {
      const videoElement = document.createElement('video-js');
      videoElement.classList.add('vjs-big-play-centered');
      videoElement.style.width = '100vw';
      videoElement.style.height = '100vh';
      videoRef.current.appendChild(videoElement);

      player = playerRef.current = videojs(videoElement, {
        controls: true,
        autoplay: true,
        preload: 'auto',
        sources: [{ src: activeSrc, type }],
        poster: activePoster,
        fluid: false,
        playbackRates: [0.5, 1, 1.25, 1.5, 2]
      });

      player.on('play', () => setIsPlaying(true));
      player.on('pause', () => setIsPlaying(false));
    } else if (player) {
      player.src({ src: activeSrc, type });
      if (activePoster) player.poster(activePoster);
    }

    player.ready(() => {
      if (activeStartTime > 0) {
        player.currentTime(activeStartTime);
      }
      player.play().catch(() => {});
    });

    // Timeupdate listener for clip boundary enforcement & playlist auto-advance
    const handleTimeUpdate = () => {
      if (!playerRef.current) return;
      const curr = playerRef.current.currentTime() || 0;
      const dur = playerRef.current.duration() || 0;
      setCurrentTimeSec(curr);
      setDurationSec(dur);

      if (typeof activeEndTime === 'number' && activeEndTime > 0 && curr >= activeEndTime) {
        if (repeatMode === 'one') {
          playerRef.current.currentTime(activeStartTime);
          playerRef.current.play().catch(() => {});
        } else if (isPlaylist) {
          if (clipIndex + 1 < clipPlaylist.length) {
            setClipIndex(prev => prev + 1);
          } else if (repeatMode === 'all') {
            setClipIndex(0);
          } else {
            playerRef.current.pause();
          }
        } else {
          playerRef.current.pause();
        }
      }
    };

    player.on('timeupdate', handleTimeUpdate);

    return () => {
      if (player) {
        player.off('timeupdate', handleTimeUpdate);
      }
    };
  }, [clipIndex, activeSrc, activeStartTime, activeEndTime, repeatMode, isPlaylist, clipPlaylist.length, type]);

  const lastPositionRef = useRef<number>(0);

  const saveLastSeenPosition = () => {
    let pos = lastPositionRef.current;
    if (playerRef.current) {
      try {
        const curr = playerRef.current.currentTime();
        if (typeof curr === 'number' && curr > 0) {
          pos = Math.floor(curr);
        }
      } catch (e) {}
    }
    if (activeMediaId && pos > 0) {
      mediaService.saveLastSeen(activeMediaId, pos);
    }
  };

  const handleExitPlayer = () => {
    saveLastSeenPosition();
    if (onBack) onBack();
  };

  // Global D-pad Remote & Keyboard Shortcut listener
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (['Space', 'KeyK'].includes(e.code)) {
        e.preventDefault();
        togglePlayPause();
      } else if (e.code === 'KeyN' || (e.code === 'ArrowRight' && e.shiftKey)) {
        e.preventDefault();
        handleNextClip();
      } else if (e.code === 'KeyP' || (e.code === 'ArrowLeft' && e.shiftKey)) {
        e.preventDefault();
        handlePrevClip();
      } else if (e.code === 'KeyR') {
        e.preventDefault();
        cycleRepeatMode();
      } else if (e.code === 'KeyS' || e.code === 'KeyM') {
        e.preventDefault();
        setSoundOnly(prev => !prev);
      } else if (['Escape', 'BrowserBack', 'Backspace'].includes(e.code)) {
        e.preventDefault();
        handleExitPlayer();
      }
    };

    const handleBeforeUnload = () => {
      saveLastSeenPosition();
    };

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [clipIndex, isPlaylist, clipPlaylist.length, repeatMode, onBack, activeMediaId]);

  // Cleanup Video.js & save last seen position on unmount
  useEffect(() => {
    return () => {
      saveLastSeenPosition();
      if (playerRef.current && !playerRef.current.isDisposed()) {
        playerRef.current.dispose();
        playerRef.current = null;
      }
    };
  }, [activeMediaId]);

  const togglePlayPause = () => {
    if (!playerRef.current) return;
    if (playerRef.current.paused()) {
      playerRef.current.play();
    } else {
      playerRef.current.pause();
    }
  };

  const handleNextClip = () => {
    if (!isPlaylist) return;
    if (clipIndex + 1 < clipPlaylist.length) {
      setClipIndex(prev => prev + 1);
    } else if (repeatMode === 'all') {
      setClipIndex(0);
    }
  };

  const handlePrevClip = () => {
    if (!isPlaylist) return;
    if (currentTimeSec - activeStartTime > 3) {
      if (playerRef.current) playerRef.current.currentTime(activeStartTime);
    } else if (clipIndex > 0) {
      setClipIndex(prev => prev - 1);
    } else if (repeatMode === 'all') {
      setClipIndex(clipPlaylist.length - 1);
    }
  };

  const cycleRepeatMode = () => {
    setRepeatMode(prev => {
      if (prev === 'off') return 'all';
      if (prev === 'all') return 'one';
      return 'off';
    });
  };

  const formatSecs = (secs: number) => {
    if (isNaN(secs) || secs < 0) return '00:00';
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  };

  // Sound-Only Seek Slider handler
  const handleAudioSeek = (e: React.ChangeEvent<HTMLInputElement>) => {
    const seekVal = parseFloat(e.target.value);
    setCurrentTimeSec(seekVal);
    if (playerRef.current) {
      playerRef.current.currentTime(seekVal);
    }
  };

  const clipStartBound = activeStartTime || 0;
  const clipEndBound = activeEndTime || durationSec || 100;
  const clipLength = Math.max(1, clipEndBound - clipStartBound);
  const currentClipOffset = Math.max(0, currentTimeSec - clipStartBound);
  const progressPercent = Math.min(100, Math.max(0, (currentClipOffset / clipLength) * 100));

  return (
    <div className="fullscreen-player-container">
      {/* Back Button */}
      {onBack && (
        <button className="btn-player-back" onClick={handleExitPlayer}>
          ← {isPlaylist ? `Exit ${categoryName || 'Clip'} Playlist` : 'Back'}
        </button>
      )}

      {/* Video.js element container (Hidden when Sound-Only mode is enabled for zero GPU overhead) */}
      <div 
        ref={videoRef} 
        style={{ 
          width: '100%', 
          height: '100%',
          display: soundOnly ? 'none' : 'block'
        }} 
      />

      {/* Control overlay bar for Video Mode with Playlist buttons */}
      {!soundOnly && (
        <div className="player-playlist-controls-overlay">
          {/* Sound Only Mode Toggle */}
          <button 
            className="btn-control-pill" 
            onClick={() => setSoundOnly(true)}
            title="Switch to Sound-Only Audio Player"
          >
            🎵 Sound Only Mode
          </button>

          {/* Repeat Mode Toggle */}
          <button 
            className={`btn-control-pill ${repeatMode !== 'off' ? 'active' : ''}`}
            onClick={cycleRepeatMode}
          >
            {repeatMode === 'all' && '🔁 Repeat All'}
            {repeatMode === 'one' && '🔂 Repeat One'}
            {repeatMode === 'off' && '➡️ Repeat Off'}
          </button>

          {/* Playlist Next / Prev */}
          {isPlaylist && (
            <div className="playlist-nav-btns">
              <button className="btn-control-pill" onClick={handlePrevClip} title="Previous Clip (P)">
                ⏮️ Prev
              </button>
              <span className="playlist-pos-badge">
                Clip {clipIndex + 1} / {clipPlaylist.length}
              </span>
              <button className="btn-control-pill" onClick={handleNextClip} title="Next Clip (N)">
                Next ⏭️
              </button>
            </div>
          )}
        </div>
      )}

      {/* FULL-SCREEN SOUND ONLY AUDIO PLAYER MODE UI */}
      {soundOnly && (
        <div className="sound-only-audio-player">
          {/* Animated Background Glow */}
          <div className="audio-bg-glow" />

          <div className="audio-player-card">
            {/* Pulsing Album Art Vinyl Container */}
            <div className={`album-art-wrap ${isPlaying ? 'playing-spin' : ''}`}>
              {activePoster ? (
                <img src={activePoster} alt="Clip Poster" className="album-poster-img" />
              ) : (
                <div className="album-poster-fallback">🎵</div>
              )}
              <div className="vinyl-center-hole" />
            </div>

            {/* Audio Equalizer Wave Animation */}
            <div className={`equalizer-wave ${isPlaying ? 'active' : ''}`}>
              <span className="bar bar-1"></span>
              <span className="bar bar-2"></span>
              <span className="bar bar-3"></span>
              <span className="bar bar-4"></span>
              <span className="bar bar-5"></span>
            </div>

            {/* Song / Clip Metadata */}
            <div className="audio-track-info">
              <h2 className="audio-clip-title">
                {currentClip ? currentClip.title : (allMediaList.find(m => m.id === mediaId)?.title || 'Audio Stream')}
              </h2>
              {parentMedia && <h4 className="audio-parent-title">🎥 {parentMedia.title}</h4>}
              {categoryName && <span className="audio-cat-tag">🏷️ {categoryName}</span>}
            </div>

            {/* Audio Timeline & Progress Bar */}
            <div className="audio-timeline-wrap">
              <span className="time-text">{formatSecs(currentTimeSec)}</span>
              <input 
                type="range"
                min={clipStartBound}
                max={clipEndBound}
                step={0.1}
                value={currentTimeSec}
                onChange={handleAudioSeek}
                className="audio-seek-slider"
                style={{
                  background: `linear-gradient(to right, #3b82f6 ${progressPercent}%, rgba(255,255,255,0.15) ${progressPercent}%)`
                }}
              />
              <span className="time-text">{formatSecs(clipEndBound)}</span>
            </div>

            {/* Audio Control Buttons */}
            <div className="audio-controls-row">
              {/* Repeat Mode */}
              <button 
                className={`audio-btn-icon ${repeatMode !== 'off' ? 'active' : ''}`}
                onClick={cycleRepeatMode}
                title="Toggle Repeat Mode"
              >
                {repeatMode === 'all' ? '🔁' : repeatMode === 'one' ? '🔂' : '➡️'}
              </button>

              {/* Previous Clip */}
              <button 
                className="audio-btn-icon" 
                onClick={handlePrevClip}
                disabled={!isPlaylist}
                title="Previous Clip"
              >
                ⏮️
              </button>

              {/* Play / Pause Main Button */}
              <button className="audio-btn-play-pause" onClick={togglePlayPause}>
                {isPlaying ? '⏸️' : '▶️'}
              </button>

              {/* Next Clip */}
              <button 
                className="audio-btn-icon" 
                onClick={handleNextClip}
                disabled={!isPlaylist}
                title="Next Clip"
              >
                ⏭️
              </button>

              {/* Switch back to Video Mode */}
              <button 
                className="audio-btn-icon video-switch-btn"
                onClick={() => setSoundOnly(false)}
                title="Switch back to Video Mode"
              >
                🎥
              </button>
            </div>

            {/* Playlist Indicator */}
            {isPlaylist && (
              <div className="audio-playlist-indicator">
                Playing Clip {clipIndex + 1} of {clipPlaylist.length} in Category
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
