//go:generate go tool templ generate
//go:generate go tool gotailwind -i ./assets/css/input.css -o ./assets/css/output.css

package main

import (
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"spotifgo/utils"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	datastar "github.com/starfederation/datastar-go/datastar"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

var tokenAuth *jwtauth.JWTAuth
var auth *spotifyauth.Authenticator

//go:embed assets/*
var assets embed.FS

func init() {
	tokenSecret := os.Getenv("TOKEN_SECRET")
	if tokenSecret == "" {
		// default token secret for development
		tokenSecret = "secret"
	}

	tokenAuth = jwtauth.New("HS256", []byte(tokenSecret), nil)

	host := os.Getenv("HOST")
	if host == "" {
		host = "http://localhost:3000"
	}
	redirectURL := fmt.Sprintf("%s/auth/callback", host)
	// the redirect URL must be an exact match of a URL you've registered for your application
	// scopes determine which permissions the user is prompted to authorize
	auth = spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURL),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopeUserReadPlaybackState,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadRecentlyPlayed,
			spotifyauth.ScopeUserTopRead,
		),
		spotifyauth.WithClientID(os.Getenv("SPOTIFY_CLIENT_ID")),
		spotifyauth.WithClientSecret(os.Getenv("SPOTIFY_CLIENT_SECRET")),
	)
}

func getSpotifyClient(r *http.Request) *spotify.Client {
	_, claims, err := jwtauth.FromContext(r.Context())

	if err != nil {
		return nil
	}

	spotifyToken, ok := claims["token"].(map[string]interface{})
	if !ok {
		return nil
	}
	token := utils.OAuthTokenFromInterface(spotifyToken)
	return spotify.New(auth.Client(r.Context(), token))
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Get("/auth/login", func(w http.ResponseWriter, r *http.Request) {
			// get the user to this URL - how you do that is up to you
			// you should specify a unique state string to identify the session

			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				http.Error(w, "Failed to generate state", http.StatusInternalServerError)
				return
			}
			state := base64.URLEncoding.EncodeToString(b)

			http.SetCookie(w, &http.Cookie{
				Name:     "state",
				Value:    state,
				Path:     "/",
				HttpOnly: true,
				MaxAge:   int((time.Minute * 5) / time.Second),
			})

			url := auth.AuthURL(state)

			http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		})

		r.Get("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
			stateCookie, err := r.Cookie("state")
			if err != nil {
				log.Println(err)
				http.Error(w, "Failed to get state cookie", http.StatusInternalServerError)
				return
			}
			if stateCookie == nil {
				http.Error(w, "Failed to get state cookie", http.StatusInternalServerError)
				return
			}
			state := stateCookie.Value
			http.SetCookie(w, &http.Cookie{
				Name:     "state",
				Value:    "",
				Path:     "/",
				HttpOnly: true,
				MaxAge:   -1,
			})

			token, err := auth.Token(r.Context(), state, r)
			if err != nil {
				log.Println(err)
				http.Error(w, "Couldn't get token", http.StatusNotFound)
				return
			}
			_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{"token": token})
			http.SetCookie(w, &http.Cookie{
				Name:     "jwt",
				Value:    tokenString,
				Path:     "/",
				HttpOnly: true,
			})
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(utils.AuthMiddleware(tokenAuth, utils.WithRedirectUrl("/auth/login")))

		r.Get("/", templ.Handler(hello(TemplCounterSignals{})).ServeHTTP)

		r.Post("/rpc/get-playing-song", Star(GetPlayingSong))
		r.Post("/rpc/queue-track", Star(QueueTrack))
		r.Post("/rpc/update-selected-song", Star(UpdateSelectedSong))
		r.Post("/rpc/get-top-songs", Star(GetTopSongs))
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

func Star[T any](fn func(*datastar.ServerSentEventGenerator, *T, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		var store = new(T)
		if err := datastar.ReadSignals(r, store); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sse := datastar.NewSSE(w, r)
		fn(sse, store, r)
	})
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
