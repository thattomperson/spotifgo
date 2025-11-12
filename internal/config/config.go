package config

import (
	"crypto/rand"
	"os"
)

type Config struct {
	Port                string
	Host                string
	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyRedirectURL  string
	TokenSecret         string
}

func NewConfig() *Config {
	config := &Config{}
	config.Load()
	return config
}

func (c *Config) Load() {
	c.Port = os.Getenv("PORT")
	c.Host = os.Getenv("HOST")
	c.SpotifyClientID = os.Getenv("SPOTIFY_CLIENT_ID")
	c.SpotifyClientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	c.SpotifyRedirectURL = os.Getenv("SPOTIFY_REDIRECT_URL")
	c.TokenSecret = os.Getenv("TOKEN_SECRET")

	if c.TokenSecret == "" {
		c.TokenSecret = rand.Text()
	}

	if c.Port == "" {
		c.Port = "8080"
	}

	if c.Host == "" {
		c.Host = "http://localhost:" + c.Port
	}

	if c.SpotifyRedirectURL == "" {
		c.SpotifyRedirectURL = c.Host + "/auth/callback"
	}
}

func (c *Config) Sanitized() *Config {
	return &Config{
		Port: c.Port,
		Host: c.Host,
	}
}
