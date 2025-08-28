package utils

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"golang.org/x/oauth2"
)

type authOptions struct {
	redirectUrl string
}

type AuthOption func(*authOptions)

func WithRedirectUrl(redirectURL string) AuthOption {
	return func(options *authOptions) {
		options.redirectUrl = redirectURL
	}
}

func AuthMiddleware(ja *jwtauth.JWTAuth, opts ...AuthOption) func(http.Handler) http.Handler {
	options := &authOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			token, _, err := jwtauth.FromContext(r.Context())

			if err != nil {
				log.Printf("Getting token from context failed: %v\n", err)
				http.Redirect(w, r, options.redirectUrl, http.StatusTemporaryRedirect)
				return
			}

			if token == nil {
				log.Println("No token found in request context")
				http.Redirect(w, r, options.redirectUrl, http.StatusTemporaryRedirect)
				return
			}

			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		})
	}
}

func OAuthTokenFromInterface(rawToken map[string]interface{}) *oauth2.Token {
	token := &oauth2.Token{
		AccessToken:  "",
		TokenType:    "",
		RefreshToken: "",
	}
	if accessToken, ok := rawToken["access_token"].(string); ok {
		token.AccessToken = accessToken
	}
	if tokenType, ok := rawToken["token_type"].(string); ok {
		token.TokenType = tokenType
	}
	if refreshToken, ok := rawToken["refresh_token"].(string); ok {
		token.RefreshToken = refreshToken
	}
	if expiry, ok := rawToken["expiry"].(string); ok {
		// Try to parse expiry as RFC3339
		if t, err := time.Parse(time.RFC3339, expiry); err == nil {
			token.Expiry = t
		}
	}

	return token
}
