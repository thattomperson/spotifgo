package utils

import (
	"log"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
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
