package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gesellix/bose-soundtouch/pkg/client"
	"github.com/gesellix/bose-soundtouch/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

const (
	pongWait   = 40 * time.Second
	pingPeriod = 20 * time.Second // must be less than pongWait
)

// HandleDeviceWebSocket upgrades the connection and proxies device WebSocket events to the browser.
func (s *Server) HandleDeviceWebSocket(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	deviceIP, err := s.lookupIP(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Upgrade the HTTP connection to a WebSocket for the browser
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Create a SoundTouch WebSocket client for the target device
	c := client.NewClientFromHost(deviceIP)
	wsClient := c.NewWebSocketClient(client.DefaultWebSocketConfig())

	// Channel-based write pump per Gorilla best practices
	sendCh := make(chan []byte, 64) // buffer to smooth bursts
	closeCh := make(chan struct{})

	// Helper to enqueue JSON messages; drop if buffer is full to avoid blocking
	enqueue := func(v interface{}) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}

		select {
		case sendCh <- b:
		default:
			// drop to protect connection under burst
		}
	}

	// Reader: we don't expect messages from the browser; just keep the
	// connection alive by processing control frames and detect close.
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	go func() {
		defer func() {
			close(closeCh)

			_ = wsClient.Disconnect()
			_ = conn.Close()
		}()

		for {
			mt, _, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WebSocket] Browser connection closed for %s: %v", deviceIP, err)
				return
			}

			if mt == websocket.CloseMessage {
				return
			}
		}
	}()

	// Writer: single writer goroutine handles JSON writes and ping keepalive
	go func() {
		pingTicker := time.NewTicker(pingPeriod)

		defer func() {
			pingTicker.Stop()

			_ = conn.Close()
		}()

		for {
			select {
			case msg, ok := <-sendCh:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}

				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-pingTicker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-closeCh:
				return
			}
		}
	}()

	// Forward typed events with a simple envelope into the send queue
	wsClient.SetHandlers(&models.WebSocketEventHandlers{
		OnNowPlaying: func(e *models.NowPlayingUpdatedEvent) {
			enqueue(map[string]interface{}{"type": "nowPlayingUpdated", "payload": e})
		},
		OnVolumeUpdated: func(e *models.VolumeUpdatedEvent) {
			enqueue(map[string]interface{}{"type": "volumeUpdated", "payload": e})
		},
		OnConnectionState: func(e *models.ConnectionStateUpdatedEvent) {
			enqueue(map[string]interface{}{"type": "connectionStateUpdated", "payload": e})
		},
		OnPresetUpdated: func(e *models.PresetUpdatedEvent) {
			enqueue(map[string]interface{}{"type": "presetUpdated", "payload": e})
		},
		OnZoneUpdated: func(e *models.ZoneUpdatedEvent) {
			enqueue(map[string]interface{}{"type": "zoneUpdated", "payload": e})
		},
		OnBassUpdated: func(e *models.BassUpdatedEvent) {
			enqueue(map[string]interface{}{"type": "bassUpdated", "payload": e})
		},
		OnUnknownEvent: func(event *models.WebSocketEvent) {
			bytes, _ := json.Marshal(event)
			enqueue(map[string]interface{}{"type": "unknown", "payload": json.RawMessage(bytes)})
		},
		OnSpecialMessage: func(msg *models.SpecialMessage) {
			enqueue(map[string]interface{}{"type": "special", "payload": msg})
		},
	})

	// Add a separate goroutine to monitor the device connection status
	go func() {
		wsClient.Wait()
		log.Printf("[WebSocket] Device %s client terminated", deviceIP)

		_ = conn.Close()
	}()

	// Connect to the device WebSocket
	if err := wsClient.Connect(); err != nil {
		enqueue(map[string]interface{}{"type": "error", "message": err.Error()})
		return
	}

	// Optional: send an initial snapshot for convenience
	go func() {
		info, err := s.sm.GetLiveDeviceInfo(deviceIP)
		if err != nil {
			return
		}

		// Supplement with volume and now playing
		c := client.NewClientFromHost(deviceIP)
		payload := map[string]interface{}{
			"deviceID": info.DeviceID,
			"name":     info.Name,
			"type":     info.Type,
			//"maccAddress":     info.MaccAddress,
			"serialNumber":    info.SerialNumber,
			"softwareVersion": info.SoftwareVer,
			// Provide IP in both styles for frontend robustness
			"ip_address": deviceIP,
			"ipAddress":  deviceIP,
		}

		if vol, err := c.GetVolume(); err == nil {
			payload["volume"] = vol
			// Also add at top level for flatter frontend parsing
			payload["actualVolume"] = vol.ActualVolume
		}

		if np, err := c.GetNowPlaying(); err == nil {
			payload["nowPlaying"] = np
		}

		enqueue(map[string]interface{}{"type": "snapshotInfo", "payload": payload})
	}()
}
