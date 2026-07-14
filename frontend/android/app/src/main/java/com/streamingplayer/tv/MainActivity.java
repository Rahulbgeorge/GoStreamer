package com.streamingplayer.tv;

import android.os.Bundle;
import android.util.Log;
import com.getcapacitor.BridgeActivity;
import java.io.IOException;
import java.net.HttpURLConnection;
import java.net.URL;

public class MainActivity extends BridgeActivity {
    private static final String TAG = "NetworkDebug";
    private static final String TARGET_URL = "http://192.168.29.142:80";

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        
        // Debugging network connectivity to the server
        new Thread(() -> {
            try {
                Log.d(TAG, "Attempting to connect to " + TARGET_URL + "...");
                URL url = new URL(TARGET_URL);
                HttpURLConnection connection = (HttpURLConnection) url.openConnection();
                connection.setConnectTimeout(5000);
                connection.connect();
                int responseCode = connection.getResponseCode();
                Log.d(TAG, "Server responded with code: " + responseCode);
                if (responseCode == 200) {
                    Log.d(TAG, "Successfully connected to server at " + TARGET_URL);
                }
                connection.disconnect();
            } catch (IOException e) {
                Log.e(TAG, "Failed to connect to server at " + TARGET_URL + ". Error: " + e.getMessage());
                Log.e(TAG, "Ensure the device is on the same Wi-Fi and the server is running on port 80.");
            }
        }).start();
    }
}
