package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"onlyoffice-fnos/internal/config"
)

// SettingsResponse represents the settings API response
type SettingsResponse struct {
	DocumentServerURL    string `json:"documentServerUrl"`
	DocumentServerSecret string `json:"documentServerSecret"`
	BaseURL              string `json:"baseUrl"`
}

// SaveSettingsRequest represents the request to save settings
type SaveSettingsRequest struct {
	DocumentServerURL    string `json:"documentServerUrl"`
	DocumentServerSecret string `json:"documentServerSecret"`
	BaseURL              string `json:"baseUrl"`
}

// handleGetSettings handles GET /api/settings
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.settingsStore.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			// Return empty settings if config doesn't exist
			s.respondJSON(w, http.StatusOK, &SettingsResponse{})
			return
		}
		s.respondError(w, http.StatusInternalServerError, "Failed to load settings")
		return
	}

	s.respondJSON(w, http.StatusOK, &SettingsResponse{
		DocumentServerURL:    settings.DocumentServerURL,
		DocumentServerSecret: settings.DocumentServerSecret,
		BaseURL:              settings.BaseURL,
	})
}

// handleSaveSettings handles POST /api/settings
func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		s.respondHTMXOrJSON(w, r, false, "表单数据无效")
		return
	}

	serverURL := strings.TrimSuffix(r.FormValue("documentServerUrl"), "/")
	secret := r.FormValue("documentServerSecret")
	baseURL := strings.TrimSuffix(r.FormValue("baseUrl"), "/")

	// Validate URL
	if serverURL == "" {
		s.respondHTMXOrJSON(w, r, false, "Document Server 地址不能为空")
		return
	}

	if baseURL == "" {
		s.respondHTMXOrJSON(w, r, false, "本机回调地址不能为空")
		return
	}

	settings := &config.Settings{
		DocumentServerURL:    serverURL,
		DocumentServerSecret: secret,
		BaseURL:              baseURL,
	}

	if err := s.settingsStore.Save(settings); err != nil {
		s.respondHTMXOrJSON(w, r, false, "保存设置失败")
		return
	}

	// Update server's baseURL
	s.baseURL = baseURL

	s.respondHTMXOrJSON(w, r, true, "设置已保存")
}

// respondHTMXOrJSON responds with HTML for htmx requests or JSON otherwise
func (s *Server) respondHTMXOrJSON(w http.ResponseWriter, r *http.Request, success bool, message string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		class := "error"
		if success {
			class = "success"
		}
		fmt.Fprintf(w, `<div class="message %s">%s</div>`, class, message)
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": success,
		"message": message,
	})
}

// handleGenerateKey handles POST /api/settings/generate-key
func (s *Server) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	newSecret := s.jwtManager.GenerateSecret()

	// For htmx requests, return an input element with the new secret
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<input type="text" id="secret" name="documentServerSecret" class="input-field" value="%s">`, newSecret)
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"secret": newSecret,
	})
}

// handleValidateConnection handles POST /api/settings/validate
func (s *Server) handleValidateConnection(w http.ResponseWriter, r *http.Request) {
	var serverURL string

	// Support both JSON and form data
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var req struct {
			DocumentServerURL string `json:"documentServerUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.respondError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
		serverURL = req.DocumentServerURL
	} else {
		// Form data
		if err := r.ParseForm(); err != nil {
			s.respondError(w, http.StatusBadRequest, "Invalid form data")
			return
		}
		serverURL = r.FormValue("documentServerUrl")
	}

	if serverURL == "" {
		// Try to load from settings
		settings, err := s.settingsStore.Load()
		if err != nil || settings.DocumentServerURL == "" {
			s.respondError(w, http.StatusBadRequest, "Document Server URL is required")
			return
		}
		serverURL = settings.DocumentServerURL
	}

	// Normalize URL
	serverURL = strings.TrimSuffix(serverURL, "/")

	// Try to connect to the Document Server
	valid, err := s.validateDocumentServer(serverURL)

	// For htmx requests, return HTML status
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if valid {
			w.Write([]byte(`<div class="message success">连接成功！</div>`))
		} else {
			errMsg := "连接失败"
			if err != nil {
				errMsg = fmt.Sprintf("连接失败: %s", err.Error())
			}
			fmt.Fprintf(w, `<div class="message error">%s</div>`, errMsg)
		}
		return
	}

	if valid {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   true,
			"message": "Connection successful",
		})
	} else {
		errMsg := "Connection failed"
		if err != nil {
			errMsg = err.Error()
		}
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": errMsg,
		})
	}
}

// validateDocumentServer checks if the Document Server is accessible
func (s *Server) validateDocumentServer(serverURL string) (bool, error) {
	// Try to access the Document Server's healthcheck or API endpoint
	// OnlyOffice Document Server typically has /healthcheck endpoint
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try healthcheck endpoint first
	healthURL := serverURL + "/healthcheck"
	resp, err := client.Get(healthURL)
	if err != nil {
		// Try the web-apps endpoint as fallback
		webAppsURL := serverURL + "/web-apps/apps/api/documents/api.js"
		resp, err = client.Get(webAppsURL)
		if err != nil {
			return false, fmt.Errorf("cannot connect to server: %v", err)
		}
	}
	defer resp.Body.Close()

	// Read response body (limited to prevent memory issues)
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024*1024))

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, nil
	}

	return false, fmt.Errorf("server returned status %d", resp.StatusCode)
}
