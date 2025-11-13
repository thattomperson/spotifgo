//go:generate go tool templ generate
//go:generate go tool gotailwind --input assets/css/input.css --output assets/css/output.css --cwd ../..

package main

import (
	"log"

	"github.com/thattomperson/spotifgo/internal/app"
	"github.com/thattomperson/spotifgo/internal/config"
	"github.com/thattomperson/spotifgo/internal/routes"
)

func main() {
	application := app.NewApp(config.NewConfig())

	routes.SetupRoutes(application)

	if err := application.Start(); err != nil {
		log.Fatal(err)
	}
}
