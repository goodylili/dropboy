package server

import "net/http"

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Public.
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)

	// Authenticated /api/v1.
	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/machines", s.handleMachines)
	api.HandleFunc("GET /api/v1/tree", s.handleTree)
	api.HandleFunc("GET /api/v1/status", s.handleStatus)
	api.HandleFunc("GET /api/v1/folders", s.handleFoldersList)
	api.HandleFunc("POST /api/v1/folders", s.handleFoldersAdd)
	api.HandleFunc("DELETE /api/v1/folders", s.handleFoldersRemove)
	api.HandleFunc("GET /api/v1/conflicts", s.handleConflictsList)
	api.HandleFunc("POST /api/v1/conflicts/resolve", s.handleConflictsResolve)
	api.HandleFunc("POST /api/v1/restore", s.handleRestore)
	api.HandleFunc("GET /api/v1/settings", s.handleSettingsGet)
	api.HandleFunc("PUT /api/v1/settings", s.handleSettingsPut)
	api.HandleFunc("GET /api/v1/file", s.handleFileGet)
	api.HandleFunc("GET /api/v1/events", s.handleEvents)
	api.HandleFunc("POST /api/v1/pause", s.handlePause)
	api.HandleFunc("POST /api/v1/resume", s.handleResume)
	api.HandleFunc("POST /api/v1/sync", s.handleSync)
	// /unlock and /forget-passphrase get an aggressive per-source rate limit
	// to slow brute-force passphrase guesses.
	api.Handle("POST /api/v1/unlock", s.rateLimit("unlock", s.unlockRL, http.HandlerFunc(s.handleUnlock)))
	api.Handle("POST /api/v1/forget-passphrase", s.rateLimit("forget", s.unlockRL, http.HandlerFunc(s.handleForgetPassphrase)))

	mux.Handle("/api/v1/", s.authMiddleware(api))

	// SPA assets and shell.
	mux.Handle("/", s.spaHandler())
}
