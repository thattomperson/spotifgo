//go:generate go tool templ generate
//go:generate go tool gotailwind -i ./assets/css/input.css -o ./assets/css/output.css

package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"spotifgo/internal/auth"
	"spotifgo/internal/handler"
	"spotifgo/utils"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

var (
	authService *auth.Auth
)

//go:embed assets/*
var assets embed.FS

func init() {
	tokenSecret := os.Getenv("TOKEN_SECRET")
	if tokenSecret == "" {
		// default token secret for development
		tokenSecret = "secret"
	}

	tokenAuth := auth.NewTokenAuth(tokenSecret)

	host := os.Getenv("HOST")
	if host == "" {
		host = "http://localhost:" + os.Getenv("PORT")
	}
	redirectURL := fmt.Sprintf("%s/auth/callback", host)
	// the redirect URL must be an exact match of a URL you've registered for your application
	// scopes determine which permissions the user is prompted to authorize
	authenticator := auth.NewAuthenticator(redirectURL, os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))

	authService = auth.NewAuth(authenticator, tokenAuth)
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Get("/auth/login", authService.LoginHandler)
		r.Get("/auth/callback", authService.CallbackHandler)
	})

	r.Group(func(r chi.Router) {
		r.Use(authService.VerifierMiddleware())
		r.Use(authService.AuthMiddleware(auth.WithRedirectUrl("/auth/login")))

		r.Get("/", templ.Handler(hello(handler.SpotigoSignals{})).ServeHTTP)

		rpcHandlers := handler.NewRpcHandlers(authService)

		r.Post("/rpc/get-playing-song", utils.Star(rpcHandlers.GetPlayingSong))
		r.Post("/rpc/queue-track", utils.Star(rpcHandlers.QueueTrack))
		r.Post("/rpc/add-to-playlist", utils.Star(rpcHandlers.AddToPlaylist))
		r.Post("/rpc/update-selected-song", utils.Star(rpcHandlers.UpdateSelectedSong))
		r.Post("/rpc/get-top-songs", utils.Star(rpcHandlers.GetTopSongs))
		r.Post("/rpc/get-detailed-track-info", utils.Star(rpcHandlers.GetDetailedTrackInfo))

		r.Get("/auth/logout", authService.LogoutHandler)
	})

	FileServer(r, "/assets", http.FS(must(fs.Sub(assets, "assets"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

func must[T any](res T, err error) T {
	if err != nil {
		panic(err)
	}
	return res
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
