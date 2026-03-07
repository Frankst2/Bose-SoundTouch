package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gesellix/bose-soundtouch/pkg/client"
	"github.com/gesellix/bose-soundtouch/pkg/service/setup"
	"github.com/go-chi/chi/v5"
)

// HandleGetStockholmDeviceInfo returns live information for a device.
func (s *Server) HandleGetStockholmDeviceInfo(w http.ResponseWriter, r *http.Request) {
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

	info, err := s.sm.GetLiveDeviceInfo(deviceIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Include IP address in both snake_case and camelCase for frontend compatibility
	type deviceInfoResponse struct {
		*setup.DeviceInfoXML `json:",inline"`
		IPAddress            string `json:"ip_address"`
		IPAddressCamel       string `json:"ipAddress,omitempty"`
	}

	resp := deviceInfoResponse{
		DeviceInfoXML:  info,
		IPAddress:      deviceIP,
		IPAddressCamel: deviceIP,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleDeviceKey sends a key command to a device.
func (s *Server) HandleDeviceKey(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")

	key := chi.URLParam(r, "key")
	if deviceID == "" || key == "" {
		http.Error(w, "Device ID and Key are required", http.StatusBadRequest)
		return
	}

	deviceIP, err := s.lookupIP(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	c := client.NewClientFromHost(deviceIP)

	err = c.SendKey(key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send key %s to %s: %v", key, deviceIP, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Key sent"}); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleDeviceVolume sets the volume level for a device.
func (s *Server) HandleDeviceVolume(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")

	levelStr := chi.URLParam(r, "level")
	if deviceID == "" || levelStr == "" {
		http.Error(w, "Device ID and Level are required", http.StatusBadRequest)
		return
	}

	deviceIP, err := s.lookupIP(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	level, err := strconv.Atoi(levelStr)
	if err != nil {
		http.Error(w, "Invalid volume level", http.StatusBadRequest)
		return
	}

	c := client.NewClientFromHost(deviceIP)

	err = c.SetVolume(level)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to set volume to %d on %s: %v", level, deviceIP, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Volume set"}); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
