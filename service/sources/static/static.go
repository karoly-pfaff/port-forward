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
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			http.ServeFile(w, r, candidate)
			return
		}
	}

	http.ServeFile(w, r, filepath.Join(dir, "index.html"))
}
