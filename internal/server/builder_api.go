package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// v1 handlers — agent builder
// ---------------------------------------------------------------------------

// handleV1CreateBuild handles POST /api/v1/builds — creates a new agent build
// (admin only). The build runs in a background goroutine and the build ID is
// returned immediately.
func (s *Server) handleV1CreateBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "use POST")
		return
	}

	if _, ok := s.v1CheckAuth(w, r, "operator-manage"); !ok {
		return
	}

	var cfg BuildConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON: "+err.Error())
		return
	}

	build, err := s.builder.CreateBuild(&cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, build)
}

// handleV1ListBuilds handles GET /api/v1/builds — lists all build records.
func (s *Server) handleV1ListBuilds(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.v1CheckAuth(w, r, "list"); !ok {
		return
	}
	builds := s.builder.ListBuilds()
	writeJSON(w, http.StatusOK, builds)
}

// handleV1GetBuild handles GET /api/v1/builds/{id} — returns a single build's
// status.
func (s *Server) handleV1GetBuild(w http.ResponseWriter, r *http.Request) {
	buildID := r.PathValue("id")
	if _, ok := s.v1CheckAuth(w, r, "list"); !ok {
		return
	}
	build := s.builder.GetBuild(buildID)
	if build == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "build not found")
		return
	}
	writeJSON(w, http.StatusOK, build)
}

// handleV1DownloadBuild handles GET /api/v1/builds/{id}/download — serves the
// built binary for download.
func (s *Server) handleV1DownloadBuild(w http.ResponseWriter, r *http.Request) {
	buildID := r.PathValue("id")
	if _, ok := s.v1CheckAuth(w, r, "operator-manage"); !ok {
		return
	}
	build := s.builder.GetBuild(buildID)
	if build == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "build not found")
		return
	}
	if build.Status != BuildStatusComplete {
		writeError(w, http.StatusConflict, "BUILD_NOT_READY", "build status: "+build.Status)
		return
	}
	if build.BinaryPath == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "binary path is empty")
		return
	}
	// Check the file exists.
	if _, err := os.Stat(build.BinaryPath); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "binary file not found on disk")
		return
	}
	filename := filepath.Base(build.BinaryPath)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, build.BinaryPath)
}

// handleV1DeleteBuild handles DELETE /api/v1/builds/{id} — deletes a build
// and its binary.
func (s *Server) handleV1DeleteBuild(w http.ResponseWriter, r *http.Request) {
	buildID := r.PathValue("id")
	if _, ok := s.v1CheckAuth(w, r, "operator-manage"); !ok {
		return
	}
	if !s.builder.DeleteBuild(buildID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "build not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": buildID})
}

// ---------------------------------------------------------------------------
// v1 handlers — build profiles
// ---------------------------------------------------------------------------

// handleV1ListProfiles handles GET /api/v1/profiles — lists all build profiles.
func (s *Server) handleV1ListProfiles(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.v1CheckAuth(w, r, "list"); !ok {
		return
	}
	profiles := s.profiles.List()
	writeJSON(w, http.StatusOK, profiles)
}

// handleV1CreateProfile handles POST /api/v1/profiles — creates a new build
// profile (admin only).
func (s *Server) handleV1CreateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "use POST")
		return
	}

	if _, ok := s.v1CheckAuth(w, r, "operator-manage"); !ok {
		return
	}

	var p Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON: "+err.Error())
		return
	}

	created, err := s.profiles.Create(&p)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// handleV1GetProfile handles GET /api/v1/profiles/{id} — returns a single
// build profile.
func (s *Server) handleV1GetProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("id")
	if _, ok := s.v1CheckAuth(w, r, "list"); !ok {
		return
	}
	profile := s.profiles.Get(profileID)
	if profile == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// handleV1DeleteProfile handles DELETE /api/v1/profiles/{id} — deletes a
// build profile (admin only).
func (s *Server) handleV1DeleteProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("id")
	if _, ok := s.v1CheckAuth(w, r, "operator-manage"); !ok {
		return
	}
	if !s.profiles.Delete(profileID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": profileID})
}