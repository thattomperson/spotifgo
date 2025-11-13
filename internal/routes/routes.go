package routes

import (
	"net/http"
	"os"

	"github.com/thattomperson/spotifgo/internal/app"
	"github.com/thattomperson/spotifgo/internal/auth"
	"github.com/thattomperson/spotifgo/internal/handler"
	"github.com/thattomperson/spotifgo/internal/ui/pages"
	"github.com/thattomperson/spotifgo/internal/utils"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func SetupRoutes(app *app.App) {
	authenticator := auth.NewAuthenticator(app.Config.SpotifyRedirectURL, app.Config.SpotifyClientID, app.Config.SpotifyClientSecret)
	tokenAuth := auth.NewTokenAuth(app.Config.TokenSecret)
	authService := auth.NewAuth(authenticator, tokenAuth)

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

		r.Get("/", templ.Handler(pages.HomePage(handler.SpotigoSignals{})).ServeHTTP)

		rpcHandlers := handler.NewRpcHandlers(authService)

		r.Post("/rpc/get-playing-song", utils.Star(rpcHandlers.GetPlayingSong))
		r.Post("/rpc/queue-track", utils.Star(rpcHandlers.QueueTrack))
		r.Post("/rpc/add-to-playlist", utils.Star(rpcHandlers.AddToPlaylist))
		r.Post("/rpc/update-selected-song", utils.Star(rpcHandlers.UpdateSelectedSong))
		r.Post("/rpc/get-top-songs", utils.Star(rpcHandlers.GetTopSongs))
		r.Post("/rpc/get-detailed-track-info", utils.Star(rpcHandlers.GetDetailedTrackInfo))

		r.Get("/auth/logout", authService.LogoutHandler)
	})

	r.Get("/assets/*", http.StripPrefix("/assets", http.FileServer(http.FS(os.DirFS("assets")))).ServeHTTP)

	app.Router.Mount("/", r)
}
