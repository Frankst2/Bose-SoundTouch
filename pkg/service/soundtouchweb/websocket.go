package soundtouchweb

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gesellix/bose-soundtouch/pkg/models"
	"github.com/gesellix/bose-soundtouch/pkg/service/soundtouchweb/webtypes"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// HandleWebSocket handles browser WebSocket connections for real-time updates.
func (app *WebApp) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := app.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	defer func() {
		app.WSMutex.Lock()
		delete(app.WSClients, conn)
		app.WSMutex.Unlock()
		conn.Close()
	}()

	app.WSMutex.Lock()
	app.WSClients[conn] = true
	app.WSMutex.Unlock()

	devices := make(map[string]interface{})
	for id, device := range app.Devices {
		devices[id] = map[string]interface{}{
			"info":     device.DeviceInfo,
			"status":   device.Status,
			"lastSeen": device.LastSeen,
		}
	}

	if err := conn.WriteJSON(webtypes.WebSocketMessage{Type: "devices", Data: devices}); err != nil {
		log.Printf("Failed to send initial data: %v", err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	go func() {
		defer conn.Close()
		for {
			if _, _, err := conn.NextReader(); err != nil {
				log.Printf("WebSocket read error: %v", err)
				return
			}
		}
	}()

	for range ticker.C {
		if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
			log.Printf("Failed to send ping: %v", err)
			return
		}

		for id, device := range app.Devices {
			if device.Status.IsConnected {
				if err := conn.WriteJSON(webtypes.WebSocketMessage{
					Type:     "status_update",
					DeviceID: id,
					Data:     device.Status,
				}); err != nil {
					log.Printf("Failed to send status update: %v", err)
					return
				}
			}
		}
	}
}

// HandleAPIDiscover acknowledges a discovery request (actual discovery is triggered by Mount).
func (app *WebApp) HandleAPIDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(webtypes.APIResponse{
		Success: true,
		Data:    map[string]string{"message": "Discovery started"},
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// ConnectDeviceWebSocket establishes a WebSocket connection to a SoundTouch device.
func (app *WebApp) ConnectDeviceWebSocket(deviceID string, conn *webtypes.DeviceConnection) {
	if conn.Client == nil {
		return
	}

	wsClient := conn.Client.NewWebSocketClient(nil)

	wsClient.OnNowPlaying(func(event *models.NowPlayingUpdatedEvent) {
		conn.Status.NowPlaying = &event.NowPlaying
		conn.Status.LastActivity = time.Now()
	})

	wsClient.OnVolumeUpdated(func(event *models.VolumeUpdatedEvent) {
		conn.Status.Volume = &event.Volume
		conn.Status.LastActivity = time.Now()
	})

	wsClient.OnConnectionState(func(event *models.ConnectionStateUpdatedEvent) {
		conn.Status.IsConnected = event.ConnectionState.IsConnected()
		conn.Status.LastActivity = time.Now()
	})

	wsClient.OnPresetUpdated(func(event *models.PresetUpdatedEvent) {
		conn.Status.Presets = &event.Presets
		conn.Status.LastActivity = time.Now()
	})

	if err := wsClient.Connect(); err != nil {
		log.Printf("Failed to connect WebSocket for device %s: %v", deviceID, err)
		return
	}

	conn.WebSocket = wsClient
	conn.Status.IsConnected = true

	log.Printf("WebSocket connected for device %s", deviceID)

	wsClient.Wait()

	conn.Status.IsConnected = false

	log.Printf("WebSocket disconnected for device %s", deviceID)
}

// UpdateDeviceStatus fetches current status from a device.
func (app *WebApp) UpdateDeviceStatus(_ string, conn *webtypes.DeviceConnection) {
	if conn.Client == nil {
		return
	}

	statusUpdated := false

	if nowPlaying, err := conn.Client.GetNowPlaying(); err == nil {
		conn.Status.NowPlaying = nowPlaying
		statusUpdated = true
	}

	if volume, err := conn.Client.GetVolume(); err == nil {
		conn.Status.Volume = volume
		statusUpdated = true
	}

	if presets, err := conn.Client.GetPresets(); err == nil {
		conn.Status.Presets = presets
		statusUpdated = true
	}

	if statusUpdated {
		conn.Status.LastActivity = time.Now()
	}

	if sources, err := conn.Client.GetSources(); err == nil {
		conn.Status.Sources = sources
	}

	if bass, err := conn.Client.GetBass(); err == nil {
		conn.Status.Bass = bass
	}

	conn.Status.IsConnected = statusUpdated
	conn.Status.LastActivity = time.Now()
}

// HandleDeviceWebSocket handles per-device WebSocket connections for real-time device-specific updates.
func (app *WebApp) HandleDeviceWebSocket(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	if deviceID == "" {
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	device, exists := app.Devices[deviceID]
	if !exists {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	conn, err := app.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Device WebSocket upgrade failed for %s: %v", deviceID, err)
		return
	}
	defer conn.Close()

	log.Printf("Device WebSocket connected for %s", deviceID)

	if err := conn.WriteJSON(webtypes.WebSocketMessage{
		Type:     "device_status",
		DeviceID: deviceID,
		Data:     map[string]interface{}{"info": device.DeviceInfo, "status": device.Status},
	}); err != nil {
		log.Printf("Failed to send initial device status: %v", err)
		return
	}

	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	go func() {
		defer conn.Close()
		for {
			if _, _, err := conn.NextReader(); err != nil {
				log.Printf("Device WebSocket read error for %s: %v", deviceID, err)
				return
			}
		}
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
			log.Printf("Failed to send ping to device WebSocket %s: %v", deviceID, err)
			return
		}

		if err := conn.WriteJSON(webtypes.WebSocketMessage{
			Type:     "device_status",
			DeviceID: deviceID,
			Data:     map[string]interface{}{"info": device.DeviceInfo, "status": device.Status},
		}); err != nil {
			log.Printf("Failed to send device status update for %s: %v", deviceID, err)
			return
		}

		if device.WebSocket != nil && device.Status.IsConnected {
			if err := conn.WriteJSON(webtypes.WebSocketMessage{
				Type:     "device_realtime",
				DeviceID: deviceID,
				Data: map[string]interface{}{
					"nowPlaying": device.Status.NowPlaying,
					"volume":     device.Status.Volume,
					"timestamp":  time.Now(),
				},
			}); err != nil {
				log.Printf("Failed to send realtime update for %s: %v", deviceID, err)
				return
			}
		}
	}
}

// BroadcastDeviceList sends the updated device list to all connected browser WebSocket clients.
func (app *WebApp) BroadcastDeviceList() {
	app.WSMutex.RLock()
	defer app.WSMutex.RUnlock()

	devices := make(map[string]interface{})
	for id, device := range app.Devices {
		devices[id] = map[string]interface{}{
			"info":     device.DeviceInfo,
			"status":   device.Status,
			"lastSeen": device.LastSeen,
		}
	}

	app.broadcast(webtypes.WebSocketMessage{Type: "devices", Data: devices})
}

// BroadcastDiscoveryStatus sends discovery progress to all connected browser WebSocket clients.
func (app *WebApp) BroadcastDiscoveryStatus(status string, deviceCount int) {
	app.WSMutex.RLock()
	defer app.WSMutex.RUnlock()

	app.broadcast(webtypes.WebSocketMessage{
		Type: "discovery_status",
		Data: map[string]interface{}{"status": status, "deviceCount": deviceCount},
	})
}

// broadcast sends a message to all registered WS clients, removing failed ones.
// Caller must hold at least a read lock on WSMutex.
func (app *WebApp) broadcast(msg webtypes.WebSocketMessage) {
	var failed []*websocket.Conn

	for client := range app.WSClients {
		if err := client.WriteJSON(msg); err != nil {
			log.Printf("Failed to broadcast to WebSocket client: %v", err)
			failed = append(failed, client)
		}
	}

	for _, client := range failed {
		delete(app.WSClients, client)
		client.Close()
	}
}