package static

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func HasClient(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !info.IsDir()
}

func ServeClient(w http.ResponseWriter, r *http.Request, dir string) {
	cleanPath := path.Clean("/" + r.URL.Path)
	relative := strings.TrimPrefix(cleanPath, "/")
	if relative != "" && relative != "." {
		candidate := filepath.Join(dir, filepath.FromSlash(relative))
		// Defence-in-depth: only serve a candidate that stays inside dir. path.Clean
		// already collapses any "..", but verify containment explicitly so the
		// guarantee does not rely on that (or on http.ServeFile's own ".." check).
		if within(dir, candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				http.ServeFile(w, r, candidate)
				return
			}
		}
	}

	http.ServeFile(w, r, filepath.Join(dir, "index.html"))
}

// within reports whether candidate resolves inside dir (no traversal escape).
func within(dir, candidate string) bool {
	rel, err := filepath.Rel(dir, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
