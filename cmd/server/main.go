//go:generate go tool templ generate
//go:generate go tool gotailwind -i ../../assets/css/input.css -o ../../assets/css/output.css

package main

import (
	"log"
	"net/http"
	"strings"

	"spotifgo/internal/app"
	"spotifgo/internal/config"
	"spotifgo/internal/routes"

	"github.com/go-chi/chi/v5"
)

func main() {
	application := app.NewApp(config.NewConfig())

	routes.SetupRoutes(application)

	if err := application.Start(); err != nil {
		log.Fatal(err)
	}
}

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
