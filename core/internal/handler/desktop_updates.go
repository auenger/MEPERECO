package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Desktop releases are public, static and deployed by the operator. Never
// expose directory listings, environment files, or paths outside that root.
var desktopUpdateFile = regexp.MustCompile(`^(latest(-[a-z0-9]+)*\.ya?ml|[A-Za-z0-9][A-Za-z0-9._-]*\.(exe|msi|dmg|zip|blockmap|AppImage|deb|rpm|tar\.gz))$`)

func NewDesktopUpdateHandler(directory string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/updates/desktop/")
		if directory == "" || filename == "" || filepath.Base(filename) != filename || strings.Contains(filename, "\\") || !desktopUpdateFile.MatchString(filename) {
			http.NotFound(w, r)
			return
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer root.Close()
		file, err := root.Open(filename)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(filename, ".yml") || strings.HasSuffix(filename, ".yaml") {
			w.Header().Set("Content-Type", "application/yaml")
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		// ServeContent honors HEAD, ranges, conditional requests and 206 responses.
		http.ServeContent(w, r, filename, info.ModTime(), file)
	}
}
