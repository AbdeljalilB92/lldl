package atomic

import (
	"fmt"
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

			// Verify no temp files remain (CreateTemp uses .lldl-tmp-* pattern)
			entries, _ := os.ReadDir(filepath.Dir(tt.path))
			for _, e := range entries {
				if matched, _ := filepath.Match(".lldl-tmp-*", e.Name()); matched {
					t.Fatalf("temp file should not exist after successful write: %s", e.Name())
				}
			}
		})
	}
}

func TestWriteFile_ConcurrentWrites(t *testing.T) {
	// Verify that concurrent writes to different files in the same directory
	// don't collide because each uses a unique temp file.
	dir := t.TempDir()
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			path := filepath.Join(dir, filepath.Base(fmt.Sprintf("file%d.txt", idx)))
			errs <- WriteFile(path, []byte("content"), 0644)
		}(i)
	}

	for i := 0; i < 10; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent write %d failed: %v", i, err)
		}
	}
}
