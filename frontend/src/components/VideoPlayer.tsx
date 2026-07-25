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

  // TV & Custom controls state
  const [volume, setVolume] = useState<number>(1.0);
  const [tvFocusedOption, setTvFocusedOption] = useState<string>('playpause');
  const [showTvControls, setShowTvControls] = useState<boolean>(true);
  const controlsTimeoutRef = useRef<any>(null);
  const [isTvMode, setIsTvMode] = useState<boolean>(() => document.body.classList.contains('tv-mode'));

  // TV seeking states & refs
  const [isSeeking, setIsSeeking] = useState<boolean>(false);
  const [seekTargetTime, setSeekTargetTime] = useState<number>(0);
  const [seekOffsetAccumulated, setSeekOffsetAccumulated] = useState<number>(0);
  const [scrubberDisabled, setScrubberDisabled] = useState<boolean>(false);
  const seekTargetTimeRef = useRef<number>(0);
  const seekDebounceTimeoutRef = useRef<any>(null);
  const lastSeekPressTimeRef = useRef<number>(0);
  const seekPressStartTimeRef = useRef<number>(0);
  const wasPlayingRef = useRef<boolean>(true);

  // Scrubber preview state
  const [scrubberReady, setScrubberReady] = useState(false);
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
    setScrubberDisabled(false); // Reset error state on source change
    const checkScrubber = async () => {
      try {
        const status = await mediaService.getScrubberStatus(activeMediaId);
        if (active && status.ready) {
          setScrubberReady(true);
          scrubberReadyRef.current = true;
          scrubberIntervalRef.current = status.interval;
        } else {
          setScrubberReady(false);
          scrubberReadyRef.current = false;
        }
      } catch (err) {
        console.error("Failed to check scrubber status", err);
        setScrubberReady(false);
        scrubberReadyRef.current = false;
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

      const isTv = document.body.classList.contains('tv-mode');
      player = playerRef.current = videojs(videoElement, {
        controls: !isTv,
        autoplay: true,
        preload: 'auto',
        sources: [{ src: activeSrc, type }],
        poster: activePoster,
        fluid: false,
        playbackRates: [0.5, 1, 1.25, 1.5, 2],
        userActions: {
          hotkeys: !isTv
        }
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
      if (playerRef.current) {
        setVolume(playerRef.current.volume());
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
      const isTv = document.body.classList.contains('tv-mode');
      const isNavKey = ['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Enter'].includes(e.key);
      
      if (isNavKey && !isTv) {
        document.body.classList.add('tv-mode');
        setIsTvMode(true);
      }

      const activeTvMode = isTv || isNavKey;

      if (activeTvMode) {
        // Handle TV Remote Back / Escape keys
        if (['Escape', 'BrowserBack', 'Backspace'].includes(e.key) || ['Escape', 'BrowserBack', 'Backspace'].includes(e.code)) {
          e.preventDefault();
          e.stopPropagation();
          handleExitPlayer();
          return;
        }

        if (isNavKey) {
          e.preventDefault();
          e.stopPropagation();

          if (soundOnly) {
            // Sound-Only Mode: controls card always visible, D-pad navigates
            if (e.key === 'Enter') {
              handleTvSelect(tvFocusedOption);
            } else {
              handleTvNavigation(e.key, tvFocusedOption);
            }
          } else {
            // Video Mode: D-pad menu toggles and seeking
            if (showTvControls) {
              if (e.key === 'ArrowUp') {
                setShowTvControls(false);
              } else if (e.key === 'Enter') {
                handleTvSelect(tvFocusedOption);
              } else {
                handleTvNavigation(e.key, tvFocusedOption);
              }
            } else {
              if (e.key === 'ArrowDown') {
                setShowTvControls(true);
                setTvFocusedOption('playpause');
              } else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
                handleGlobalSeek(e.key);
              } else if (e.key === 'Enter') {
                togglePlayPause();
              }
            }
          }
          return;
        }
      } else {
        // Desktop Standard Hotkeys
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
      }
    };

    const handleKeyUp = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
        seekPressStartTimeRef.current = 0;
      }
    };

    const handleBeforeUnload = () => {
      saveLastSeenPosition();
    };

    // Use capture phase to intercept keys before they reach focused Video.js elements
    window.addEventListener('keydown', handleKeyDown, true);
    window.addEventListener('keyup', handleKeyUp, true);
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => {
      window.removeEventListener('keydown', handleKeyDown, true);
      window.removeEventListener('keyup', handleKeyUp, true);
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [clipIndex, isPlaylist, clipPlaylist.length, repeatMode, onBack, activeMediaId, tvFocusedOption, showTvControls, soundOnly]);

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

  const handleSeekChange = (offset: number) => {
    if (!playerRef.current) return;
    const curr = playerRef.current.currentTime() || 0;
    const dur = playerRef.current.duration() || 0;
    const next = Math.max(0, Math.min(dur, curr + offset));
    playerRef.current.currentTime(next);
    setCurrentTimeSec(next);
  };

  const handleVolumeChange = (increase: boolean) => {
    if (!playerRef.current) return;
    setVolume(prev => {
      const next = increase ? Math.min(1, prev + 0.05) : Math.max(0, prev - 0.05);
      playerRef.current.volume(next);
      return next;
    });
  };

  const resetControlsTimeout = () => {
    setShowTvControls(true);
    if (controlsTimeoutRef.current) {
      clearTimeout(controlsTimeoutRef.current);
    }
    if (isPlaying) {
      controlsTimeoutRef.current = setTimeout(() => {
        setShowTvControls(false);
      }, 5000);
    }
  };

  // Set up auto-hide on playing status change
  useEffect(() => {
    resetControlsTimeout();
    return () => {
      if (controlsTimeoutRef.current) {
        clearTimeout(controlsTimeoutRef.current);
      }
    };
  }, [isPlaying]);

  const handleGlobalSeek = (key: string) => {
    if (!playerRef.current) return;

    const now = Date.now();
    if (seekPressStartTimeRef.current === 0 || (now - lastSeekPressTimeRef.current > 500)) {
      seekPressStartTimeRef.current = now;
    }
    const lastPressTime = lastSeekPressTimeRef.current;
    lastSeekPressTimeRef.current = now;

    const heldDuration = now - seekPressStartTimeRef.current;

    // Base step is 5s. Double speed is 10s. If held for more than 5s, speed is 30s.
    let step = 5;
    if (heldDuration > 5000) {
      step = 30;
    } else if (heldDuration < 200 && (now - lastPressTime < 300)) {
      step = 10;
    }

    const direction = key === 'ArrowRight' ? 1 : -1;
    const delta = step * direction;

    setIsSeeking(true);
    if (showTvControls) {
      resetControlsTimeout();
    }

    const startTime = isSeeking ? seekTargetTimeRef.current : (playerRef.current.currentTime() || 0);
    const dur = playerRef.current.duration() || 0;
    const target = Math.max(0, Math.min(dur, startTime + delta));
    seekTargetTimeRef.current = target;
    setSeekTargetTime(target);

    setSeekOffsetAccumulated(prevOffset => {
      const nextOffset = (isSeeking ? prevOffset : 0) + delta;
      return nextOffset;
    });

    if (!isSeeking) {
      if (playerRef.current && !playerRef.current.paused()) {
        playerRef.current.pause();
        wasPlayingRef.current = true;
      } else {
        wasPlayingRef.current = false;
      }
    }

    if (seekDebounceTimeoutRef.current) {
      clearTimeout(seekDebounceTimeoutRef.current);
    }

    seekDebounceTimeoutRef.current = setTimeout(() => {
      if (playerRef.current) {
        playerRef.current.currentTime(seekTargetTimeRef.current);
        if (wasPlayingRef.current) {
          playerRef.current.play().catch(() => {});
        }
      }
      setIsSeeking(false);
      setSeekOffsetAccumulated(0);
    }, 1000);
  };

  const getTvRows = () => {
    const isPlaylist = clipPlaylist.length > 0;
    const row0 = [];
    if (soundOnly) {
      if (isPlaylist) row0.push('prev');
      row0.push('rewind', 'playpause', 'forward');
      if (isPlaylist) row0.push('next');
      row0.push('repeat', 'videoMode');
    } else {
      if (isPlaylist) row0.push('prev');
      row0.push('rewind', 'playpause', 'forward');
      if (isPlaylist) row0.push('next');
      row0.push('repeat', 'soundOnly');
    }
    return [row0];
  };

  const handleTvNavigation = (key: string, currentFocused: string) => {
    const rows = getTvRows();
    let currentRowIdx = -1;
    let currentColIdx = -1;

    for (let r = 0; r < rows.length; r++) {
      const colIdx = rows[r].indexOf(currentFocused);
      if (colIdx !== -1) {
        currentRowIdx = r;
        currentColIdx = colIdx;
        break;
      }
    }

    if (currentRowIdx === -1) {
      setTvFocusedOption('playpause');
      return 'playpause';
    }

    let nextOption = currentFocused;

    if (key === 'ArrowLeft') {
      const newColIdx = Math.max(0, currentColIdx - 1);
      nextOption = rows[currentRowIdx][newColIdx];
    } else if (key === 'ArrowRight') {
      const newColIdx = Math.min(rows[currentRowIdx].length - 1, currentColIdx + 1);
      nextOption = rows[currentRowIdx][newColIdx];
    } else if (key === 'ArrowDown') {
      const nextRowIdx = Math.min(rows.length - 1, currentRowIdx + 1);
      if (nextRowIdx !== currentRowIdx) {
        const ratio = currentColIdx / rows[currentRowIdx].length;
        const targetColIdx = Math.min(rows[nextRowIdx].length - 1, Math.round(ratio * rows[nextRowIdx].length));
        nextOption = rows[nextRowIdx][targetColIdx];
      }
    } else if (key === 'ArrowUp') {
      const prevRowIdx = Math.max(0, currentRowIdx - 1);
      if (prevRowIdx !== currentRowIdx) {
        const ratio = currentColIdx / rows[currentRowIdx].length;
        const targetColIdx = Math.min(rows[prevRowIdx].length - 1, Math.round(ratio * rows[prevRowIdx].length));
        nextOption = rows[prevRowIdx][targetColIdx];
      }
    }

    setTvFocusedOption(nextOption);
    return nextOption;
  };

  const handleTvSelect = (currentFocused: string) => {
    switch (currentFocused) {
      case 'prev':
        handlePrevClip();
        break;
      case 'rewind':
        handleSeekChange(-10);
        break;
      case 'playpause':
        togglePlayPause();
        break;
      case 'forward':
        handleSeekChange(10);
        break;
      case 'next':
        handleNextClip();
        break;
      case 'volDown':
        handleVolumeChange(false);
        break;
      case 'volUp':
        handleVolumeChange(true);
        break;
      case 'repeat':
        cycleRepeatMode();
        break;
      case 'soundOnly':
        setSoundOnly(true);
        setTvFocusedOption('playpause');
        break;
      case 'videoMode':
        setSoundOnly(false);
        setTvFocusedOption('playpause');
        break;
      default:
        break;
    }
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
      {/* TV Global Seek Overlay (Centered) */}
      {isTvMode && !soundOnly && isSeeking && (
        <div className="tv-seek-overlay-card">
          <div className="tv-seek-time-badge">
            {formatSecs(seekTargetTime)}
          </div>
          <div className={`tv-seek-offset-badge ${seekOffsetAccumulated >= 0 ? 'forward' : 'backward'}`}>
            {seekOffsetAccumulated >= 0 ? `+${seekOffsetAccumulated}s` : `${seekOffsetAccumulated}s`}
          </div>

          {scrubberReady && !scrubberDisabled && (
            <div className="tv-seek-thumbnail-container">
              <img 
                src={mediaService.getScrubberImageUrl(
                  activeMediaId, 
                  Math.floor(seekTargetTime / scrubberIntervalRef.current) + 1
                )} 
                alt="Seek Preview" 
                onError={() => {
                  setScrubberDisabled(true);
                  console.warn("Scrubber image failed to load, disabling scrubber previews.");
                }}
                className="tv-seek-thumbnail-img"
              />
            </div>
          )}
        </div>
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

      {/* Control overlay bar for Video Mode with Playlist buttons (Desktop) */}
      {!isTvMode && !soundOnly && (
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

      {/* TV Mode Controls Overlay (Bottom) */}
      {isTvMode && !soundOnly && showTvControls && (
        <div className="tv-controls-overlay">
          {/* Progress bar */}
          <div className="tv-progress-container">
            <span className="tv-time-text">{formatSecs(currentTimeSec)}</span>
            <div className="tv-progress-bar-bg">
              <div 
                className="tv-progress-bar-fill" 
                style={{ width: `${progressPercent}%` }} 
              />
            </div>
            <span className="tv-time-text">{formatSecs(clipEndBound)}</span>
          </div>

          {/* Controls Row */}
          <div className="tv-controls-buttons-row">
            {/* Prev Clip */}
            {isPlaylist && (
              <button 
                className={`tv-btn-pill ${tvFocusedOption === 'prev' ? 'focused' : ''}`}
                onClick={handlePrevClip}
              >
                ⏮️ Prev
              </button>
            )}

            {/* Rewind */}
            <button 
              className={`tv-btn-pill ${tvFocusedOption === 'rewind' ? 'focused' : ''}`}
              onClick={() => handleSeekChange(-10)}
            >
              ⏪ -10s
            </button>

            {/* Play/Pause */}
            <button 
              className={`tv-btn-play-pause ${tvFocusedOption === 'playpause' ? 'focused' : ''}`}
              onClick={togglePlayPause}
            >
              {isPlaying ? '⏸️ Pause' : '▶️ Play'}
            </button>

            {/* Forward */}
            <button 
              className={`tv-btn-pill ${tvFocusedOption === 'forward' ? 'focused' : ''}`}
              onClick={() => handleSeekChange(10)}
            >
              +10s ⏩
            </button>

            {/* Next Clip */}
            {isPlaylist && (
              <button 
                className={`tv-btn-pill ${tvFocusedOption === 'next' ? 'focused' : ''}`}
                onClick={handleNextClip}
              >
                Next ⏭️
              </button>
            )}

            {/* Repeat Mode */}
            <button 
              className={`tv-btn-pill ${repeatMode !== 'off' ? 'active' : ''} ${tvFocusedOption === 'repeat' ? 'focused' : ''}`}
              onClick={cycleRepeatMode}
            >
              {repeatMode === 'all' && '🔁 All'}
              {repeatMode === 'one' && '🔂 One'}
              {repeatMode === 'off' && '➡️ Off'}
            </button>

            {/* Sound Only */}
            <button 
              className={`tv-btn-pill ${tvFocusedOption === 'soundOnly' ? 'focused' : ''}`}
              onClick={() => {
                setSoundOnly(true);
                setTvFocusedOption('playpause');
              }}
            >
              🎵 Sound Only
            </button>
          </div>
        </div>
      )}

      {/* TV Mode Minimised Progress Bar Overlay (Only when seeking and controls menu is closed) */}
      {isTvMode && !soundOnly && isSeeking && !showTvControls && (
        <div className="tv-controls-overlay minimized">
          <div className="tv-progress-container">
            <span className="tv-time-text">{formatSecs(seekTargetTime)}</span>
            <div className="tv-progress-bar-bg">
              <div 
                className="tv-progress-bar-fill" 
                style={{ width: `${(seekTargetTime / clipEndBound) * 100}%` }} 
              />
            </div>
            <span className="tv-time-text">{formatSecs(clipEndBound)}</span>
          </div>
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
                tabIndex={isTvMode ? -1 : 0}
                className="audio-seek-slider"
                style={{
                  background: `linear-gradient(to right, #3b82f6 ${progressPercent}%, rgba(255,255,255,0.15) ${progressPercent}%)`,
                  pointerEvents: isTvMode ? 'none' : 'auto'
                }}
              />
              <span className="time-text">{formatSecs(clipEndBound)}</span>
            </div>

            {/* Audio Control Buttons */}
            <div className="audio-controls-row">
              {/* Repeat Mode */}
              <button 
                className={`audio-btn-icon ${repeatMode !== 'off' ? 'active' : ''} ${isTvMode && tvFocusedOption === 'repeat' ? 'focused' : ''}`}
                onClick={cycleRepeatMode}
                title="Toggle Repeat Mode"
              >
                {repeatMode === 'all' ? '🔁' : repeatMode === 'one' ? '🔂' : '➡️'}
              </button>

              {/* Previous Clip */}
              <button 
                className={`audio-btn-icon ${isTvMode && tvFocusedOption === 'prev' ? 'focused' : ''}`} 
                onClick={handlePrevClip}
                disabled={!isPlaylist}
                title="Previous Clip"
              >
                ⏮️
              </button>

              {/* Rewind */}
              <button 
                className={`audio-btn-icon ${isTvMode && tvFocusedOption === 'rewind' ? 'focused' : ''}`} 
                onClick={() => handleSeekChange(-10)}
                title="Rewind 10s"
              >
                ⏪
              </button>

              {/* Play / Pause Main Button */}
              <button 
                className={`audio-btn-play-pause ${isTvMode && tvFocusedOption === 'playpause' ? 'focused' : ''}`} 
                onClick={togglePlayPause}
              >
                {isPlaying ? '⏸️' : '▶️'}
              </button>

              {/* Forward */}
              <button 
                className={`audio-btn-icon ${isTvMode && tvFocusedOption === 'forward' ? 'focused' : ''}`} 
                onClick={() => handleSeekChange(10)}
                title="Forward 10s"
              >
                ⏩
              </button>

              {/* Next Clip */}
              <button 
                className={`audio-btn-icon ${isTvMode && tvFocusedOption === 'next' ? 'focused' : ''}`} 
                onClick={handleNextClip}
                disabled={!isPlaylist}
                title="Next Clip"
              >
                ⏭️
              </button>

              {/* Vol Down */}
              <button 
                className={`audio-btn-icon ${isTvMode && tvFocusedOption === 'volDown' ? 'focused' : ''}`} 
                onClick={() => handleVolumeChange(false)}
                title="Volume Down"
              >
                🔉
              </button>

              {/* Vol Up */}
              <button 
                className={`audio-btn-icon ${isTvMode && tvFocusedOption === 'volUp' ? 'focused' : ''}`} 
                onClick={() => handleVolumeChange(true)}
                title="Volume Up"
              >
                🔊
              </button>

              {/* Switch back to Video Mode */}
              <button 
                className={`audio-btn-icon video-switch-btn ${isTvMode && tvFocusedOption === 'videoMode' ? 'focused' : ''}`}
                onClick={() => {
                  setSoundOnly(false);
                  setTvFocusedOption('playpause');
                }}
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

            {/* Volume Status badge in soundOnly mode */}
            {isTvMode && (
              <div className="audio-vol-badge" style={{ marginTop: '0.75rem', color: '#94a3b8', fontSize: '0.85rem', fontWeight: 600 }}>
                🔊 Volume: {Math.round(volume * 100)}%
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
