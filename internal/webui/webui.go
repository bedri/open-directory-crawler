package webui

import (
	"embed"
	"io"
	"net/http"
)

//go:embed index.html
var staticFiles embed.FS

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		f, err := staticFiles.Open("index.html")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer f.Close()
		io.Copy(w, f)
	})
}
