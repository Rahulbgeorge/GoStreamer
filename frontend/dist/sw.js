const CACHE_NAME = 'gostreamer-cache-v1';
const ASSETS_TO_CACHE = [
  '/',
  '/index.html',
  '/favicon.svg',
  '/icons.svg',
];

// Install Event - Pre-cache Static Assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      console.log('[Service Worker] Pre-caching static assets');
      return cache.addAll(ASSETS_TO_CACHE);
    }).then(() => self.skipWaiting())
  );
});

// Activate Event - Clean up Old Caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cache) => {
          if (cache !== CACHE_NAME) {
            console.log('[Service Worker] Deleting old cache:', cache);
            return caches.delete(cache);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

// Fetch Event - Serve Cached Assets (Stale-While-Revalidate / Cache-First)
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // Bypass cache for API endpoints, hot-reloading (Vite), and chrome extensions
  if (url.pathname.startsWith('/api/') || url.hostname === 'localhost' && url.port === '5173' && url.pathname.includes('@vite')) {
    return;
  }

  // Range requests (e.g. streaming videos) should bypass cache as Cache API doesn't support them fully
  if (event.request.headers.has('range')) {
    return;
  }

  event.respondWith(
    caches.match(event.request).then((cachedResponse) => {
      if (cachedResponse) {
        // Fetch in background to update cache (Stale-While-Revalidate)
        fetch(event.request).then((networkResponse) => {
          if (networkResponse.status === 200) {
            caches.open(CACHE_NAME).then((cache) => cache.put(event.request, networkResponse));
          }
        }).catch(() => { /* ignore */ });
        return cachedResponse;
      }
      return fetch(event.request);
    })
  );
});

// Message Event - Receive Upload & Download Status updates from the React App
self.addEventListener('message', (event) => {
  const { type, payload } = event.data;

  if (type === 'UPLOAD_PROGRESS') {
    const { filename, progress, uploadedBytes, totalBytes, speed } = payload;
    const mbUploaded = (uploadedBytes / (1024 * 1024)).toFixed(1);
    const mbTotal = (totalBytes / (1024 * 1024)).toFixed(1);
    const speedText = speed !== null && speed !== undefined ? ` @ ${speed.toFixed(2)} MB/s` : '';

    self.registration.showNotification(`Uploading ${filename}`, {
      body: `${progress}% complete (${mbUploaded} MB / ${mbTotal} MB)${speedText}`,
      tag: 'upload-progress',
      icon: '/favicon.svg',
      silent: true,
      renotify: false
    });
  }

  if (type === 'UPLOAD_COMPLETE') {
    const { filename } = payload;
    // Close the progress notification first
    self.registration.getNotifications({ tag: 'upload-progress' }).then((notifications) => {
      notifications.forEach((n) => n.close());
    });

    self.registration.showNotification('Upload Complete! 🎉', {
      body: `File "${filename}" has been successfully uploaded and processed.`,
      icon: '/favicon.svg',
      tag: 'upload-complete'
    });
  }

  if (type === 'UPLOAD_ERROR') {
    const { filename } = payload;
    self.registration.getNotifications({ tag: 'upload-progress' }).then((notifications) => {
      notifications.forEach((n) => n.close());
    });

    self.registration.showNotification('Upload Failed ⚠️', {
      body: `Failed to upload "${filename}". Please check your network connection.`,
      icon: '/favicon.svg',
      tag: 'upload-error'
    });
  }

  // Handle download events (satisfying the service worker for downloading requirement)
  if (type === 'DOWNLOAD_PROGRESS') {
    const { title, progress, speed } = payload;
    const speedText = speed ? ` @ ${speed}` : '';
    self.registration.showNotification(`Downloading ${title}`, {
      body: `Progress: ${progress}%${speedText}`,
      tag: 'download-progress',
      icon: '/favicon.svg',
      silent: true,
      renotify: false
    });
  }

  if (type === 'DOWNLOAD_COMPLETE') {
    const { title } = payload;
    self.registration.getNotifications({ tag: 'download-progress' }).then((notifications) => {
      notifications.forEach((n) => n.close());
    });

    self.registration.showNotification('Download Complete! 📥', {
      body: `"${title}" has been downloaded.`,
      icon: '/favicon.svg',
      tag: 'download-complete'
    });
  }
});
