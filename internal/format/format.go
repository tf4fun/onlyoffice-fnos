package format

import (
	"errors"
	"strings"
)

var (
	ErrFormatNotSupported = errors.New("format not supported")
)

// Format represents a file format with its properties
type Format struct {
	Extension     string `json:"extension"`
	Type          string `json:"type"` // word, cell, slide
	Editable      bool   `json:"editable"`
	ViewOnly      bool   `json:"viewOnly"`
	Convertible   bool   `json:"convertible"`
	ConvertTarget string `json:"convertTarget"`
}

// Manager handles file format operations
type Manager struct {
	formats map[string]*Format
}

// NewManager creates a new FormatManager with predefined formats
func NewManager() *Manager {
	m := &Manager{
		formats: make(map[string]*Format),
	}
	m.initFormats()
	return m
}

// initFormats initializes the format mapping table
func (m *Manager) initFormats() {
	// Editable formats (OOXML)
	m.formats["docx"] = &Format{Extension: "docx", Type: "word", Editable: true}
	m.formats["xlsx"] = &Format{Extension: "xlsx", Type: "cell", Editable: true}
	m.formats["pptx"] = &Format{Extension: "pptx", Type: "slide", Editable: true}

	// Convertible formats - Word
	m.formats["doc"] = &Format{Extension: "doc", Type: "word", Convertible: true, ConvertTarget: "docx"}
	m.formats["odt"] = &Format{Extension: "odt", Type: "word", Convertible: true, ConvertTarget: "docx"}
	m.formats["rtf"] = &Format{Extension: "rtf", Type: "word", Convertible: true, ConvertTarget: "docx"}
	m.formats["txt"] = &Format{Extension: "txt", Type: "word", Convertible: true, ConvertTarget: "docx"}

	// Convertible formats - Cell
	m.formats["xls"] = &Format{Extension: "xls", Type: "cell", Convertible: true, ConvertTarget: "xlsx"}
	m.formats["ods"] = &Format{Extension: "ods", Type: "cell", Convertible: true, ConvertTarget: "xlsx"}
	m.formats["csv"] = &Format{Extension: "csv", Type: "cell", Convertible: true, ConvertTarget: "xlsx"}

	// Convertible formats - Slide
	m.formats["ppt"] = &Format{Extension: "ppt", Type: "slide", Convertible: true, ConvertTarget: "pptx"}
	m.formats["odp"] = &Format{Extension: "odp", Type: "slide", Convertible: true, ConvertTarget: "pptx"}

	// View-only formats
	m.formats["pdf"] = &Format{Extension: "pdf", Type: "word", ViewOnly: true}
	m.formats["djvu"] = &Format{Extension: "djvu", Type: "word", ViewOnly: true}
	m.formats["oxps"] = &Format{Extension: "oxps", Type: "word", ViewOnly: true}
	m.formats["epub"] = &Format{Extension: "epub", Type: "word", ViewOnly: true}
	m.formats["fb2"] = &Format{Extension: "fb2", Type: "word", ViewOnly: true}
}

// GetFormat returns the format information for a given extension
func (m *Manager) GetFormat(extension string) (*Format, bool) {
	ext := strings.ToLower(strings.TrimPrefix(extension, "."))
	f, ok := m.formats[ext]
	return f, ok
}

// IsEditable returns true if the format can be directly edited
func (m *Manager) IsEditable(extension string) bool {
	f, ok := m.GetFormat(extension)
	if !ok {
		return false
	}
	return f.Editable
}

// IsConvertible returns true if the format can be converted to OOXML
func (m *Manager) IsConvertible(extension string) bool {
	f, ok := m.GetFormat(extension)
	if !ok {
		return false
	}
	return f.Convertible
}

// IsViewOnly returns true if the format can only be viewed
func (m *Manager) IsViewOnly(extension string) bool {
	f, ok := m.GetFormat(extension)
	if !ok {
		return false
	}
	return f.ViewOnly
}

// GetConvertTarget returns the target OOXML format for conversion
func (m *Manager) GetConvertTarget(extension string) string {
	f, ok := m.GetFormat(extension)
	if !ok || !f.Convertible {
		return ""
	}
	return f.ConvertTarget
}

// GetDocumentType returns the document type (word, cell, slide) for a given extension
func (m *Manager) GetDocumentType(extension string) string {
	f, ok := m.GetFormat(extension)
	if !ok {
		return ""
	}
	return f.Type
}

// GetAllConvertibleFormats returns all formats that can be converted
func (m *Manager) GetAllConvertibleFormats() []*Format {
	var result []*Format
	for _, f := range m.formats {
		if f.Convertible {
			result = append(result, f)
		}
	}
	return result
}
