# Task 2 — Streaming Media Server (Netflix-like)

**Status:** ⬜ Planned — awaiting approval  
**Date:** 2026-06-13  

---

## Prompt

> We are building an inhouse server for streaming media over the internet and locally to the smart TV.
> It should look similar to netflix.
>
> Features expected:
> - For each video user will pass a torrents link, directly or a torrents link
>     - if its a magnet link, directly download the video and request the user for the name of the file, usually even the downloaded file will have a name, try parsing out data like, quality, year etc. from the name and just try to keep the name, optionally user should have ability to modify it
>   - Functionality to upload video, as chunks, so that internet doesn't cause an issue
>   - TV support, use React TV to build it, such that i can compile it to app and install on tv so that it becomes easy to navigate through the site.
>
> - the video should stream really fast
> - we are using an old system without gpu support, so try to use resources wisely, and ideally the videos should just download and play really quick
>
> maintain a good directory structure to store and play the videos, and find and navigate them up easily

---

## Plan

See [implementation_plan.md](file:///Users/rahulbg/.gemini/antigravity/brain/45876af7-524b-48cb-b6aa-9fcd9e1b245c/implementation_plan.md) for the full technical plan.

### Summary

6-phase delivery:
1. **Backend Foundation** — Go project, SQLite, config, media model
2. **Media Ingestion** — Torrent downloads (anacrolix/torrent) + chunked uploads + filename parsing
3. **Video Streaming** — HTTP Range Requests (zero CPU), thumbnail generation
4. **Directory Structure** — UUID-based media storage, clean organization
5. **React Native TV App** — Netflix-style UI with react-native-tvos + react-native-video
6. **Admin Web Panel** — Browser-based library management (add torrents, upload files, edit metadata)

### Key Technology Choices

| Component | Technology | Rationale |
|---|---|---|
| Torrent client | `anacrolix/torrent` | Mature Go library, built-in streaming-while-downloading, seekable readers |
| Video streaming | `http.ServeFile` (Range Requests) | Zero CPU cost, built into Go, handles byte-range negotiation |
| TV framework | `react-native-tvos` | Best TV support, works for Android TV + Apple TV |
| Video player | `react-native-video` v7+ | ExoPlayer on Android, hardware decoding, HLS/MP4/MKV |
| TV navigation | `react-tv-space-navigation` | Spatial focus management for D-pad navigation |
| No-GPU strategy | Direct play only, no transcoding | Serve original files as-is; only remux with ffmpeg -c copy if needed |

---

## Research Completed

- Go torrent libraries (anacrolix/torrent vs cenkalti/rain)
- Video streaming without GPU (range requests, HLS remuxing, Jellyfin/Plex approach)
- React Native TV ecosystem (react-native-tvos, video players, spatial navigation)
- Android TV codec support (H.264 universal, H.265 wide, VP9 wide)
