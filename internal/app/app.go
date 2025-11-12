package app

import (
	"net/http"
	"spotifgo/internal/config"

	"github.com/go-chi/chi"
)

type App struct {
	Config *config.Config
	Router *chi.Mux
}

func NewApp(config *config.Config) *App {
	return &App{
		Config: config,
		Router: chi.NewRouter(),
	}
}

func (a *App) Start() error {
	return http.ListenAndServe(":"+a.Config.Port, a.Router)
}
