package handler

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopUpdateFeed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "latest.yml"), []byte("version: 1.0.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meperstudio-desktop-1.0.0-windows-x64.exe"), []byte("0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private.txt"), []byte("hidden"), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewDesktopUpdateHandler(dir)
	for _, tc := range []struct {
		method, path, rangeHeader string
		status                    int
		body                      string
	}{
		{"GET", "latest.yml", "", 200, "version: 1.0.0\n"},
		{"HEAD", "latest.yml", "", 200, ""},
		{"GET", "meperstudio-desktop-1.0.0-windows-x64.exe", "bytes=2-4", 206, "234"},
		{"GET", "private.txt", "", 404, "404 page not found\n"},
		{"GET", "", "", 404, "404 page not found\n"},
		{"GET", "../latest.yml", "", 404, "404 page not found\n"},
	} {
		req := httptest.NewRequest(tc.method, "/updates/desktop/"+tc.path, nil)
		if tc.rangeHeader != "" {
			req.Header.Set("Range", tc.rangeHeader)
		}
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != tc.status || w.Body.String() != tc.body {
			t.Fatalf("%s %s status=%d body=%q", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
