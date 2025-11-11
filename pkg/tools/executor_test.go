package tools

import (
	"testing"
)

func TestValidateResourcePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid pattern path",
			path:    "mcp/resources/patterns/repository-pattern.md",
			wantErr: false,
		},
		{
			name:    "valid guideline path",
			path:    "mcp/resources/guidelines/api-design.md",
			wantErr: false,
		},
		{
			name:    "valid adr path",
			path:    "mcp/resources/adr/001-microservices.md",
			wantErr: false,
		},
		{
			name:    "valid base path",
			path:    "mcp/resources",
			wantErr: false,
		},
		{
			name:    "directory traversal with ..",
			path:    "mcp/resources/../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path outside resources",
			path:    "mcp/other/file.md",
			wantErr: true,
		},
		{
			name:    "absolute path outside resources",
			path:    "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "relative path outside resources",
			path:    "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path with .. in middle",
			path:    "mcp/resources/patterns/../../../etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResourcePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResourcePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
