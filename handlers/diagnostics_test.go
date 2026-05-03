package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestDiagnosticsHandler creates a DiagnosticsHandler backed by a
// temporary directory. The directory is cleaned up after the test.
func setupTestDiagnosticsHandler(t *testing.T) *DiagnosticsHandler {
	t.Helper()
	dir := t.TempDir()
	return NewDiagnosticsHandler(dir)
}

// =============================================================================
// POST /api/diagnostics/beacon-report — Submit a beacon report
// =============================================================================

// TestDiagnostics_SubmitReport verifies that POSTing a valid JSON report
// creates a file with the correct content and returns the report number.
func TestDiagnostics_SubmitReport(t *testing.T) {
	h := setupTestDiagnosticsHandler(t)

	reportJSON := `{"session":{"id":"test-uuid","expectedRoom":"Office"},"readings":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/beacon-report", strings.NewReader(reportJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleSubmitBeaconReport(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response contains report number and filename.
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["reportNumber"] != float64(1) {
		t.Errorf("expected reportNumber 1, got %v", resp["reportNumber"])
	}
	if resp["filename"] != "report-001.json" {
		t.Errorf("expected filename 'report-001.json', got %v", resp["filename"])
	}

	// Verify the file was written to disk.
	filePath := filepath.Join(h.Dir, "report-001.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	// Verify the file content is valid JSON and contains our data.
	var savedReport map[string]interface{}
	if err := json.Unmarshal(data, &savedReport); err != nil {
		t.Fatalf("saved report is not valid JSON: %v", err)
	}

	session, ok := savedReport["session"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'session' key in saved report")
	}
	if session["id"] != "test-uuid" {
		t.Errorf("expected session.id 'test-uuid', got %v", session["id"])
	}
}

// TestDiagnostics_SequentialNumbering verifies that multiple reports get
// sequential numbers (001, 002, 003).
func TestDiagnostics_SequentialNumbering(t *testing.T) {
	h := setupTestDiagnosticsHandler(t)

	// Submit 3 reports.
	for i := 1; i <= 3; i++ {
		reportJSON := `{"test": true}`
		req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/beacon-report", strings.NewReader(reportJSON))
		w := httptest.NewRecorder()
		h.HandleSubmitBeaconReport(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("report %d: expected 201, got %d", i, w.Code)
		}
	}

	// Verify 3 files exist with correct names.
	files := h.listReportFiles()
	if len(files) != 3 {
		t.Fatalf("expected 3 report files, got %d", len(files))
	}

	expectedFiles := []string{"report-001.json", "report-002.json", "report-003.json"}
	for i, expected := range expectedFiles {
		if files[i] != expected {
			t.Errorf("expected file[%d] = '%s', got '%s'", i, expected, files[i])
		}
	}
}

// TestDiagnostics_InvalidJSON verifies that submitting invalid JSON returns 400.
func TestDiagnostics_InvalidJSON(t *testing.T) {
	h := setupTestDiagnosticsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/beacon-report", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	h.HandleSubmitBeaconReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDiagnostics_DirectoryAutoCreation verifies that the diagnostics directory
// is created automatically on the first report submission.
func TestDiagnostics_DirectoryAutoCreation(t *testing.T) {
	// Use a subdirectory that doesn't exist yet.
	dir := filepath.Join(t.TempDir(), "nested", "diagnostics")
	h := NewDiagnosticsHandler(dir)

	// Directory should not exist before the first report.
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("expected directory to not exist before first report")
	}

	reportJSON := `{"session": {"id": "auto-create-test"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/beacon-report", strings.NewReader(reportJSON))
	w := httptest.NewRecorder()

	h.HandleSubmitBeaconReport(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Directory should now exist.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected directory to be auto-created after first report")
	}
}

// TestDiagnostics_PrettyPrinted verifies that saved report files are
// pretty-printed (indented) for human readability.
func TestDiagnostics_PrettyPrinted(t *testing.T) {
	h := setupTestDiagnosticsHandler(t)

	reportJSON := `{"key":"value","nested":{"a":1}}`
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/beacon-report", strings.NewReader(reportJSON))
	w := httptest.NewRecorder()

	h.HandleSubmitBeaconReport(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	// Read the saved file and check for indentation.
	data, err := os.ReadFile(filepath.Join(h.Dir, "report-001.json"))
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	content := string(data)
	// Pretty-printed JSON should have newlines and indentation.
	if !strings.Contains(content, "\n") {
		t.Error("expected pretty-printed JSON with newlines")
	}
	if !strings.Contains(content, "  ") {
		t.Error("expected pretty-printed JSON with indentation")
	}
}

// =============================================================================
// GET /api/diagnostics/beacon-reports — List existing reports
// =============================================================================

// TestDiagnostics_ListReports verifies that GET returns the correct count and
// filenames for existing reports.
func TestDiagnostics_ListReports(t *testing.T) {
	h := setupTestDiagnosticsHandler(t)

	// Submit 2 reports first.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/beacon-report", strings.NewReader(`{"i": true}`))
		w := httptest.NewRecorder()
		h.HandleSubmitBeaconReport(w, req)
	}

	// Now list them.
	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics/beacon-reports", nil)
	w := httptest.NewRecorder()

	h.HandleListBeaconReports(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["count"] != float64(2) {
		t.Errorf("expected count 2, got %v", resp["count"])
	}

	files, ok := resp["files"].([]interface{})
	if !ok {
		t.Fatal("expected 'files' to be an array")
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

// TestDiagnostics_ListReportsEmpty verifies that GET returns count 0 and empty
// files array when no reports exist.
func TestDiagnostics_ListReportsEmpty(t *testing.T) {
	h := setupTestDiagnosticsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics/beacon-reports", nil)
	w := httptest.NewRecorder()

	h.HandleListBeaconReports(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["count"] != float64(0) {
		t.Errorf("expected count 0, got %v", resp["count"])
	}
}

// TestDiagnostics_ListReportsNonExistentDir verifies that GET returns empty
// results gracefully when the diagnostics directory hasn't been created yet.
func TestDiagnostics_ListReportsNonExistentDir(t *testing.T) {
	h := NewDiagnosticsHandler(filepath.Join(t.TempDir(), "does-not-exist"))

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics/beacon-reports", nil)
	w := httptest.NewRecorder()

	h.HandleListBeaconReports(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["count"] != float64(0) {
		t.Errorf("expected count 0 for non-existent directory, got %v", resp["count"])
	}
}
