package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDocumentMetadata_JSONSerialization(t *testing.T) {
	metadata := DocumentMetadata{
		Title:        "Test Document",
		Category:     "guideline",
		Path:         "docs/guidelines/test.md",
		LastModified: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Size:         1024,
		Checksum:     "abc123def456",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled DocumentMetadata
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	// Verify all fields
	if unmarshaled.Title != metadata.Title {
		t.Errorf("expected title %q, got %q", metadata.Title, unmarshaled.Title)
	}
	if unmarshaled.Category != metadata.Category {
		t.Errorf("expected category %q, got %q", metadata.Category, unmarshaled.Category)
	}
	if unmarshaled.Path != metadata.Path {
		t.Errorf("expected path %q, got %q", metadata.Path, unmarshaled.Path)
	}
	if !unmarshaled.LastModified.Equal(metadata.LastModified) {
		t.Errorf("expected lastModified %v, got %v", metadata.LastModified, unmarshaled.LastModified)
	}
	if unmarshaled.Size != metadata.Size {
		t.Errorf("expected size %d, got %d", metadata.Size, unmarshaled.Size)
	}
	if unmarshaled.Checksum != metadata.Checksum {
		t.Errorf("expected checksum %q, got %q", metadata.Checksum, unmarshaled.Checksum)
	}
}

func TestDocumentSection_JSONSerialization(t *testing.T) {
	section := DocumentSection{
		Heading: "Main Section",
		Level:   1,
		Content: "This is the main section content.",
		Subsections: []DocumentSection{
			{
				Heading: "Subsection 1",
				Level:   2,
				Content: "This is subsection content.",
			},
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(section)
	if err != nil {
		t.Fatalf("failed to marshal section: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled DocumentSection
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal section: %v", err)
	}

	// Verify all fields
	if unmarshaled.Heading != section.Heading {
		t.Errorf("expected heading %q, got %q", section.Heading, unmarshaled.Heading)
	}
	if unmarshaled.Level != section.Level {
		t.Errorf("expected level %d, got %d", section.Level, unmarshaled.Level)
	}
	if unmarshaled.Content != section.Content {
		t.Errorf("expected content %q, got %q", section.Content, unmarshaled.Content)
	}
	if len(unmarshaled.Subsections) != len(section.Subsections) {
		t.Errorf("expected %d subsections, got %d", len(section.Subsections), len(unmarshaled.Subsections))
	}
	if len(unmarshaled.Subsections) > 0 {
		if unmarshaled.Subsections[0].Heading != section.Subsections[0].Heading {
			t.Errorf("expected subsection heading %q, got %q", section.Subsections[0].Heading, unmarshaled.Subsections[0].Heading)
		}
	}
}

func TestDocumentContent_JSONSerialization(t *testing.T) {
	content := DocumentContent{
		Sections: []DocumentSection{
			{
				Heading: "Introduction",
				Level:   1,
				Content: "This is the introduction.",
			},
			{
				Heading: "Details",
				Level:   1,
				Content: "This section contains details.",
			},
		},
		RawContent: "# Introduction\n\nThis is the introduction.\n\n# Details\n\nThis section contains details.",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal content: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled DocumentContent
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal content: %v", err)
	}

	// Verify all fields
	if len(unmarshaled.Sections) != len(content.Sections) {
		t.Errorf("expected %d sections, got %d", len(content.Sections), len(unmarshaled.Sections))
	}
	if unmarshaled.RawContent != content.RawContent {
		t.Errorf("expected rawContent %q, got %q", content.RawContent, unmarshaled.RawContent)
	}
}

func TestDocument_JSONSerialization(t *testing.T) {
	doc := Document{
		Metadata: DocumentMetadata{
			Title:        "Complete Document",
			Category:     "pattern",
			Path:         "docs/patterns/test.md",
			LastModified: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Size:         2048,
			Checksum:     "def456ghi789",
		},
		Content: DocumentContent{
			Sections: []DocumentSection{
				{
					Heading: "Overview",
					Level:   1,
					Content: "This is an overview.",
				},
			},
			RawContent: "# Overview\n\nThis is an overview.",
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("failed to marshal document: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Document
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal document: %v", err)
	}

	// Verify metadata
	if unmarshaled.Metadata.Title != doc.Metadata.Title {
		t.Errorf("expected title %q, got %q", doc.Metadata.Title, unmarshaled.Metadata.Title)
	}
	if unmarshaled.Metadata.Category != doc.Metadata.Category {
		t.Errorf("expected category %q, got %q", doc.Metadata.Category, unmarshaled.Metadata.Category)
	}

	// Verify content
	if len(unmarshaled.Content.Sections) != len(doc.Content.Sections) {
		t.Errorf("expected %d sections, got %d", len(doc.Content.Sections), len(unmarshaled.Content.Sections))
	}
	if unmarshaled.Content.RawContent != doc.Content.RawContent {
		t.Errorf("expected rawContent %q, got %q", doc.Content.RawContent, unmarshaled.Content.RawContent)
	}
}

func TestADRDocument_JSONSerialization(t *testing.T) {
	adr := ADRDocument{
		DocumentMetadata: DocumentMetadata{
			Title:        "ADR-001: Use Microservices",
			Category:     "adr",
			Path:         "docs/adr/001-microservices.md",
			LastModified: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Size:         4096,
			Checksum:     "ghi789jkl012",
		},
		ADRId:          "001",
		Status:         "Accepted",
		Date:           time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		TechnicalStory: "We need to adopt microservices architecture for better scalability.",
		Deciders: []Decider{
			{
				FullName: "John Doe",
				Role:     "Tech Lead",
				RACI:     "Accountable",
			},
			{
				FullName: "Jane Smith",
				Role:     "Senior Developer",
				RACI:     "Responsible",
			},
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(adr)
	if err != nil {
		t.Fatalf("failed to marshal ADR: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled ADRDocument
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal ADR: %v", err)
	}

	// Verify ADR-specific fields
	if unmarshaled.ADRId != adr.ADRId {
		t.Errorf("expected ADRId %q, got %q", adr.ADRId, unmarshaled.ADRId)
	}
	if unmarshaled.Status != adr.Status {
		t.Errorf("expected status %q, got %q", adr.Status, unmarshaled.Status)
	}
	if !unmarshaled.Date.Equal(adr.Date) {
		t.Errorf("expected date %v, got %v", adr.Date, unmarshaled.Date)
	}
	if unmarshaled.TechnicalStory != adr.TechnicalStory {
		t.Errorf("expected technicalStory %q, got %q", adr.TechnicalStory, unmarshaled.TechnicalStory)
	}

	// Verify deciders
	if len(unmarshaled.Deciders) != len(adr.Deciders) {
		t.Errorf("expected %d deciders, got %d", len(adr.Deciders), len(unmarshaled.Deciders))
	}
	if len(unmarshaled.Deciders) > 0 {
		if unmarshaled.Deciders[0].FullName != adr.Deciders[0].FullName {
			t.Errorf("expected decider name %q, got %q", adr.Deciders[0].FullName, unmarshaled.Deciders[0].FullName)
		}
		if unmarshaled.Deciders[0].Role != adr.Deciders[0].Role {
			t.Errorf("expected decider role %q, got %q", adr.Deciders[0].Role, unmarshaled.Deciders[0].Role)
		}
		if unmarshaled.Deciders[0].RACI != adr.Deciders[0].RACI {
			t.Errorf("expected decider RACI %q, got %q", adr.Deciders[0].RACI, unmarshaled.Deciders[0].RACI)
		}
	}

	// Verify embedded metadata
	if unmarshaled.Title != adr.Title {
		t.Errorf("expected title %q, got %q", adr.Title, unmarshaled.Title)
	}
	if unmarshaled.Category != adr.Category {
		t.Errorf("expected category %q, got %q", adr.Category, unmarshaled.Category)
	}
}

func TestDecider_JSONSerialization(t *testing.T) {
	decider := Decider{
		FullName: "Alice Johnson",
		Role:     "Product Manager",
		RACI:     "Consulted",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(decider)
	if err != nil {
		t.Fatalf("failed to marshal decider: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Decider
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal decider: %v", err)
	}

	// Verify all fields
	if unmarshaled.FullName != decider.FullName {
		t.Errorf("expected fullName %q, got %q", decider.FullName, unmarshaled.FullName)
	}
	if unmarshaled.Role != decider.Role {
		t.Errorf("expected role %q, got %q", decider.Role, unmarshaled.Role)
	}
	if unmarshaled.RACI != decider.RACI {
		t.Errorf("expected RACI %q, got %q", decider.RACI, unmarshaled.RACI)
	}
}

func TestDocumentIndex_JSONSerialization(t *testing.T) {
	index := DocumentIndex{
		Category: "guidelines",
		Documents: []DocumentMetadata{
			{
				Title:        "API Design",
				Category:     "guideline",
				Path:         "docs/guidelines/api-design.md",
				LastModified: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Size:         1024,
				Checksum:     "abc123",
			},
			{
				Title:        "Security Guidelines",
				Category:     "guideline",
				Path:         "docs/guidelines/security.md",
				LastModified: time.Date(2024, 1, 16, 11, 0, 0, 0, time.UTC),
				Size:         2048,
				Checksum:     "def456",
			},
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("failed to marshal index: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled DocumentIndex
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal index: %v", err)
	}

	// Verify fields
	if unmarshaled.Category != index.Category {
		t.Errorf("expected category %q, got %q", index.Category, unmarshaled.Category)
	}
	if len(unmarshaled.Documents) != len(index.Documents) {
		t.Errorf("expected %d documents, got %d", len(index.Documents), len(unmarshaled.Documents))
	}
	if len(unmarshaled.Documents) > 0 {
		if unmarshaled.Documents[0].Title != index.Documents[0].Title {
			t.Errorf("expected document title %q, got %q", index.Documents[0].Title, unmarshaled.Documents[0].Title)
		}
	}
}

func TestFileEvent_JSONSerialization(t *testing.T) {
	event := FileEvent{
		Type:     "modify",
		Path:     "docs/guidelines/api-design.md",
		IsDir:    false,
		Checksum: "abc123def456",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled FileEvent
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	// Verify all fields
	if unmarshaled.Type != event.Type {
		t.Errorf("expected type %q, got %q", event.Type, unmarshaled.Type)
	}
	if unmarshaled.Path != event.Path {
		t.Errorf("expected path %q, got %q", event.Path, unmarshaled.Path)
	}
	if unmarshaled.IsDir != event.IsDir {
		t.Errorf("expected isDir %v, got %v", event.IsDir, unmarshaled.IsDir)
	}
	if unmarshaled.Checksum != event.Checksum {
		t.Errorf("expected checksum %q, got %q", event.Checksum, unmarshaled.Checksum)
	}
}
