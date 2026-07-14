import React, { useEffect, useRef, useState } from 'react';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import { mediaService } from '../services/mediaService';
import './VideoPlayer.css';

interface VideoPlayerProps {
  src: string;
  type: string;
  poster?: string;
  onBack?: () => void;
  mediaId: string;
}

export const VideoPlayer: React.FC<VideoPlayerProps> = ({ src, type, poster, onBack, mediaId }) => {
  const videoRef = useRef<HTMLDivElement | null>(null);
  const playerRef = useRef<any>(null);
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

  // Poll or check scrubber status once on play
  useEffect(() => {
    let active = true;
    const checkScrubber = async () => {
      try {
        const status = await mediaService.getScrubberStatus(mediaId);
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
  }, [mediaId]);

  useEffect(() => {
    // Make sure Video.js player is initialized only once
    if (!playerRef.current && videoRef.current) {
      const videoElement = document.createElement('video-js');
      videoElement.classList.add('vjs-big-play-centered');
      videoElement.style.width = '100vw';
      videoElement.style.height = '100vh';
      videoRef.current.appendChild(videoElement);

      const player = playerRef.current = videojs(videoElement, {
        controls: true,
        autoplay: true,
        preload: 'auto',
        sources: [{ src, type }],
        poster: poster,
        fluid: false,
        playbackRates: [0.5, 1, 1.25, 1.5, 2]
      });

      // Attach hover listener for seek previews
      const progressControl = videoRef.current?.querySelector('.vjs-progress-control') as HTMLElement;
      const playerContainer = videoRef.current;

      let handleMouseMove: ((e: MouseEvent) => void) | null = null;
      let handleMouseLeave: (() => void) | null = null;
      let previewTimeoutId: any = null;

      const showSeekPreview = (targetTime: number) => {
        if (!player || !scrubberReadyRef.current) return;
        if (previewTimeoutId) {
          clearTimeout(previewTimeoutId);
        }

        const rect = progressControl?.getBoundingClientRect();
        const containerRect = playerContainer?.getBoundingClientRect();
        const duration = player.duration() || 0;

        if (rect && containerRect && duration > 0) {
          const pct = Math.max(0, Math.min(1, targetTime / duration));
          const mins = Math.floor(targetTime / 60);
          const secs = Math.floor(targetTime % 60);
          const timeStr = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
          
          const frameNum = Math.floor(targetTime / scrubberIntervalRef.current) + 1;
          const imgUrl = mediaService.getScrubberImageUrl(mediaId, frameNum);
          
          const xRel = rect.left - containerRect.left + (pct * rect.width);

          setPreviewState({
            show: true,
            x: xRel,
            time: timeStr,
            imageUrl: imgUrl
          });

          // Hide preview after 1.5 seconds of inactivity
          previewTimeoutId = setTimeout(() => {
            setPreviewState(prev => ({ ...prev, show: false }));
          }, 1500);
        }
      };

      if (progressControl && playerContainer) {
        handleMouseMove = (e: MouseEvent) => {
          if (previewTimeoutId) {
            clearTimeout(previewTimeoutId);
          }
          if (!playerRef.current || !scrubberReadyRef.current) return;
          const rect = progressControl.getBoundingClientRect();
          const containerRect = playerContainer.getBoundingClientRect();
          
          const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
          const duration = playerRef.current.duration() || 0;
          const hoverTime = pct * duration;
          
          const mins = Math.floor(hoverTime / 60);
          const secs = Math.floor(hoverTime % 60);
          const timeStr = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
          
          const frameNum = Math.floor(hoverTime / scrubberIntervalRef.current) + 1;
          const imgUrl = mediaService.getScrubberImageUrl(mediaId, frameNum);
          
          const xRel = e.clientX - containerRect.left;

          setPreviewState({
            show: true,
            x: xRel,
            time: timeStr,
            imageUrl: imgUrl
          });
        };

        handleMouseLeave = () => {
          setPreviewState(prev => ({ ...prev, show: false }));
        };

        progressControl.addEventListener('mousemove', handleMouseMove);
        progressControl.addEventListener('mouseleave', handleMouseLeave);
      }

      let isSeeking = false;
      let targetSeekTime = 0;
      let actualSeekTimeout: any = null;
      let seekMultiplier = 1;
      let lastSeekTime = 0;
      let resetMultiplierTimeout: any = null;

      // Simple keyboard short-cuts for TV D-pad / Player control (Space/Arrows)
      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.code === 'Space') {
          if (player.paused()) player.play();
          else player.pause();
        } else if (e.code === 'ArrowLeft' || e.code === 'ArrowRight') {
          const now = Date.now();
          const timeDiff = now - lastSeekTime;
          lastSeekTime = now;

          if (resetMultiplierTimeout) {
            clearTimeout(resetMultiplierTimeout);
          }

          // Accelerate if consecutive seeks are triggered within 450ms
          if (timeDiff < 450) {
            seekMultiplier = Math.min(8, seekMultiplier + 1); // limit to 8x acceleration (80s per tap)
          } else {
            seekMultiplier = 1;
          }

          resetMultiplierTimeout = setTimeout(() => {
            seekMultiplier = 1;
          }, 800);

          const duration = player.duration() || 0;

          // Initialize targetSeekTime on starting seek session
          if (!isSeeking) {
            isSeeking = true;
            targetSeekTime = player.currentTime() || 0;
          }

          const step = 10 * seekMultiplier;
          if (e.code === 'ArrowLeft') {
            targetSeekTime = Math.max(0, targetSeekTime - step);
          } else {
            targetSeekTime = Math.min(duration, targetSeekTime + step);
          }

          // Render seek preview thumbnail instantly
          showSeekPreview(targetSeekTime);

          // Debounce the actual video player seeking to avoid buffering lag
          if (actualSeekTimeout) {
            clearTimeout(actualSeekTimeout);
          }

          actualSeekTimeout = setTimeout(() => {
            player.currentTime(targetSeekTime);
            isSeeking = false;
          }, 500); // 500ms after last arrow release, commit seek to player
        } else if (e.code === 'Escape' || e.code === 'BrowserBack' || e.code === 'Backspace') {
          if (onBack) onBack();
        }
      };

      window.addEventListener('keydown', handleKeyDown);

      playerRef.current.on('dispose', () => {
        window.removeEventListener('keydown', handleKeyDown);
        if (previewTimeoutId) clearTimeout(previewTimeoutId);
        if (actualSeekTimeout) clearTimeout(actualSeekTimeout);
        if (resetMultiplierTimeout) clearTimeout(resetMultiplierTimeout);
        if (progressControl) {
          if (handleMouseMove) progressControl.removeEventListener('mousemove', handleMouseMove);
          if (handleMouseLeave) progressControl.removeEventListener('mouseleave', handleMouseLeave);
        }
      });
    }
  }, [src, type, poster, onBack, mediaId]);

  // Dispose player on unmount
  useEffect(() => {
    return () => {
      const player = playerRef.current;
      if (player && !player.isDisposed()) {
        player.dispose();
        playerRef.current = null;
      }
    };
  }, []);

  return (
    <div style={{ position: 'fixed', top: 0, left: 0, width: '100vw', height: '100vh', backgroundColor: '#000', zIndex: 9999 }}>
      {onBack && (
        <button 
          onClick={onBack}
          style={{
            position: 'absolute',
            top: '20px',
            left: '20px',
            zIndex: 10000,
            padding: '10px 20px',
            backgroundColor: 'rgba(0,0,0,0.6)',
            border: '1px solid #333',
            borderRadius: '4px',
            cursor: 'pointer',
            fontWeight: 'bold'
          }}
        >
          ← Back
        </button>
      )}
      <div ref={videoRef} style={{ width: '100%', height: '100%' }} />

      {/* Scrubber Thumbnail Hover Preview */}
      {previewState.show && scrubberReady && (
        <div 
          className="scrubber-preview-box"
          style={{
            position: 'absolute',
            left: `${previewState.x}px`,
            bottom: '60px', // Right above the progress control bar
            transform: 'translateX(-50%)',
            pointerEvents: 'none',
            zIndex: 10005,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '6px'
          }}
        >
          <div className="preview-image-wrap">
            <img 
              src={previewState.imageUrl} 
              alt="Seek preview" 
              className="preview-image" 
              onError={(e) => {
                (e.target as HTMLElement).style.display = 'none';
              }}
            />
          </div>
          <span className="preview-time">{previewState.time}</span>
        </div>
      )}
    </div>
  );
};

