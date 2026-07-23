package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ---------------------------------------------------------------------------
// VirusTotal integration (v3 API)
// ---------------------------------------------------------------------------

// VTReport holds the result of a VirusTotal scan.
type VTReport struct {
	Detections int    `json:"detections"`
	Total      int    `json:"total"`
	ReportURL  string `json:"report_url"`
	Scanned    bool   `json:"scanned"`
}

// VirusTotalScanner uploads files to VirusTotal and polls for analysis results
// using the v3 API. The API key is passed via the X-Apikey header.
type VirusTotalScanner struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewVirusTotalScanner creates a new scanner with the given API key.
func NewVirusTotalScanner(apiKey string) *VirusTotalScanner {
	return &VirusTotalScanner{
		apiKey:  apiKey,
		baseURL: "https://www.virustotal.com/api/v3",
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// ScanFile uploads a file to VirusTotal, polls until the analysis completes,
// and returns the report. The SHA256 of the file is computed locally for the
// report lookup endpoint.
func (vts *VirusTotalScanner) ScanFile(filePath string) (*VTReport, error) {
	// Compute SHA256 of the file for report lookup.
	hash, err := hashFileServer(filePath)
	if err != nil {
		return nil, fmt.Errorf("hash file: %w", err)
	}

	// Upload the file to VT.
	if err := vts.uploadFile(filePath); err != nil {
		return nil, fmt.Errorf("upload to VT: %w", err)
	}

	// Poll for the analysis report.
	report, err := vts.pollReport(hash)
	if err != nil {
		return nil, fmt.Errorf("poll VT report: %w", err)
	}
	return report, nil
}

// uploadFile uploads a file to VirusTotal via the v3 files endpoint using
// multipart form data.
func (vts *VirusTotalScanner) uploadFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequest("POST", vts.baseURL+"/files", &body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Apikey", vts.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := vts.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("VT upload failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// pollReport polls the VT v3 files endpoint until the analysis completes.
// Checks every 15 seconds for up to 60 attempts (15 minutes max).
func (vts *VirusTotalScanner) pollReport(sha256 string) (*VTReport, error) {
	maxAttempts := 60
	for i := 0; i < maxAttempts; i++ {
		time.Sleep(15 * time.Second)

		report, done, err := vts.getReport(sha256)
		if err != nil {
			return nil, err
		}
		if done {
			return report, nil
		}
	}
	return nil, fmt.Errorf("VT analysis timed out after %d attempts", maxAttempts)
}

// getReport fetches the VT report for a file hash. Returns (report, true, nil)
// when the analysis is complete, (nil, false, nil) when still pending.
func (vts *VirusTotalScanner) getReport(sha256 string) (*VTReport, bool, error) {
	req, err := http.NewRequest("GET", vts.baseURL+"/files/"+sha256, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("X-Apikey", vts.apiKey)

	resp, err := vts.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Analysis not yet available.
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("VT report failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			Attributes struct {
				Status            string `json:"status"`
				LastAnalysisStats struct {
					Malicious  int `json:"malicious"`
					Suspicious int `json:"suspicious"`
					Harmless   int `json:"harmless"`
					Undetected int `json:"undetected"`
				} `json:"last_analysis_stats"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("decode VT response: %w", err)
	}

	if result.Data.Attributes.Status != "completed" {
		return nil, false, nil
	}

	stats := result.Data.Attributes.LastAnalysisStats
	detections := stats.Malicious + stats.Suspicious
	total := stats.Malicious + stats.Suspicious + stats.Harmless + stats.Undetected

	return &VTReport{
		Detections: detections,
		Total:      total,
		ReportURL:  "https://www.virustotal.com/gui/file/" + sha256,
		Scanned:    true,
	}, true, nil
}