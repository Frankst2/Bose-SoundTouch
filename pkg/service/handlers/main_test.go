package handlers

import (
	"github.com/gesellix/bose-soundtouch/pkg/service/datastore"
	"github.com/go-chi/chi/v5"
)

func setupRouter(targetURL string, ds *datastore.DataStore) (*chi.Mux, *Server) {
	server := NewServer(ds, nil, "http://localhost:8000", false, false, false, false)
	server.SetSoundcorkURL(targetURL)

	r := chi.NewRouter()
	r.Use(server.OriginMiddleware)
	r.Use(server.ShortcutMiddleware)
	r.Use(server.RecordMiddleware)

	r.Get("/", server.HandleRoot)

	// Setup media and web directories for tests
	r.Get("/media/*", server.HandleMedia())
	r.Get("/web/*", server.HandleWeb())

	// Setup BMX for tests
	r.Route("/bmx", func(r chi.Router) {
		r.Get("/registry/v1/services", server.HandleBMXRegistry)
		r.Get("/tunein/v1/playback/station/{stationID}", server.HandleTuneInPlayback)
		r.Get("/tunein/v1/playback/episodes/{podcastID}", server.HandleTuneInPodcastInfo)
		r.Get("/tunein/v1/playback/episode/{podcastID}", server.HandleTuneInPlaybackPodcast)
		r.Post("/orion/v1/playback/station/{data}", server.HandleOrionPlayback)
	})

	// Legacy or direct domain calls without /bmx prefix
	r.Get("/registry/v1/services", server.HandleBMXRegistry)
	r.Get("/tunein/v1/playback/station/{stationID}", server.HandleTuneInPlayback)
	r.Get("/tunein/v1/playback/episodes/{podcastID}", server.HandleTuneInPodcastInfo)
	r.Get("/tunein/v1/playback/episode/{podcastID}", server.HandleTuneInPlaybackPodcast)
	r.Post("/orion/v1/playback/station/{data}", server.HandleOrionPlayback)

	// Setup Marge for tests
	r.Route("/marge", func(r chi.Router) {
		r.Get("/streaming/sourceproviders", server.HandleMargeSourceProviders)
		r.Get("/accounts/{account}/full", server.HandleMargeAccountFull)
		r.Post("/streaming/support/power_on", server.HandleMargePowerOn)
		r.Get("/updates/soundtouch", server.HandleMargeSoftwareUpdate)
		r.Get("/accounts/{account}/devices/{device}/presets", server.HandleMargePresets)
		r.Post("/accounts/{account}/devices/{device}/presets/{presetNumber}", server.HandleMargeUpdatePreset)
		r.Post("/accounts/{account}/devices/{device}/recents", server.HandleMargeAddRecent)
		r.Post("/accounts/{account}/devices", server.HandleMargeAddDevice)
		r.Delete("/accounts/{account}/devices/{device}", server.HandleMargeRemoveDevice)
		r.Get("/streaming/account/{account}/provider_settings", server.HandleMargeProviderSettings)
		r.Get("/streaming/device/{device}/streaming_token", server.HandleMargeStreamingToken)
		r.Post("/streaming/support/customersupport", server.HandleMargeCustomerSupport)
		r.Get("/streaming/device_setting/account/{account}/device/{device}/device_settings", server.HandleMargeGetDeviceSettings)
		r.Post("/streaming/device_setting/account/{account}/device/{device}/device_settings", server.HandleMargeUpdateDeviceSettings)
		r.Get("/streaming/account/{account}/emailaddress", server.HandleMargeGetEmailAddress)
	})

	// Legacy or direct domain calls without /marge prefix
	r.Get("/streaming/sourceproviders", server.HandleMargeSourceProviders)
	r.Get("/accounts/{account}/full", server.HandleMargeAccountFull)
	r.Post("/streaming/support/power_on", server.HandleMargePowerOn)
	r.Get("/updates/soundtouch", server.HandleMargeSoftwareUpdate)
	r.Get("/accounts/{account}/devices/{device}/presets", server.HandleMargePresets)
	r.Post("/accounts/{account}/devices/{device}/presets/{presetNumber}", server.HandleMargeUpdatePreset)
	r.Post("/accounts/{account}/devices/{device}/recents", server.HandleMargeAddRecent)
	r.Post("/accounts/{account}/devices", server.HandleMargeAddDevice)
	r.Delete("/accounts/{account}/devices/{device}", server.HandleMargeRemoveDevice)
	r.Get("/streaming/account/{account}/provider_settings", server.HandleMargeProviderSettings)
	r.Get("/streaming/device/{device}/streaming_token", server.HandleMargeStreamingToken)
	r.Post("/streaming/support/customersupport", server.HandleMargeCustomerSupport)
	r.Get("/streaming/device_setting/account/{account}/device/{device}/device_settings", server.HandleMargeGetDeviceSettings)
	r.Post("/streaming/device_setting/account/{account}/device/{device}/device_settings", server.HandleMargeUpdateDeviceSettings)
	r.Get("/streaming/account/{account}/emailaddress", server.HandleMargeGetEmailAddress)

	// Setup Customer for tests
	r.Route("/customer", func(r chi.Router) {
		r.Get("/account/{account}", server.HandleMargeAccountProfile)
		r.Post("/account/{account}", server.HandleMargeUpdateAccountProfile)
		r.Post("/account/{account}/password", server.HandleMargeChangePassword)
	})

	// Setup Devices for tests
	r.Route("/devices", func(r chi.Router) {
		r.Get("/", server.HandleListDiscoveredDevices)
		r.Post("/", server.HandleAddManualDevice)

		r.Route("/{deviceId}", func(r chi.Router) {
			r.Delete("/", server.HandleRemoveDevice)
			r.Get("/events", server.HandleGetDeviceEvents)
			r.Get("/info", server.HandleGetDeviceInfo)
			r.Get("/ws", server.HandleDeviceWebSocket)
			r.Post("/key/{key}", server.HandleDeviceKey)
			r.Post("/volume/{level}", server.HandleDeviceVolume)
			r.Post("/reboot", server.HandleRebootDevice)
		})
	})

	r.Get("/version", server.HandleGetVersionInfo)

	// Setup Setup for tests
	r.Route("/setup", func(r chi.Router) {
		r.Post("/discover", server.HandleTriggerDiscovery)
		r.Get("/discovery-status", server.HandleGetDiscoveryStatus)
		r.Get("/settings", server.HandleGetSettings)
		r.Post("/settings", server.HandleUpdateSettings)
		r.Get("/ca.crt", server.HandleGetCACert)
		r.Get("/proxy-settings", server.HandleGetProxySettings)
		r.Post("/proxy-settings", server.HandleUpdateProxySettings)
		r.Get("/interaction-stats", server.HandleGetInteractionStats)
		r.Get("/interactions", server.HandleListInteractions)
		r.Get("/interaction-content", server.HandleGetInteractionContent)
		r.Get("/interactions/sessions/{session}/download", server.HandleDownloadSession)
		r.Delete("/interactions/sessions/{session}", server.HandleDeleteSession)
		r.Delete("/interactions/sessions", server.HandleCleanupSessions)

		r.Get("/dns-discoveries", server.HandleGetDNSDiscoveries)
		r.Delete("/dns-discoveries", server.HandleClearDNSDiscoveries)

		r.Route("/devices/{deviceId}", func(r chi.Router) {
			r.Get("/summary", server.HandleGetMigrationSummary)
			r.Post("/migrate", server.HandleMigrateDevice)
			r.Post("/revert", server.HandleRevertMigration)
			r.Post("/trust-ca", server.HandleTrustCACert)
			r.Post("/ensure-remote-services", server.HandleEnsureRemoteServices)
			r.Post("/remove-remote-services", server.HandleRemoveRemoteServices)
			r.Post("/backup", server.HandleBackupConfig)
			r.Post("/sync", server.HandleInitialSync)
			r.Post("/test-connection", server.HandleTestConnection)
			r.Post("/test-hosts", server.HandleTestHostsRedirection)
			r.Post("/test-dns", server.HandleTestDNSRedirection)
		})
	})

	r.NotFound(server.HandleNotFound)

	return r, server
}

func init() {
	// Silence logger for tests
	// log.SetOutput(io.Discard)
}
