package auth

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"spotifgo/utils"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

var scopes = []string{
	spotifyauth.ScopeUserReadPrivate,
	spotifyauth.ScopeUserReadCurrentlyPlaying,
	spotifyauth.ScopeUserReadPlaybackState,
	spotifyauth.ScopeUserModifyPlaybackState,
	spotifyauth.ScopeUserReadRecentlyPlayed,
	spotifyauth.ScopeUserTopRead,
	spotifyauth.ScopePlaylistModifyPublic,
	spotifyauth.ScopePlaylistModifyPrivate,
	spotifyauth.ScopePlaylistReadPrivate,
}

type Auth struct {
	auth      *spotifyauth.Authenticator
	tokenAuth *jwtauth.JWTAuth
}

func NewAuthenticator(redirectURL string, clientID string, clientSecret string) *spotifyauth.Authenticator {
	return spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURL),
		spotifyauth.WithScopes(scopes...),
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithClientSecret(clientSecret),
	)
}

func NewTokenAuth(secret string) *jwtauth.JWTAuth {
	return jwtauth.New("HS256", []byte(secret), nil)
}

func NewAuth(auth *spotifyauth.Authenticator, tokenAuth *jwtauth.JWTAuth) *Auth {
	return &Auth{
		auth:      auth,
		tokenAuth: tokenAuth,
	}
}

func (a *Auth) AuthURL(state string) string {
	return a.auth.AuthURL(state)
}

func (a *Auth) VerifierMiddleware() func(http.Handler) http.Handler {
	return jwtauth.Verifier(a.tokenAuth)
}

type authMiddlewareOptions struct {
	redirectUrl string
}

type AuthMiddlewareOption func(*authMiddlewareOptions)

func WithRedirectUrl(redirectURL string) AuthMiddlewareOption {
	return func(options *authMiddlewareOptions) {
		options.redirectUrl = redirectURL
	}
}

func (a *Auth) AuthMiddleware(opts ...AuthMiddlewareOption) func(http.Handler) http.Handler {
	options := &authMiddlewareOptions{
		redirectUrl: "/",
	}
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

func (a *Auth) GetSpotifyClient(r *http.Request) *spotify.Client {
	_, claims, err := jwtauth.FromContext(r.Context())
	if err != nil {
		return nil
	}

	spotifyToken, ok := claims["token"].(map[string]interface{})
	if !ok {
		return nil
	}
	token := utils.OAuthTokenFromInterface(spotifyToken)
	return spotify.New(a.auth.Client(r.Context(), token))
}

func (a *Auth) CallbackHandler(w http.ResponseWriter, r *http.Request) {
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

	url := a.auth.AuthURL(state)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (a *Auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
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

	token, err := a.auth.Token(r.Context(), state, r)
	if err != nil {
		log.Println(err)
		http.Error(w, "Couldn't get token", http.StatusNotFound)
		return
	}
	_, tokenString, _ := a.tokenAuth.Encode(map[string]interface{}{"token": token})
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (a *Auth) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
