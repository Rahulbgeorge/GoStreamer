# Android App Integration Guide (Capacitor)

This directory contains the React-based frontend of the **StreamingPlayer** application, which is configured to compile into a native Android app using **Capacitor**. It supports both standard Android mobile devices and Android TV (Leanback interface).

---

## 🛠️ System Prerequisites

Before building or running the native Android application, ensure you have the following installed and configured on your machine:

1. **Node.js & npm**: Node.js `v18` or higher is recommended.
2. **Android Studio**: Install the latest stable version of [Android Studio](https://developer.android.com/studio).
3. **Android SDK & Build Tools**:
   - Open Android Studio and go to **SDK Manager** (Settings > Appearance & Behavior > System Settings > Android SDK).
   - Install **Android SDK Platform 36** (target SDK version).
   - In the **SDK Tools** tab, verify that the following are installed:
     - Android SDK Build-Tools (version 36+)
     - Android SDK Command-line Tools (latest)
     - Android Emulator (if using virtual devices)
4. **Environment Variables**:
   Ensure `ANDROID_HOME` (or `ANDROID_SDK_ROOT`) is set in your shell profile (e.g., `~/.zshrc` or `~/.bashrc`):
   ```bash
   export ANDROID_HOME=$HOME/Library/Android/sdk
   export PATH=$PATH:$ANDROID_HOME/emulator
   export PATH=$PATH:$ANDROID_HOME/platform-tools
   ```

---

## ⚙️ Capacitor & Android Configurations

The native Android project is structured around the following configuration files:

* **[capacitor.config.ts](file:///Users/rahulbg/Projects/StreamingPlayer/frontend/capacitor.config.ts)**:
  Defines the application ID (`com.streamingplayer.tv`), app name (`StreamingPlayer`), and the built web directory (`dist`). Here is the configured structure:
  ```json
  {
  	"appId": "com.streamingplayer.tv",
  	"appName": "StreamingPlayer",
  	"webDir": "dist",
  	"server": {
  		"url": "http://192.168.29.142:80",
  		"cleartext": true
  	}
  }
  ```
* **[variables.gradle](file:///Users/rahulbg/Projects/StreamingPlayer/frontend/android/variables.gradle)**:
  Specifies the target, compile, and minimum SDK levels (Min: 24, Target/Compile: 36).
* **[AndroidManifest.xml](file:///Users/rahulbg/Projects/StreamingPlayer/frontend/android/app/src/main/AndroidManifest.xml)**:
  - **Cleartext Traffic**: Enabled (`android:usesCleartextTraffic="true"`) to allow connection to the local Go backend server over HTTP.
  - **Android TV Support**: Includes the `LEANBACK_LAUNCHER` intent category and declares touchscreen requirements as optional to ensure compatibility with Android TV devices.

---

## 🚀 Step-by-Step Build & Sync Workflow

To package and run the web frontend inside the native Android container, follow these steps:

### 1. Install Web Dependencies
Ensure all npm packages (including Capacitor) are installed:
```bash
npm install
```

### 2. Build Web Assets
Compile the TypeScript and React code into optimized production assets inside the `dist` directory:
```bash
npm run build
```

### 3. Sync Assets with Android Project
Copy the built web files and update Android plugin dependencies:
```bash
npm run android:sync
```
*(This is a helper script that runs `npx cap sync android` under the hood).*

### 4. Run the Application
You can run the app using one of two methods:

#### Method A: Direct Command Line (Recommended for quick runs)
Run the application directly on a connected physical device or running emulator:
```bash
npm run android:run
```
*(Runs `npx cap run android` under the hood).*

#### Method B: Android Studio (Recommended for debugging and signing)
Open the native Android project in Android Studio:
```bash
npm run android:open
```
Once Android Studio opens the project:
1. Wait for Gradle sync to complete.
2. Select your device/emulator in the top toolbar.
3. Click the **Run (Green Play)** button.

---

## ⚠️ Important: Backend Server Connection Gotchas

Because the Go backend runs on your development machine and the Android app runs inside a simulator or physical phone, they do not share the same `localhost`.

### 1. Local Network IP Discovery
The app uses a dynamic base URL resolver in [mediaService.ts](file:///Users/rahulbg/Projects/StreamingPlayer/frontend/src/services/mediaService.ts#L9-L35):
```typescript
const defaultBase = `http://192.168.29.142/api/v1`;
```
- It sends an initial request to `defaultBase/local-ip` to fetch the Go server's local IP address and dynamically switches the API base.

### 2. Required Setup Action
If your developer machine runs on a different local IP address (e.g. `192.168.1.100`), or if you are using the Android Emulator:
- Open [mediaService.ts](file:///Users/rahulbg/Projects/StreamingPlayer/frontend/src/services/mediaService.ts) and modify `defaultBase` to:
  - **For Emulators (AVD)**: `http://10.0.2.2:8080/api/v1` (where `10.0.2.2` maps to the host's localhost, adjust port as necessary).
  - **For Physical Devices**: `http://<your-computer-local-ip>:<port>/api/v1` (both your computer and Android device must be connected to the same Wi-Fi network).

---

## 🔍 Debugging & Profiling

1. **Web Inspector (Console & Network Logs)**:
   - Run the app on your device or emulator.
   - Open **Google Chrome** on your computer.
   - Navigate to `chrome://inspect`.
   - Find your device and click **Inspect** next to the **StreamingPlayer** WebView target. This lets you debug JavaScript, view console outputs, inspect network requests, and edit CSS in real time.
2. **Native Logs**:
   - Check the **Logcat** tab in Android Studio, filtering by `Capacitor` or your app process name to inspect native hooks, permissions, or system events.
