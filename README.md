# StreamingPlayer

StreamingPlayer is a personal media hosting and streaming solution. It consists of a high-performance **Go-based backend** that serves media files, processes torrents, and handles YouTube downloads, alongside a modern **React + TypeScript frontend** that plays media, tracks library stats, and manages downloads.

The project is also pre-configured with **Capacitor** to allow compiling the frontend into a native Android app (suitable for both mobile devices and Android TV/Leanback).

---

## 📂 Project Structure

* **`backend/`**: A Go application utilizing the Gin framework. It manages SQLite media metadata, serves video streams (supporting scrubber thumbnails), registers uploads, handles magnet torrent downloads, and interacts with YouTube formats.
* **`frontend/`**: A Vite-powered React and TypeScript web app using Video.js. It also contains the configuration files and native directories required for building and packaging as an Android app.
* **`tasks/`**: Archive logs detailing development progress and code verification.

---

## 🚀 Getting Started

### 1. Running the Go Backend

1. Navigate to the backend directory:
   ```bash
   cd backend
   ```
2. Build or run the server:
   ```bash
   go run ./cmd/server
   ```
   *By default, the backend will initialize SQLite databases, scan local media dirs, and start listening for API connections on its configured port (typically port 8080).*

### 2. Running the Frontend (Web Development Mode)

1. Navigate to the frontend directory:
   ```bash
   cd frontend
   ```
2. Install npm dependencies:
   ```bash
   npm install
   ```
3. Run the Vite development server:
   ```bash
   npm run dev
   ```
   *Open the returned URL (typically `http://localhost:5173`) in your web browser to access the client interface.*

---

## 📱 Compiling into an Android App

The frontend utilizes **Capacitor** to build a native Android package. This app is configured with **Leanback (Android TV)** launcher support, cleartext traffic permissions (for local HTTP servers), and a dynamic local server IP lookup.

For the step-by-step workflow on how to build, sync, and run the Android app, please refer to the detailed guide:

👉 **[Android App Integration Guide (Capacitor)](file:///Users/rahulbg/Projects/StreamingPlayer/frontend/README.android.md)**

### Quick Build & Sync Commands:
If your prerequisites (Android Studio, SDK) are already set up:
```bash
cd frontend
npm install
npm run build
npm run android:sync   # Copies built assets into the native Android folder
npm run android:run    # Runs the app on a connected device/emulator
# OR
npm run android:open   # Opens the native project in Android Studio
```
For more information on emulator network configurations, dynamic local IPs, and web inspector debugging, read the **[detailed Android guide](file:///Users/rahulbg/Projects/StreamingPlayer/frontend/README.android.md)**.
# GoStreamer
