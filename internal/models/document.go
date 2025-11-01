package models

import (
	"time"
)

// DocumentMetadata represents metadata for a documentation file
type DocumentMetadata struct {
	Title        string    `json:"title"`
	Category     string    `json:"category"` // "guideline", "pattern", "adr"
	Path         string    `json:"path"`
	LastModified time.Time `json:"lastModified"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
}

// DocumentContent represents the parsed content of a documentation file
type DocumentContent struct {
	Sections   []DocumentSection `json:"sections"`
	RawContent string            `json:"rawContent"`
}

// DocumentSection represents a section within a document
type DocumentSection struct {
	Heading     string            `json:"heading"`
	Level       int               `json:"level"`
	Content     string            `json:"content"`
	Subsections []DocumentSection `json:"subsections,omitempty"`
}

// Document represents a complete documentation file with metadata and content
type Document struct {
	Metadata DocumentMetadata `json:"metadata"`
	Content  DocumentContent  `json:"content"`
}

// DocumentIndex represents an index of documents by category
type DocumentIndex struct {
	Category  string             `json:"category"`
	Documents []DocumentMetadata `json:"documents"`
}

// ADRDocument represents an Architecture Decision Record with specific fields
type ADRDocument struct {
	DocumentMetadata
	ADRId          string    `json:"adrId"`
	Status         string    `json:"status"` // "Pending", "Deciding", "Accepted", "Superseded"
	Date           time.Time `json:"date"`
	Deciders       []Decider `json:"deciders"`
	TechnicalStory string    `json:"technicalStory"`
}

// Decider represents a person involved in an ADR decision
type Decider struct {
	FullName string `json:"fullName"`
	Role     string `json:"role"`
	RACI     string `json:"raci"` // "Accountable", "Responsible", "Consulted", "Informed"
}

// FileEvent represents a file system event
type FileEvent struct {
	Type     string `json:"type"` // "create", "modify", "delete"
	Path     string `json:"path"`
	IsDir    bool   `json:"isDir"`
	Checksum string `json:"checksum,omitempty"`
}
