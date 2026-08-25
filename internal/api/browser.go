package api

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// BrowserHandler serves the local UI and protects its same-origin API boundary.
func BrowserHandler(api http.Handler, assets fs.FS, expectedHost string) (http.Handler, error) {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, err
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		// 1. Reject requests that did not arrive through the configured loopback origin.
		if request.Host != expectedHost {
			http.Error(response, "invalid local Host", http.StatusForbidden)
			return
		}
		if origin := request.Header.Get("Origin"); origin != "" && origin != "http://"+expectedHost {
			http.Error(response, "invalid local Origin", http.StatusForbidden)
			return
		}
		response.Header().Set("Content-Security-Policy", fmt.Sprintf("default-src 'self'; connect-src 'self' ws://%s; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'", expectedHost))
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")

		// 2. Route domain calls to the same authoritative service as the Unix API.
		if strings.HasPrefix(request.URL.Path, "/v1/") {
			api.ServeHTTP(response, request)
			return
		}
		if request.URL.Path == "/" {
			index, err := fs.ReadFile(dist, "index.html")
			if err != nil {
				http.Error(response, "Web UI index is unavailable", http.StatusInternalServerError)
				return
			}
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = response.Write(index)
			return
		}
		if contentType := mime.TypeByExtension(filepath.Ext(request.URL.Path)); contentType != "" {
			response.Header().Set("Content-Type", contentType)
		}
		files.ServeHTTP(response, request)
	}), nil
}
