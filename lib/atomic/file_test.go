package atomic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		data    []byte
		perm    os.FileMode
		wantErr bool
	}{
		{
			name:    "successful write and rename",
			path:    filepath.Join(dir, "out.json"),
			data:    []byte(`{"key":"value"}`),
			perm:    0644,
			wantErr: false,
		},
		{
			name:    "restrictive permissions",
			path:    filepath.Join(dir, "secret.json"),
			data:    []byte("token-data"),
			perm:    0600,
			wantErr: false,
		},
		{
			name:    "empty content",
			path:    filepath.Join(dir, "empty.txt"),
			data:    []byte{},
			perm:    0644,
			wantErr: false,
		},
		{
			name:    "invalid directory returns error",
			path:    filepath.Join("/nonexistent/deep/path", "file.txt"),
			data:    []byte("nope"),
			perm:    0644,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteFile(tt.path, tt.data, tt.perm)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				// Verify temp file is cleaned up
				_, statErr := os.Stat(tt.path + ".tmp")
				if !os.IsNotExist(statErr) {
					t.Fatal("temp file should be cleaned up on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify content matches
			got, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("reading output file: %v", err)
			}
			if string(got) != string(tt.data) {
				t.Fatalf("content mismatch: got %q, want %q", got, tt.data)
			}

			// Verify permission bits
			info, err := os.Stat(tt.path)
			if err != nil {
				t.Fatalf("stat output file: %v", err)
			}
			if got := info.Mode().Perm(); got != tt.perm {
				t.Fatalf("permission mismatch: got %o, want %o", got, tt.perm)
			}

			// Verify no temp file remains
			_, statErr := os.Stat(tt.path + ".tmp")
			if !os.IsNotExist(statErr) {
				t.Fatal("temp file should not exist after successful write")
			}
		})
	}
}
