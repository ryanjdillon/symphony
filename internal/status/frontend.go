package status

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var frontendFS embed.FS

// FrontendHandler returns an http.Handler that serves the embedded frontend.
// Falls back to index.html for SPA routing.
func FrontendHandler() http.Handler {
	sub, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("failed to create frontend sub-filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the file directly
		f, err := sub.Open(r.URL.Path[1:]) // strip leading /
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for unmatched routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
