package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"charmtool/internal/models"
	"charmtool/internal/storage"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	store *storage.FileStore
}

// New creates a new Handler
func New(store *storage.FileStore) *Handler {
	return &Handler{store: store}
}

// UploadPOS handles POST /api/upload/pos
func (h *Handler) UploadPOS(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Parse POS file
	posData, err := models.ParsePOS(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse POS file: %v", err), http.StatusBadRequest)
		return
	}

	// Convert to XFile
	xf := models.ConvertPOSToXFile(posData, header.Filename)

	// Save to session
	if err := h.store.UpdateSession(sessionID, xf); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	setJSONContentType(w)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"filename":   header.Filename,
		"components": len(xf.Components),
		"stations":   len(xf.Stations),
	})
}

// UploadStack handles POST /api/upload/stack
func (h *Handler) UploadStack(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	// Get current XFile
	xf, err := h.store.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Parse Stack file
	stations, err := models.ParseStack(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse Stack file: %v", err), http.StatusBadRequest)
		return
	}

	// Merge into XFile
	merged := models.MergeStationsIntoXFile(xf, stations, header.Filename)

	// Save to session
	if err := h.store.UpdateSession(sessionID, xf); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	setJSONContentType(w)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"filename": header.Filename,
		"merged":   merged,
		"total":    len(xf.Stations),
	})
}

// GetXFile handles GET /api/xfile
func (h *Handler) GetXFile(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	xf, err := h.store.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	setJSONContentType(w)
	json.NewEncoder(w).Encode(xf)
}

// UpdateXFile handles POST /api/xfile/update
func (h *Handler) UpdateXFile(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	var xf models.XFile
	if err := json.NewDecoder(r.Body).Decode(&xf); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateSession(sessionID, &xf); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	setJSONContentType(w)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// Validate handles GET /api/validate
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	xf, err := h.store.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Get filename from query param or use default
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = "output.dpv"
	}

	result := models.ValidateDPV(xf, filename)

	setJSONContentType(w)
	json.NewEncoder(w).Encode(result)
}

// ExportRequest contains optional log data for export
type ExportRequest struct {
	Log string `json:"log"`
}

// Export handles GET/POST /api/export
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	xf, err := h.store.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Get base filename from query param or derive from original POS
	baseName := r.URL.Query().Get("filename")
	if baseName == "" {
		baseName = xf.OriginalPOS
		if baseName == "" {
			baseName = "output"
		}
		// Remove extension
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	// Parse log content from POST body if present
	var logContent string
	if r.Method == http.MethodPost && r.Body != nil {
		var req ExportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			logContent = req.Log
		}
	}

	dpvFilename := baseName + ".dpv"

	// Validate before export
	validation := models.ValidateDPV(xf, dpvFilename)
	if !validation.Valid {
		setJSONContentType(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    false,
			"validation": validation,
			"message":    "DPV validation failed. Please fix errors before exporting.",
		})
		return
	}

	// Generate DPV content
	dpvContent, err := models.GenerateDPV(xf, dpvFilename)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate DPV: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate Stack content
	stackContent := models.GenerateStack(xf)

	// Create ZIP file
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add DPV file
	dpvWriter, err := zipWriter.Create(dpvFilename)
	if err != nil {
		http.Error(w, "Failed to create ZIP", http.StatusInternalServerError)
		return
	}
	io.WriteString(dpvWriter, dpvContent)

	// Add Stack file
	stackFilename := baseName + ".stack"
	stackWriter, err := zipWriter.Create(stackFilename)
	if err != nil {
		http.Error(w, "Failed to create ZIP", http.StatusInternalServerError)
		return
	}
	io.WriteString(stackWriter, stackContent)

	// Add original POS file
	if len(xf.POSRows) > 0 {
		posFilename := baseName + ".pos"
		posContent := models.GeneratePOS(xf)
		posWriter, err := zipWriter.Create(posFilename)
		if err != nil {
			http.Error(w, "Failed to create ZIP", http.StatusInternalServerError)
			return
		}
		io.WriteString(posWriter, posContent)
	}

	// Add Log file if provided
	if logContent != "" {
		logFilename := baseName + ".log"
		logWriter, err := zipWriter.Create(logFilename)
		if err != nil {
			http.Error(w, "Failed to create ZIP", http.StatusInternalServerError)
			return
		}
		io.WriteString(logWriter, logContent)
	}

	// Add README.txt with setup instructions
	readmeContent := models.GenerateReadme(xf, dpvFilename)
	readmeWriter, err := zipWriter.Create("README.txt")
	if err != nil {
		http.Error(w, "Failed to create ZIP", http.StatusInternalServerError)
		return
	}
	io.WriteString(readmeWriter, readmeContent)

	if err := zipWriter.Close(); err != nil {
		http.Error(w, "Failed to finalize ZIP", http.StatusInternalServerError)
		return
	}

	// Send ZIP file
	zipFilename := baseName + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.Write(buf.Bytes())
}
