package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"spotifgo/internal/auth"
	"spotifgo/internal/ui/components/dialog"
	"spotifgo/internal/ui/components/toast"
	trackcard "spotifgo/internal/ui/components/track-card"
	"spotifgo/internal/utils"

	"github.com/davecgh/go-spew/spew"
	"github.com/zmb3/spotify/v2"
)

type SpotigoSignals struct {
	SelectedSong string `json:"selected_song"`
	DialogOpen   bool   `json:"dialog_open"`
	DialogType   string `json:"dialog_type"`
	DialogItemID string `json:"dialog_item_id"`
}

type RpcHandlers struct {
	authService *auth.Auth
}

func NewRpcHandlers(authService *auth.Auth) *RpcHandlers {
	return &RpcHandlers{
		authService: authService,
	}
}

func (h *RpcHandlers) GetPlayingSong(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)
	wg := sync.WaitGroup{}

	wg.Go(func() {
		song, err := spotifyClient.PlayerCurrentlyPlaying(r.Context())
		if err != nil {
			spew.Dump(err)
			w.Generator.Redirect("/auth/login")
			log.Printf("Failed to get currently playing song: %v", err)
			return
		}

		if song.Item == nil {
			w.ReplaceInner("#playing-song", trackcard.TrackCardEmpty())
			return
		}

		if signals.SelectedSong == "" {
			signals.SelectedSong = song.Item.ID.String()
		}

		track := song.Item.SimpleTrack
		track.Album = song.Item.Album

		w.ReplaceInner("#playing-song", trackcard.TrackCard(trackcard.Props{
			Track: track,
		}))
	})
	wg.Go(func() {
		songs, err := spotifyClient.PlayerRecentlyPlayed(r.Context())
		if err != nil {
			w.Generator.Redirect("/auth/login")
			log.Printf("Failed to get recently played songs: %v", err)
			return
		}

		w.Replace("#recent-songs", trackcard.List(trackcard.ListProps{
			ID: "recent-songs",
			Tracks: utils.MapSlice(songs, func(item spotify.RecentlyPlayedItem) spotify.SimpleTrack {
				return item.Track
			}),
		}))
	})

	wg.Wait()
	w.UpdateSignals(signals)
}

func (h *RpcHandlers) QueueTrack(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)

	track, err := spotifyClient.GetTrack(r.Context(), spotify.ID(r.FormValue("track_id")))
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to get track: %v", err)
		return
	}

	err = spotifyClient.QueueSong(r.Context(), track.ID)
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to queue song: %v", err)
		return
	}

	w.Append("#toasts", toast.Toast(toast.Props{
		Title:       "Queued " + track.Name,
		Description: "You can now enjoy this song in your queue.",
	}))
}

func (h *RpcHandlers) AddToPlaylist(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)

	track, err := spotifyClient.GetTrack(r.Context(), spotify.ID(r.FormValue("track_id")))
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to get track: %v", err)
		return
	}

	// Get current playback context to find the playlist being played
	currentlyPlaying, err := spotifyClient.PlayerCurrentlyPlaying(r.Context())
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to get currently playing: %v", err)
		return
	}

	// Check if the user is currently playing from a playlist
	if currentlyPlaying == nil || currentlyPlaying.PlaybackContext.Type != "playlist" {
		// Fallback to first user playlist if not playing from a playlist
		playlists, err := spotifyClient.CurrentUsersPlaylists(r.Context())
		if err != nil {
			w.Generator.Redirect("/auth/login")
			log.Printf("Failed to get playlists: %v", err)
			return
		}

		if len(playlists.Playlists) == 0 {
			w.Append("#toasts", toast.Toast(toast.Props{
				Title:       "No playlists found",
				Description: "Please create a playlist first to add songs to it.",
			}))
			return
		}

		// Add to the first playlist as fallback
		playlist := playlists.Playlists[0]
		_, err = spotifyClient.AddTracksToPlaylist(r.Context(), playlist.ID, track.ID)
		if err != nil {
			w.Generator.Redirect("/auth/login")
			log.Printf("Failed to add track to playlist: %v", err)
			return
		}

		w.Append("#toasts", toast.Toast(toast.Props{
			Title:       "Added " + track.Name + " to " + playlist.Name,
			Description: "The song has been added to your playlist.",
		}))
		return
	}

	// Extract playlist ID from the URI (format: spotify:playlist:PLAYLIST_ID)
	playlistURI := string(currentlyPlaying.PlaybackContext.URI)
	// Split by ":" to get ["spotify", "playlist", "PLAYLIST_ID"]
	parts := strings.Split(playlistURI, ":")
	if len(parts) != 3 || parts[0] != "spotify" || parts[1] != "playlist" {
		w.Append("#toasts", toast.Toast(toast.Props{
			Title:       "Invalid playlist context",
			Description: "Unable to determine current playlist.",
		}))
		return
	}

	playlistID := spotify.ID(parts[2])

	// Get playlist details for the toast message
	playlist, err := spotifyClient.GetPlaylist(r.Context(), playlistID)
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to get playlist details: %v", err)
		return
	}

	// Add track to the current playlist
	_, err = spotifyClient.AddTracksToPlaylist(r.Context(), playlistID, track.ID)
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to add track to playlist: %v", err)
		return
	}

	w.Append("#toasts", toast.Toast(toast.Props{
		Title:       "Added " + track.Name + " to " + playlist.Name,
		Description: "The song has been added to your current playlist.",
	}))
}

func (h *RpcHandlers) UpdateSelectedSong(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)

	song, _ := spotifyClient.GetTrack(r.Context(), spotify.ID(signals.SelectedSong))
	track := song.SimpleTrack
	track.Album = song.Album

	w.ReplaceInner("#selected-song", trackcard.TrackCard(trackcard.Props{
		Track: track,
	}))

	tracks, err := spotifyClient.GetRecommendations(r.Context(), spotify.Seeds{
		Tracks: []spotify.ID{spotify.ID(signals.SelectedSong)},
	}, nil)
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to get recommendations: %v", err)
		return
	}

	w.Replace("#recommended-songs", trackcard.List(trackcard.ListProps{
		ID:     "recommended-songs",
		Tracks: tracks.Tracks,
	}))
}

func (h *RpcHandlers) GetTopSongs(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)

	songs, err := spotifyClient.CurrentUsersTopTracks(r.Context())
	if err != nil {
		log.Printf("Failed to get top songs: %v", err)
		return
	}

	w.ReplaceInner("#top-songs", trackcard.List(trackcard.ListProps{
		Tracks: utils.MapSlice(songs.Tracks, func(item spotify.FullTrack) spotify.SimpleTrack {
			track := item.SimpleTrack
			track.Album = item.Album
			return track
		}),
	}))
}

func (h *RpcHandlers) GetDetailedTrackInfo(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)
	trackID := r.FormValue("track_id")

	if trackID == "" {
		log.Printf("No track ID provided")
		return
	}

	wg := sync.WaitGroup{}

	var artist *spotify.FullArtist

	// Get detailed track info
	track, err := spotifyClient.GetTrack(r.Context(), spotify.ID(trackID))
	if err != nil || track == nil {
		log.Printf("Failed to get track details: %v", err)
		return
	}

	// Get artist details for genres (if not already available from track)
	if len(track.Artists) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			artist, err = spotifyClient.GetArtist(r.Context(), track.Artists[0].ID)
			if err != nil {
				log.Printf("Failed to get artist details: %v", err)
			}
		}()
		wg.Wait()
	}

	// Format duration
	duration := formatDuration(int(track.Duration))

	// Get album image URL
	var albumImage string
	if len(track.Album.Images) > 0 {
		albumImage = track.Album.Images[0].URL
	}

	// Get genres from artist
	var genres []string
	if artist != nil {
		genres = artist.Genres
	}

	// Format release date
	var releaseDate string
	if track.Album.ReleaseDate != "" {
		releaseDate = track.Album.ReleaseDate
	}

	// Create dialog props
	dialogProps := dialog.DetailedTrackProps{
		TrackID:     track.ID.String(),
		TrackName:   track.Name,
		ArtistName:  track.Artists[0].Name,
		AlbumName:   track.Album.Name,
		AlbumImage:  albumImage,
		Duration:    duration,
		Popularity:  int(track.Popularity),
		ReleaseDate: releaseDate,
		Genres:      genres,
	}

	signals.DialogOpen = true
	signals.DialogType = "track"
	signals.DialogItemID = trackID
	w.UpdateSignals(signals)
	// Update the dialog content
	w.ReplaceInner("#dialog-content", dialog.DetailedTrackInfo(dialogProps))
}

func formatDuration(duration int) string {
	d := time.Duration(duration) * time.Millisecond
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
