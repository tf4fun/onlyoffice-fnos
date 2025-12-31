package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"onlyoffice-fnos/internal/config"
	"onlyoffice-fnos/internal/file"
)

// ConvertRequest represents a conversion request
type ConvertRequest struct {
	Async      bool   `json:"async"`
	Filetype   string `json:"filetype"`
	Key        string `json:"key"`
	Outputtype string `json:"outputtype"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Token      string `json:"token,omitempty"`
}

// ConvertResponse represents the conversion API response
type ConvertResponse struct {
	EndConvert bool   `json:"endConvert"`
	FileURL    string `json:"fileUrl,omitempty"`
	Percent    int    `json:"percent"`
	Error      int    `json:"error,omitempty"`
}

// handleConvert handles POST /convert
// This endpoint executes format conversion via OnlyOffice conversion API
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	// Get file path from query parameter or form
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		if err := r.ParseForm(); err == nil {
			filePath = r.FormValue("path")
		}
	}

	if filePath == "" {
		s.respondError(w, http.StatusBadRequest, "File path is required")
		return
	}

	// Get file info
	fileInfo, err := s.fileService.GetFileInfo(filePath)
	if err != nil {
		log.Printf("Convert error: failed to get file info: %v", err)
		switch err {
		case file.ErrFileNotFound:
			s.respondError(w, http.StatusNotFound, "File not found")
		default:
			s.respondError(w, http.StatusInternalServerError, "Failed to get file info")
		}
		return
	}

	// Check if format is convertible
	if !s.formatManager.IsConvertible(fileInfo.Extension) {
		s.respondError(w, http.StatusBadRequest, "File format is not convertible")
		return
	}

	// Get target format
	targetFormat := s.formatManager.GetConvertTarget(fileInfo.Extension)
	if targetFormat == "" {
		s.respondError(w, http.StatusBadRequest, "No conversion target for this format")
		return
	}

	// Load settings
	settings, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Convert error: failed to load settings: %v", err)
		if err == config.ErrConfigNotFound {
			s.respondError(w, http.StatusBadRequest, "Document Server not configured")
		} else {
			s.respondError(w, http.StatusInternalServerError, "Failed to load settings")
		}
		return
	}

	if settings.DocumentServerURL == "" {
		s.respondError(w, http.StatusBadRequest, "Document Server URL not configured")
		return
	}

	// Build download URL for the source file
	downloadURL := s.buildDownloadURL(filePath)

	// Generate unique key for conversion
	conversionKey := fmt.Sprintf("convert_%s_%d", filePath, time.Now().UnixNano())

	// Build conversion request
	convReq := &ConvertRequest{
		Async:      false, // Synchronous conversion
		Filetype:   fileInfo.Extension,
		Key:        conversionKey,
		Outputtype: targetFormat,
		Title:      fileInfo.Name,
		URL:        downloadURL,
	}

	// Sign request with JWT if secret is configured
	if settings.DocumentServerSecret != "" {
		claims := map[string]interface{}{
			"async":      convReq.Async,
			"filetype":   convReq.Filetype,
			"key":        convReq.Key,
			"outputtype": convReq.Outputtype,
			"title":      convReq.Title,
			"url":        convReq.URL,
		}
		token, err := s.jwtManager.Sign(settings.DocumentServerSecret, claims)
		if err != nil {
			log.Printf("Convert error: failed to sign request: %v", err)
			s.respondError(w, http.StatusInternalServerError, "Failed to sign conversion request")
			return
		}
		convReq.Token = token
	}

	// Call conversion API
	convertedURL, err := s.callConversionAPI(settings.DocumentServerURL, convReq, settings.DocumentServerSecret)
	if err != nil {
		log.Printf("Convert error: conversion failed: %v", err)
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Conversion failed: %v", err))
		return
	}

	// Download converted file
	convertedContent, err := s.downloadConvertedFile(convertedURL)
	if err != nil {
		log.Printf("Convert error: failed to download converted file: %v", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to download converted file")
		return
	}
	defer convertedContent.Close()

	// Build target file path
	targetPath := s.buildTargetPath(filePath, targetFormat)

	// Save converted file
	if err := s.fileService.SaveFile(targetPath, convertedContent); err != nil {
		log.Printf("Convert error: failed to save converted file: %v", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to save converted file")
		return
	}

	log.Printf("Conversion successful: %s -> %s", filePath, targetPath)

	// For htmx requests, redirect to editor
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/editor?path="+url.QueryEscape(targetPath))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Return success with target path
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"targetPath": targetPath,
		"message":    "Conversion successful",
	})
}

// buildDownloadURL builds the download URL for a file
func (s *Server) buildDownloadURL(filePath string) string {
	baseURL := s.getEffectiveBaseURL()
	baseURL = strings.TrimSuffix(baseURL, "/")
	return fmt.Sprintf("%s/download?path=%s", baseURL, url.QueryEscape(filePath))
}

// getEffectiveBaseURL returns the effective base URL, trying settings first
func (s *Server) getEffectiveBaseURL() string {
	// First try the server's cached baseURL
	if s.baseURL != "" {
		return s.baseURL
	}
	// Try to load from settings
	if settings, err := s.settingsStore.Load(); err == nil && settings.BaseURL != "" {
		s.baseURL = settings.BaseURL
		return s.baseURL
	}
	// Fallback to localhost (should not happen if properly configured)
	return "http://localhost:10099"
}

// buildTargetPath builds the target file path for conversion
func (s *Server) buildTargetPath(sourcePath, targetFormat string) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name+"."+targetFormat)
}

// callConversionAPI calls the OnlyOffice conversion API
func (s *Server) callConversionAPI(serverURL string, req *ConvertRequest, secret string) (string, error) {
	// Build API URL
	apiURL := strings.TrimSuffix(serverURL, "/") + "/ConvertService.ashx"

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Add JWT token to header if configured
	if secret != "" && req.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	}

	// Send request
	client := &http.Client{
		Timeout: 5 * time.Minute, // Conversion can take a while
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var convResp ConvertResponse
	if err := json.Unmarshal(body, &convResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if convResp.Error != 0 {
		return "", fmt.Errorf("conversion error code: %d", convResp.Error)
	}

	if !convResp.EndConvert {
		return "", fmt.Errorf("conversion not complete (async mode not supported)")
	}

	if convResp.FileURL == "" {
		return "", fmt.Errorf("no file URL in response")
	}

	return convResp.FileURL, nil
}

// downloadConvertedFile downloads the converted file from the given URL
func (s *Server) downloadConvertedFile(fileURL string) (io.ReadCloser, error) {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
