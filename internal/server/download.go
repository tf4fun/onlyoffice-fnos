package server

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"

	"onlyoffice-fnos/internal/file"
)

// handleDownload handles GET /download
// This endpoint provides file access for the OnlyOffice Document Server
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Get file path from query parameter
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		s.respondError(w, http.StatusBadRequest, "File path is required")
		return
	}

	// Get file info
	fileInfo, err := s.fileService.GetFileInfo(filePath)
	if err != nil {
		log.Printf("Error getting file info for %s: %v", filePath, err)
		switch err {
		case file.ErrFileNotFound:
			s.respondError(w, http.StatusNotFound, "File not found")
		case file.ErrInvalidPath:
			s.respondError(w, http.StatusBadRequest, "Invalid file path")
		case file.ErrPermissionDenied:
			s.respondError(w, http.StatusForbidden, "Permission denied")
		default:
			s.respondError(w, http.StatusInternalServerError, "Failed to get file info")
		}
		return
	}

	// Get file content
	content, err := s.fileService.GetFileContent(filePath)
	if err != nil {
		log.Printf("Error getting file content for %s: %v", filePath, err)
		switch err {
		case file.ErrFileNotFound:
			s.respondError(w, http.StatusNotFound, "File not found")
		case file.ErrPermissionDenied:
			s.respondError(w, http.StatusForbidden, "Permission denied")
		default:
			s.respondError(w, http.StatusInternalServerError, "Failed to read file")
		}
		return
	}
	defer content.Close()

	// Set content type based on file extension
	contentType := getContentType(fileInfo.Extension)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileInfo.Name+"\"")
	w.Header().Set("Content-Length", formatInt64(fileInfo.Size))

	// Stream the file content
	if _, err := io.Copy(w, content); err != nil {
		log.Printf("Error streaming file %s: %v", filePath, err)
		// Can't send error response at this point as headers are already sent
	}
}

// getContentType returns the MIME type for a file extension
func getContentType(ext string) string {
	// Map common Office extensions to their MIME types
	mimeTypes := map[string]string{
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"doc":  "application/msword",
		"xls":  "application/vnd.ms-excel",
		"ppt":  "application/vnd.ms-powerpoint",
		"odt":  "application/vnd.oasis.opendocument.text",
		"ods":  "application/vnd.oasis.opendocument.spreadsheet",
		"odp":  "application/vnd.oasis.opendocument.presentation",
		"pdf":  "application/pdf",
		"rtf":  "application/rtf",
		"txt":  "text/plain",
		"csv":  "text/csv",
	}

	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}

	// Try to get MIME type from system
	mimeType := mime.TypeByExtension("." + ext)
	if mimeType != "" {
		return mimeType
	}

	return "application/octet-stream"
}

// formatInt64 converts int64 to string
func formatInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}
