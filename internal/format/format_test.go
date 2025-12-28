package format

import (
	"testing"

	"pgregory.net/rapid"
)

// Property 4: 格式转换映射正确性
// *For any* 可转换格式，GetConvertTarget 应返回正确的 OOXML 格式
// **Validates: Requirements 3.4**
func TestProperty4_FormatConversionMapping(t *testing.T) {
	// Expected conversion mappings based on requirements
	expectedMappings := map[string]string{
		"doc": "docx",
		"xls": "xlsx",
		"ppt": "pptx",
		"odt": "docx",
		"ods": "xlsx",
		"odp": "pptx",
		"rtf": "docx",
		"txt": "docx",
		"csv": "xlsx",
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random convertible format from the expected mappings
		extensions := make([]string, 0, len(expectedMappings))
		for ext := range expectedMappings {
			extensions = append(extensions, ext)
		}

		idx := rapid.IntRange(0, len(extensions)-1).Draw(t, "index")
		ext := extensions[idx]
		expectedTarget := expectedMappings[ext]

		m := NewManager()

		// Verify the format is convertible
		if !m.IsConvertible(ext) {
			t.Fatalf("format %s should be convertible", ext)
		}

		// Verify the conversion target is correct
		actualTarget := m.GetConvertTarget(ext)
		if actualTarget != expectedTarget {
			t.Fatalf("format %s should convert to %s, got %s", ext, expectedTarget, actualTarget)
		}

		// Verify the target format is editable (OOXML)
		if !m.IsEditable(actualTarget) {
			t.Fatalf("target format %s should be editable", actualTarget)
		}
	})
}

// Unit test: Verify all editable formats
func TestEditableFormats(t *testing.T) {
	m := NewManager()
	editableFormats := []string{"docx", "xlsx", "pptx"}

	for _, ext := range editableFormats {
		if !m.IsEditable(ext) {
			t.Errorf("format %s should be editable", ext)
		}
	}
}

// Unit test: Verify view-only formats
func TestViewOnlyFormats(t *testing.T) {
	m := NewManager()
	viewOnlyFormats := []string{"pdf", "djvu", "oxps", "epub", "fb2"}

	for _, ext := range viewOnlyFormats {
		if !m.IsViewOnly(ext) {
			t.Errorf("format %s should be view-only", ext)
		}
	}
}

// Unit test: Verify document types
func TestDocumentTypes(t *testing.T) {
	m := NewManager()

	tests := []struct {
		ext      string
		expected string
	}{
		{"docx", "word"},
		{"doc", "word"},
		{"xlsx", "cell"},
		{"xls", "cell"},
		{"pptx", "slide"},
		{"ppt", "slide"},
	}

	for _, tt := range tests {
		actual := m.GetDocumentType(tt.ext)
		if actual != tt.expected {
			t.Errorf("format %s should have type %s, got %s", tt.ext, tt.expected, actual)
		}
	}
}

// Unit test: Verify extension normalization (case insensitive, with/without dot)
func TestExtensionNormalization(t *testing.T) {
	m := NewManager()

	tests := []string{"DOCX", "Docx", ".docx", ".DOCX"}

	for _, ext := range tests {
		if !m.IsEditable(ext) {
			t.Errorf("format %s should be recognized as editable", ext)
		}
	}
}
