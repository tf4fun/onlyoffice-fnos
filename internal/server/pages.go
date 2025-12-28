package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"

	"onlyoffice-fnos/internal/config"
	"onlyoffice-fnos/internal/file"
	"onlyoffice-fnos/internal/format"
	"onlyoffice-fnos/web"
)

// SettingsPageData holds data for the settings page template
type SettingsPageData struct {
	Settings *config.Settings
}

// EditorPageData holds data for the editor page template
type EditorPageData struct {
	Title             string
	ConfigJSON        template.JS
	DocumentServerURL string
	Lang              string
}

// ConvertPageData holds data for the convert page template
type ConvertPageData struct {
	FileName        string
	FilePath        string
	FilePathEncoded string
	SourceFormat    string
	TargetFormat    string
	CanDirectEdit   bool
	Error           string
}

// ErrorPageData holds data for the error page template
type ErrorPageData struct {
	Title        string
	Message      string
	ErrorCode    string
	Details      string
	RetryURL     string
	BackURL      string
	BackText     string
	ShowSettings bool
}

// templates holds parsed templates
type templates struct {
	settings *template.Template
	editor   *template.Template
	convert  *template.Template
	error    *template.Template
}

// loadTemplates loads all HTML templates from embedded filesystem
func (s *Server) loadTemplates() error {
	var err error

	s.templates = &templates{}

	s.templates.settings, err = template.ParseFS(web.Templates, "templates/settings.tmpl")
	if err != nil {
		return err
	}

	s.templates.editor, err = template.ParseFS(web.Templates, "templates/editor.tmpl")
	if err != nil {
		return err
	}

	s.templates.convert, err = template.ParseFS(web.Templates, "templates/convert.tmpl")
	if err != nil {
		return err
	}

	s.templates.error, err = template.ParseFS(web.Templates, "templates/error.tmpl")
	if err != nil {
		return err
	}

	return nil
}

// handleSettingsPage handles GET / - renders the settings page
func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	// Load current settings
	settings, err := s.settingsStore.Load()
	if err != nil && err != config.ErrConfigNotFound {
		log.Printf("Failed to load settings: %v", err)
	}
	if settings == nil {
		settings = &config.Settings{}
	}

	data := &SettingsPageData{
		Settings: settings,
	}

	// If templates are loaded, use them
	if s.templates != nil && s.templates.settings != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.settings.Execute(w, data); err != nil {
			log.Printf("Failed to render settings template: %v", err)
			s.renderErrorPage(w, &ErrorPageData{
				Title:   "渲染错误",
				Message: "无法渲染设置页面",
			})
		}
		return
	}

	// Fallback to inline HTML
	s.renderSettingsPageFallback(w, data)
}

// handleEditorPage handles GET /editor - renders the editor page
func (s *Server) handleEditorPage(w http.ResponseWriter, r *http.Request) {
	// Get file path from query parameter
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		s.renderErrorPage(w, &ErrorPageData{
			Title:   "参数错误",
			Message: "未指定文件路径",
			BackURL: "/",
		})
		return
	}

	// Get view mode
	mode := r.URL.Query().Get("mode")

	// Load settings
	settings, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Failed to load settings: %v", err)
		s.renderErrorPage(w, &ErrorPageData{
			Title:        "配置错误",
			Message:      "无法加载 Document Server 配置，请先完成设置。",
			ShowSettings: true,
		})
		return
	}

	if settings.DocumentServerURL == "" {
		s.renderErrorPage(w, &ErrorPageData{
			Title:        "配置错误",
			Message:      "Document Server 地址未配置，请先完成设置。",
			ShowSettings: true,
		})
		return
	}

	// Get file info
	fileInfo, err := s.fileService.GetFileInfo(filePath)
	if err != nil {
		log.Printf("Failed to get file info: %v", err)
		errMsg := "无法获取文件信息"
		if err == file.ErrFileNotFound {
			errMsg = "文件不存在"
		}
		s.renderErrorPage(w, &ErrorPageData{
			Title:   "文件错误",
			Message: errMsg,
			BackURL: "/",
		})
		return
	}

	// Check if format needs conversion
	if s.formatManager.IsConvertible(fileInfo.Extension) && mode != "view" {
		// Redirect to convert page
		http.Redirect(w, r, "/convert?path="+url.QueryEscape(filePath), http.StatusFound)
		return
	}

	// Get user info from query or use defaults
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "fnos_user"
	}
	userName := r.URL.Query().Get("user_name")
	if userName == "" {
		userName = "fnOS 用户"
	}

	// Get language
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "zh"
	}

	// Build editor config
	configReq := &editorConfigRequest{
		FilePath:  filePath,
		FileInfo:  fileInfo,
		UserID:    userID,
		UserName:  userName,
		Lang:      lang,
		BaseURL:   s.baseURL,
		JWTSecret: settings.DocumentServerSecret,
		ViewMode:  mode == "view",
	}

	editorConfig, err := s.buildEditorConfig(configReq)
	if err != nil {
		log.Printf("Failed to build editor config: %v", err)
		s.renderErrorPage(w, &ErrorPageData{
			Title:   "配置错误",
			Message: "无法生成编辑器配置",
			Details: err.Error(),
			BackURL: "/",
		})
		return
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(editorConfig)
	if err != nil {
		log.Printf("Failed to marshal editor config: %v", err)
		s.renderErrorPage(w, &ErrorPageData{
			Title:   "内部错误",
			Message: "无法序列化编辑器配置",
			BackURL: "/",
		})
		return
	}

	data := &EditorPageData{
		Title:             fileInfo.Name,
		ConfigJSON:        template.JS(configJSON),
		DocumentServerURL: settings.DocumentServerURL,
		Lang:              lang,
	}

	// If templates are loaded, use them
	if s.templates != nil && s.templates.editor != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.editor.Execute(w, data); err != nil {
			log.Printf("Failed to render editor template: %v", err)
			s.renderErrorPage(w, &ErrorPageData{
				Title:   "渲染错误",
				Message: "无法渲染编辑器页面",
			})
		}
		return
	}

	// Fallback to inline HTML
	s.renderEditorPageFallback(w, data)
}

// handleConvertPage handles GET /convert - renders the convert page
func (s *Server) handleConvertPage(w http.ResponseWriter, r *http.Request) {
	// Get file path from query parameter
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		s.renderErrorPage(w, &ErrorPageData{
			Title:   "参数错误",
			Message: "未指定文件路径",
			BackURL: "/",
		})
		return
	}

	// Get file info
	fileInfo, err := s.fileService.GetFileInfo(filePath)
	if err != nil {
		log.Printf("Failed to get file info: %v", err)
		errMsg := "无法获取文件信息"
		if err == file.ErrFileNotFound {
			errMsg = "文件不存在"
		}
		s.renderErrorPage(w, &ErrorPageData{
			Title:   "文件错误",
			Message: errMsg,
			BackURL: "/",
		})
		return
	}

	// Get target format
	targetFormat := s.formatManager.GetConvertTarget(fileInfo.Extension)
	if targetFormat == "" {
		// Not convertible, redirect to editor
		http.Redirect(w, r, "/editor?path="+url.QueryEscape(filePath), http.StatusFound)
		return
	}

	data := &ConvertPageData{
		FileName:        fileInfo.Name,
		FilePath:        filePath,
		FilePathEncoded: url.QueryEscape(filePath),
		SourceFormat:    fileInfo.Extension,
		TargetFormat:    targetFormat,
		CanDirectEdit:   false, // Old formats generally can't be directly edited
	}

	// If templates are loaded, use them
	if s.templates != nil && s.templates.convert != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.convert.Execute(w, data); err != nil {
			log.Printf("Failed to render convert template: %v", err)
			s.renderErrorPage(w, &ErrorPageData{
				Title:   "渲染错误",
				Message: "无法渲染转换页面",
			})
		}
		return
	}

	// Fallback to inline HTML
	s.renderConvertPageFallback(w, data)
}

// renderErrorPage renders the error page
func (s *Server) renderErrorPage(w http.ResponseWriter, data *ErrorPageData) {
	if data.Title == "" {
		data.Title = "错误"
	}

	// If templates are loaded, use them
	if s.templates != nil && s.templates.error != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.error.Execute(w, data); err != nil {
			log.Printf("Failed to render error template: %v", err)
			// Ultimate fallback
			http.Error(w, data.Message, http.StatusInternalServerError)
		}
		return
	}

	// Fallback to inline HTML
	s.renderErrorPageFallback(w, data)
}

// editorConfigRequest holds parameters for building editor config
type editorConfigRequest struct {
	FilePath  string
	FileInfo  *file.FileInfo
	UserID    string
	UserName  string
	Lang      string
	BaseURL   string
	JWTSecret string
	ViewMode  bool
}

// buildEditorConfig builds the editor configuration
func (s *Server) buildEditorConfig(req *editorConfigRequest) (map[string]interface{}, error) {
	// Get format information
	formatInfo, ok := s.formatManager.GetFormat(req.FileInfo.Extension)
	if !ok {
		return nil, format.ErrFormatNotSupported
	}

	// Determine edit mode
	canEdit := formatInfo.Editable && !req.ViewMode
	mode := "view"
	if canEdit {
		mode = "edit"
	}

	// Generate document key
	docKey := s.configBuilder.GetDocumentKey(req.FilePath, req.FileInfo.ModTime)

	// Build download URL
	downloadURL := s.buildDownloadURL(req.FilePath)

	// Build callback URL
	callbackURL := s.buildCallbackURL(req.FilePath)

	config := map[string]interface{}{
		"document": map[string]interface{}{
			"fileType": req.FileInfo.Extension,
			"key":      docKey,
			"title":    req.FileInfo.Name,
			"url":      downloadURL,
			"permissions": map[string]interface{}{
				"edit":     canEdit,
				"download": true,
				"print":    true,
			},
		},
		"documentType": formatInfo.Type,
		"editorConfig": map[string]interface{}{
			"callbackUrl": callbackURL,
			"lang":        req.Lang,
			"mode":        mode,
			"user": map[string]interface{}{
				"id":   req.UserID,
				"name": req.UserName,
			},
		},
	}

	// Sign the configuration with JWT if secret is provided
	if req.JWTSecret != "" {
		token, err := s.jwtManager.Sign(req.JWTSecret, config)
		if err != nil {
			return nil, err
		}
		config["token"] = token
	}

	return config, nil
}

// buildCallbackURL builds the callback URL for a file
func (s *Server) buildCallbackURL(filePath string) string {
	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return baseURL + "/callback?path=" + url.QueryEscape(filePath)
}

// Fallback renderers for when templates are not available

func (s *Server) renderSettingsPageFallback(w http.ResponseWriter, data *SettingsPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>OnlyOffice Connector 设置</title>
    <script src="/static/htmx.min.js"></script>
    <style>
        body { font-family: sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
        button { padding: 10px 20px; background: #4a90d9; color: white; border: none; border-radius: 4px; cursor: pointer; margin: 5px 5px 5px 0; }
        .btn-secondary { background: #f0f0f0; color: #333; border: 1px solid #ddd; }
        .message { padding: 10px; border-radius: 4px; margin-top: 10px; }
        .success { background: #d4edda; color: #155724; }
        .error { background: #f8d7da; color: #721c24; }
        .input-row { display: flex; gap: 10px; }
        .input-row input { flex: 1; }
    </style>
</head>
<body>
    <h1>OnlyOffice Connector 设置</h1>
    <form hx-post="/api/settings" hx-target="#message" hx-swap="innerHTML">
        <div class="form-group">
            <label>Document Server 地址</label>
            <div class="input-row">
                <input type="url" id="documentServerUrl" name="documentServerUrl" value="` + data.Settings.DocumentServerURL + `" placeholder="http://192.168.1.100:8080">
                <button type="button" class="btn-secondary" hx-post="/api/settings/validate" hx-include="#documentServerUrl" hx-target="#status">测试连接</button>
            </div>
            <div id="status"></div>
        </div>
        <div class="form-group">
            <label>JWT 密钥</label>
            <div class="input-row">
                <input type="text" id="secret" name="documentServerSecret" value="` + data.Settings.DocumentServerSecret + `">
                <button type="button" class="btn-secondary" hx-post="/api/settings/generate-key" hx-target="#secret" hx-swap="outerHTML">重新生成</button>
            </div>
        </div>
        <button type="submit">保存设置</button>
        <div id="message"></div>
    </form>
</body>
</html>`
	w.Write([]byte(html))
}

func (s *Server) renderEditorPageFallback(w http.ResponseWriter, data *EditorPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>` + data.Title + ` - OnlyOffice Editor</title>
    <style>
        html, body { height: 100%; margin: 0; overflow: hidden; }
        #editor-container { width: 100%; height: 100%; }
    </style>
</head>
<body>
    <div id="editor-container"></div>
    <script src="` + data.DocumentServerURL + `/web-apps/apps/api/documents/api.js"></script>
    <script>new DocsAPI.DocEditor("editor-container", ` + string(data.ConfigJSON) + `);</script>
</body>
</html>`
	w.Write([]byte(html))
}

func (s *Server) renderConvertPageFallback(w http.ResponseWriter, data *ConvertPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>格式转换</title>
    <script src="/static/htmx.min.js"></script>
    <style>
        body { font-family: sans-serif; max-width: 500px; margin: 40px auto; padding: 20px; }
        .btn { display: block; width: 100%; padding: 12px; margin: 10px 0; text-align: center; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; }
        .btn-primary { background: #4a90d9; color: white; }
        .btn-secondary { background: #f0f0f0; color: #333; }
    </style>
</head>
<body>
    <h1>格式转换</h1>
    <p>文件: ` + data.FileName + `</p>
    <p>格式: ` + data.SourceFormat + ` → ` + data.TargetFormat + `</p>
    <div id="error"></div>
    <form hx-post="/convert" hx-target="#error" hx-swap="innerHTML">
        <input type="hidden" name="path" value="` + data.FilePath + `">
        <button type="submit" class="btn btn-primary">转换为 ` + data.TargetFormat + ` 并编辑</button>
    </form>
    <a href="/editor?path=` + data.FilePathEncoded + `&mode=view" class="btn btn-secondary">以只读模式查看</a>
    <a href="/">← 返回设置</a>
</body>
</html>`
	w.Write([]byte(html))
}

func (s *Server) renderErrorPageFallback(w http.ResponseWriter, data *ErrorPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	backURL := data.BackURL
	if backURL == "" {
		backURL = "/"
	}
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>` + data.Title + `</title>
    <style>
        body { font-family: sans-serif; max-width: 500px; margin: 40px auto; padding: 20px; text-align: center; }
        .error-box { background: #f8d7da; color: #721c24; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .btn { display: inline-block; padding: 10px 20px; background: #4a90d9; color: white; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <h1>` + data.Title + `</h1>
    <div class="error-box">` + data.Message + `</div>
    <a href="` + backURL + `" class="btn">返回</a>
</body>
</html>`
	w.Write([]byte(html))
}
