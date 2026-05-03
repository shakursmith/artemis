package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiagnosticsHandler serves beacon diagnostic endpoints.
// Used during development to capture and store beacon signal data from the
// iOS app, enabling structured analysis of room detection accuracy.
//
// Endpoints:
//   - POST /api/diagnostics/beacon-report — Receives a JSON report, writes
//     it to ./diagnostics/report-NNN.json (sequentially numbered)
//   - GET /api/diagnostics/beacon-reports — Lists existing report files and
//     returns the count (so the app knows the next report number)
type DiagnosticsHandler struct {
	// Dir is the directory where beacon reports are stored.
	// Defaults to "./diagnostics" but can be overridden for tests.
	Dir string
}

// NewDiagnosticsHandler creates a handler that writes reports to the given directory.
func NewDiagnosticsHandler(dir string) *DiagnosticsHandler {
	return &DiagnosticsHandler{Dir: dir}
}

// HandleSubmitBeaconReport handles POST /api/diagnostics/beacon-report.
//
// Accepts any JSON body (the report from BeaconDiagnosticLogger), determines
// the next sequential report number by scanning existing files in the
// diagnostics directory, and writes the report as pretty-printed JSON.
//
// Response: {"reportNumber": 1, "filename": "report-001.json"}
func (h *DiagnosticsHandler) HandleSubmitBeaconReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	// Read the request body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Validate that the body is valid JSON.
	var report json.RawMessage
	if err := json.Unmarshal(body, &report); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Ensure the diagnostics directory exists.
	if err := os.MkdirAll(h.Dir, 0755); err != nil {
		log.Printf("❌ Failed to create diagnostics directory %s: %v", h.Dir, err)
		writeError(w, http.StatusInternalServerError, "failed to create diagnostics directory")
		return
	}

	// Determine the next sequential report number.
	nextNum := h.nextReportNumber()

	// Format filename with zero-padded 3-digit number.
	filename := fmt.Sprintf("report-%03d.json", nextNum)
	filePath := filepath.Join(h.Dir, filename)

	// Pretty-print the JSON before writing.
	var prettyJSON []byte
	prettyJSON, err = json.MarshalIndent(report, "", "  ")
	if err != nil {
		// Fallback to raw body if pretty-printing fails (shouldn't happen
		// since we already validated the JSON above).
		prettyJSON = body
	}

	// Write the report file.
	if err := os.WriteFile(filePath, prettyJSON, 0644); err != nil {
		log.Printf("❌ Failed to write report to %s: %v", filePath, err)
		writeError(w, http.StatusInternalServerError, "failed to write report file")
		return
	}

	log.Printf("📊 Beacon diagnostic report saved: %s", filePath)

	// Return success with the report number and filename.
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"reportNumber": nextNum,
		"filename":     filename,
	})
}

// HandleListBeaconReports handles GET /api/diagnostics/beacon-reports.
//
// Lists all report-*.json files in the diagnostics directory, sorted by name.
// Returns the filenames and total count. If the directory doesn't exist yet,
// returns an empty list with count 0.
//
// Response: {"count": 3, "files": ["report-001.json", "report-002.json", "report-003.json"]}
func (h *DiagnosticsHandler) HandleListBeaconReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	files := h.listReportFiles()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(files),
		"files": files,
	})
}

// nextReportNumber scans the diagnostics directory for existing report-*.json
// files and returns max+1. If no files exist (or the directory doesn't exist),
// returns 1.
func (h *DiagnosticsHandler) nextReportNumber() int {
	files := h.listReportFiles()
	if len(files) == 0 {
		return 1
	}

	maxNum := 0
	for _, f := range files {
		// Parse number from "report-NNN.json".
		var num int
		if _, err := fmt.Sscanf(f, "report-%d.json", &num); err == nil {
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return maxNum + 1
}

// listReportFiles returns a sorted list of report-*.json filenames in the
// diagnostics directory. Returns an empty slice if the directory doesn't exist.
func (h *DiagnosticsHandler) listReportFiles() []string {
	entries, err := os.ReadDir(h.Dir)
	if err != nil {
		// Directory doesn't exist yet — that's fine, return empty.
		return []string{}
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, "report-") && strings.HasSuffix(name, ".json") {
			files = append(files, name)
		}
	}

	sort.Strings(files)
	return files
}
