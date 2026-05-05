// Package soundtouchweb provides the SoundTouch web UI handler.
package soundtouchweb

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gesellix/bose-soundtouch/pkg/client"
	"github.com/gesellix/bose-soundtouch/pkg/config"
	"github.com/gesellix/bose-soundtouch/pkg/discovery"
	"github.com/gesellix/bose-soundtouch/pkg/models"
	bmxpkg "github.com/gesellix/bose-soundtouch/pkg/service/bmx"
	"github.com/gesellix/bose-soundtouch/pkg/service/soundtouchweb/webtypes"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

//go:embed static
var staticFS embed.FS

// WebApp holds the application state and dependencies.
type WebApp struct {
	Devices   map[string]*webtypes.DeviceConnection
	Upgrader  websocket.Upgrader
	WSClients map[*websocket.Conn]bool
	WSMutex   sync.RWMutex
}

// NewWebApp creates a bare WebApp with no discovery wired up (used by tests).
func NewWebApp() *WebApp {
	return &WebApp{
		Devices:   make(map[string]*webtypes.DeviceConnection),
		WSClients: make(map[*websocket.Conn]bool),
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
	}
}

// New creates a WebApp and starts background device discovery.
func New() *WebApp {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	cfg.DiscoveryTimeout = 10 * time.Second
	cfg.CacheEnabled = true

	app := NewWebApp()
	discoveryService := discovery.NewUnifiedDiscoveryService(cfg)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		app.BroadcastDiscoveryStatus("starting", len(app.Devices))
		app.discoverDevices(ctx, discoveryService)
		app.BroadcastDiscoveryStatus("completed", len(app.Devices))
		app.BroadcastDeviceList()
	}()

	return app
}

// Mount registers all routes on r.
func (app *WebApp) Mount(r chi.Router) {
	subFS, _ := fs.Sub(staticFS, "static")
	r.Get("/static/*", http.StripPrefix("/static", http.FileServer(http.FS(subFS))).ServeHTTP)

	r.Get("/ws", app.HandleWebSocket)

	r.Get("/api/devices", app.HandleAPIDevices)
	r.Get("/api/device/{id}", app.HandleAPIDevice)
	r.Post("/api/discover", func(w http.ResponseWriter, r *http.Request) {
		app.HandleAPIDiscover(w, r)
		go func() {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				cfg = config.DefaultConfig()
			}
			cfg.DiscoveryTimeout = 10 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			app.BroadcastDiscoveryStatus("starting", len(app.Devices))
			app.discoverDevices(ctx, discovery.NewUnifiedDiscoveryService(cfg))
			app.BroadcastDiscoveryStatus("completed", len(app.Devices))
			app.BroadcastDeviceList()
		}()
	})

	r.Get("/api/control/{id}/{action}", app.HandleAPIControl)
	r.Post("/api/control/{id}/{action}", app.HandleAPIControl)

	r.Get("/api/tunein/search", app.HandleTuneInSearch)
	r.Get("/api/tunein/navigate", app.HandleTuneInNavigate)
	r.Get("/api/tunein/navigate/*", app.HandleTuneInNavigate)
	r.Post("/api/tunein/play/{id}", app.HandlePlayTuneIn)

	r.Post("/api/device-key/{id}/{key}", app.HandleDeviceKey)
	r.Post("/api/device-volume/{id}/{volume}", app.HandleDirectVolumeControl)
	r.Post("/api/device-power/{id}", app.HandleDevicePower)
	r.Get("/api/device-power-status/{id}", app.HandleDevicePowerStatus)
	r.Get("/api/device-ws/{id}", app.HandleDeviceWebSocket)

	r.Get("/", app.serveIndex)
	r.Get("/devices", app.serveIndex)
	r.Get("/device/*", app.serveIndex)
}

func (app *WebApp) serveIndex(w http.ResponseWriter, _ *http.Request) {
	data, _ := staticFS.ReadFile("static/index.html")
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write(data)
}

func (app *WebApp) discoverDevices(ctx context.Context, discoveryService *discovery.UnifiedDiscoveryService) {
	log.Println("Starting device discovery...")

	devices, err := discoveryService.DiscoverDevices(ctx)
	if err != nil {
		log.Printf("Discovery failed: %v", err)
		app.BroadcastDiscoveryStatus("failed", len(app.Devices))
		return
	}

	log.Printf("Found %d devices", len(devices))

	for _, device := range devices {
		deviceID := device.Host

		if _, exists := app.Devices[deviceID]; exists {
			app.Devices[deviceID].LastSeen = time.Now()
			continue
		}

		clientConfig := &client.Config{
			Host:    device.Host,
			Port:    device.Port,
			Timeout: 10 * time.Second,
		}

		soundTouchClient := client.NewClient(clientConfig)

		deviceInfo, err := soundTouchClient.GetDeviceInfo()
		if err != nil {
			log.Printf("Failed to get device info for %s: %v", device.Host, err)
			continue
		}

		conn := &webtypes.DeviceConnection{
			Client:     soundTouchClient,
			DeviceInfo: deviceInfo,
			LastSeen:   time.Now(),
			Status: webtypes.DeviceStatus{
				IsConnected:  false,
				LastActivity: time.Now(),
			},
		}

		go app.UpdateDeviceStatus(deviceID, conn)

		app.Devices[deviceID] = conn

		log.Printf("Added device: %s (%s) at %s", deviceInfo.Name, deviceInfo.Type, device.Host)
	}
}

// HandleAPIDevices returns all devices as JSON.
func (app *WebApp) HandleAPIDevices(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	devices := make(map[string]interface{})
	for id, device := range app.Devices {
		devices[id] = map[string]interface{}{
			"info":     device.DeviceInfo,
			"status":   device.Status,
			"lastSeen": device.LastSeen,
		}
	}

	if err := json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: devices}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleAPIDevice returns a specific device as JSON.
func (app *WebApp) HandleAPIDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	if deviceID == "" {
		app.sendError(w, "Device ID required", http.StatusBadRequest)
		return
	}

	device, exists := app.Devices[deviceID]
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	app.UpdateDeviceStatus(deviceID, device)

	if device.WebSocket == nil {
		go app.ConnectDeviceWebSocket(deviceID, device)
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(webtypes.APIResponse{
		Success: true,
		Data:    map[string]interface{}{"info": device.DeviceInfo, "status": device.Status},
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleAPIControl handles device control commands.
func (app *WebApp) HandleAPIControl(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	action := chi.URLParam(r, "action")

	if deviceID == "" {
		app.sendError(w, "Device ID required", http.StatusBadRequest)
		return
	}

	device, exists := app.Devices[deviceID]
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	if device.WebSocket == nil {
		go app.ConnectDeviceWebSocket(deviceID, device)
	}

	w.Header().Set("Content-Type", "application/json")

	app.handleControlAction(w, r, action, device)
}

func (app *WebApp) handleControlAction(w http.ResponseWriter, r *http.Request, action string, device *webtypes.DeviceConnection) {
	switch action {
	case "play":
		if device.Client == nil {
			app.sendError(w, "Device client not available", http.StatusInternalServerError)
			return
		}
		app.sendControlResponse(w, device.Client.Play(), "Started playback")
	case "pause":
		if device.Client == nil {
			app.sendError(w, "Device client not available", http.StatusInternalServerError)
			return
		}
		app.sendControlResponse(w, device.Client.Pause(), "Paused playback")
	case "stop":
		if device.Client == nil {
			app.sendError(w, "Device client not available", http.StatusInternalServerError)
			return
		}
		app.sendControlResponse(w, device.Client.Stop(), "Stopped playback")
	case "next":
		if device.Client == nil {
			app.sendError(w, "Device client not available", http.StatusInternalServerError)
			return
		}
		app.sendControlResponse(w, device.Client.NextTrack(), "Next track")
	case "previous":
		if device.Client == nil {
			app.sendError(w, "Device client not available", http.StatusInternalServerError)
			return
		}
		app.sendControlResponse(w, device.Client.PrevTrack(), "Previous track")
	case "volume":
		app.handleVolumeControl(w, r, device)
	case "mute":
		if device.Client == nil {
			app.sendError(w, "Device client not available", http.StatusInternalServerError)
			return
		}
		app.sendControlResponse(w, device.Client.SendKey(models.KeyMute), "Toggled mute")
	case "preset":
		app.handlePresetControl(w, r, device)
	case "bass":
		app.handleBassControl(w, r, device)
	case "source":
		app.handleSourceControl(w, r, device)
	default:
		app.sendError(w, "Unknown action", http.StatusBadRequest)
	}
}

func (app *WebApp) handleVolumeControl(w http.ResponseWriter, r *http.Request, device *webtypes.DeviceConnection) {
	if r.Method != http.MethodPost {
		app.sendError(w, "POST required for volume control", http.StatusMethodNotAllowed)
		return
	}

	var volumeReq webtypes.VolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&volumeReq); err != nil {
		app.sendError(w, "Invalid volume data", http.StatusBadRequest)
		return
	}

	if volumeReq.Level < 0 || volumeReq.Level > 100 {
		app.sendError(w, "Volume must be between 0 and 100", http.StatusBadRequest)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	app.sendControlResponse(w, device.Client.SetVolume(volumeReq.Level), fmt.Sprintf("Volume set to %d", volumeReq.Level))
}

func (app *WebApp) handlePresetControl(w http.ResponseWriter, r *http.Request, device *webtypes.DeviceConnection) {
	presetParam := r.URL.Query().Get("id")
	if presetParam == "" {
		app.sendError(w, "Preset ID required", http.StatusBadRequest)
		return
	}

	presetID, err := strconv.Atoi(presetParam)
	if err != nil {
		app.sendError(w, "Invalid preset ID", http.StatusBadRequest)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	app.sendControlResponse(w, device.Client.SelectPreset(presetID), fmt.Sprintf("Selected preset %d", presetID))
}

func (app *WebApp) handleBassControl(w http.ResponseWriter, r *http.Request, device *webtypes.DeviceConnection) {
	if r.Method != http.MethodPost {
		app.sendError(w, "POST required for bass control", http.StatusMethodNotAllowed)
		return
	}

	var bassReq webtypes.BassRequest
	if err := json.NewDecoder(r.Body).Decode(&bassReq); err != nil {
		app.sendError(w, "Invalid bass data", http.StatusBadRequest)
		return
	}

	if bassReq.Level < -9 || bassReq.Level > 9 {
		app.sendError(w, "Bass must be between -9 and 9", http.StatusBadRequest)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	app.sendControlResponse(w, device.Client.SetBass(bassReq.Level), fmt.Sprintf("Bass set to %d", bassReq.Level))
}

func (app *WebApp) handleSourceControl(w http.ResponseWriter, r *http.Request, device *webtypes.DeviceConnection) {
	sourceParam := r.URL.Query().Get("name")
	if sourceParam == "" {
		app.sendError(w, "Source name required", http.StatusBadRequest)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	account := r.URL.Query().Get("account")
	app.sendControlResponse(w, device.Client.SelectSource(sourceParam, account), fmt.Sprintf("Selected source %s", sourceParam))
}

func (app *WebApp) sendControlResponse(w http.ResponseWriter, err error, successMessage string) {
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if encErr := json.NewEncoder(w).Encode(webtypes.APIResponse{
		Success: true,
		Data:    map[string]string{"message": successMessage},
	}); encErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (app *WebApp) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(webtypes.APIResponse{Success: false, Error: message}); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

// HandleDeviceKey sends a key command to a device.
func (app *WebApp) HandleDeviceKey(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	key := chi.URLParam(r, "key")

	device, exists := app.Devices[deviceID]
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	if device.WebSocket == nil {
		go app.ConnectDeviceWebSocket(deviceID, device)
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	app.sendControlResponse(w, device.Client.SendKey(key), fmt.Sprintf("Sent key command: %s", key))
}

// HandleDirectVolumeControl sets volume via URL parameter.
func (app *WebApp) HandleDirectVolumeControl(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")

	volumeLevel, err := strconv.Atoi(chi.URLParam(r, "volume"))
	if err != nil || volumeLevel < 0 || volumeLevel > 100 {
		app.sendError(w, "Invalid volume level (0-100)", http.StatusBadRequest)
		return
	}

	device, exists := app.Devices[deviceID]
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	if device.WebSocket == nil {
		go app.ConnectDeviceWebSocket(deviceID, device)
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	app.sendControlResponse(w, device.Client.SetVolume(volumeLevel), fmt.Sprintf("Volume set to %d", volumeLevel))
}

// HandleDevicePower toggles device power.
func (app *WebApp) HandleDevicePower(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")

	device, exists := app.Devices[deviceID]
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	if device.WebSocket == nil {
		go app.ConnectDeviceWebSocket(deviceID, device)
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	app.sendControlResponse(w, device.Client.SendKey("POWER"), "Power toggle command sent")
}

// HandleDevicePowerStatus returns a lightweight power status check.
func (app *WebApp) HandleDevicePowerStatus(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")

	device, exists := app.Devices[deviceID]
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	nowPlaying, err := device.Client.GetNowPlaying()
	if err != nil {
		app.sendControlResponse(w, err, "Failed to get power status")
		return
	}

	isPoweredOn := nowPlaying != nil && nowPlaying.Source != "STANDBY"

	if encErr := json.NewEncoder(w).Encode(webtypes.APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"deviceId":    deviceID,
			"isPoweredOn": isPoweredOn,
			"source":      nowPlaying.Source,
		},
	}); encErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleTuneInSearch proxies a TuneIn search to the bmx package.
func (app *WebApp) HandleTuneInSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		app.sendError(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	resp, err := bmxpkg.TuneInSearch(query)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if encErr := json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: resp}); encErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleTuneInNavigate proxies a TuneIn browse/navigate request to the bmx package.
func (app *WebApp) HandleTuneInNavigate(w http.ResponseWriter, r *http.Request) {
	wildcard := chi.URLParam(r, "*")

	var (
		resp interface{}
		err  error
	)

	if wildcard == "" {
		resp, err = bmxpkg.TuneInNavigate("", nil)
	} else {
		firstSlash := strings.Index(wildcard, "/")
		if firstSlash == -1 {
			resp, err = bmxpkg.TuneInNavigate(wildcard, nil)
		} else {
			pfx := wildcard[:firstSlash]
			rest := wildcard[firstSlash+1:]

			switch pfx {
			case "sub":
				secondSlash := strings.Index(rest, "/")
				if secondSlash == -1 {
					resp, err = bmxpkg.TuneInNavigate(rest, nil)
				} else {
					n, parseErr := strconv.Atoi(rest[:secondSlash])
					if parseErr != nil {
						resp, err = bmxpkg.TuneInNavigate(wildcard, nil)
					} else {
						resp, err = bmxpkg.TuneInNavigate(rest[secondSlash+1:], &n)
					}
				}
			case "profiles":
				parts := strings.SplitN(rest, "/", 3)
				if len(parts) < 3 {
					resp, err = bmxpkg.TuneInNavigate(wildcard, nil)
				} else {
					resp, err = bmxpkg.TuneInNavigateProfile(parts[2])
				}
			default:
				resp, err = bmxpkg.TuneInNavigate(wildcard, nil)
			}
		}
	}

	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if encErr := json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: resp}); encErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandlePlayTuneIn plays a TuneIn item on a specific device.
func (app *WebApp) HandlePlayTuneIn(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	if deviceID == "" {
		app.sendError(w, "Device ID required", http.StatusBadRequest)
		return
	}

	device, exists := app.Devices[deviceID]
	if !exists {
		app.sendError(w, fmt.Sprintf("Device '%s' not found", deviceID), http.StatusNotFound)
		return
	}

	var req struct {
		Location     string `json:"location"`
		Name         string `json:"name"`
		Type         string `json:"type"`
		ContainerArt string `json:"containerArt"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Location == "" {
		app.sendError(w, "location is required", http.StatusBadRequest)
		return
	}

	itemType := req.Type
	if itemType == "" {
		itemType = "stationurl"
	}

	contentItem := &models.ContentItem{
		Source:       "TUNEIN",
		Type:         itemType,
		Location:     req.Location,
		ItemName:     req.Name,
		IsPresetable: true,
		ContainerArt: req.ContainerArt,
	}

	if err := device.Client.SelectContentItem(contentItem); err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if encErr := json.NewEncoder(w).Encode(webtypes.APIResponse{
		Success: true,
		Data:    map[string]string{"message": "Playing " + req.Name},
	}); encErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}