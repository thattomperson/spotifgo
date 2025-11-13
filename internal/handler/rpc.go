package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thattomperson/spotifgo/internal/auth"
	"github.com/thattomperson/spotifgo/internal/ui/components/dialog"
	"github.com/thattomperson/spotifgo/internal/ui/components/toast"
	trackcard "github.com/thattomperson/spotifgo/internal/ui/components/track-card"
	"github.com/thattomperson/spotifgo/internal/utils"
	"github.com/thattomperson/spotifgo/internal/utils/star"

	"github.com/davecgh/go-spew/spew"
	"github.com/zmb3/spotify/v2"
)

type RecommendedSongsSignal struct {
	RecommendedSongs *[]spotify.ID `json:"recommended_songs"`
}

type RecentSongsSignal struct {
	RecentSongs *[]spotify.ID `json:"recent_songs"`
}

type SpotigoSignals struct {
	CurrentTab string `json:"current_tab"`
	RecommendedSongsSignal
	RecentSongsSignal
	SelectedSong string `json:"selected_song"`
	DialogType   string `json:"dialog_type"`
	DialogItemID string `json:"dialog_item_id"`
	DialogOpen   bool   `json:"dialog_open"`
}

type RpcHandlers struct {
	authService *auth.Auth
}

func NewRpcHandlers(authService *auth.Auth) *RpcHandlers {
	return &RpcHandlers{
		authService: authService,
	}
}

func (h *RpcHandlers) GetPlayingSong(w *star.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
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

type QueueTrackSignal struct {
	RecentSongsSignal
	RecommendedSongsSignal
}

func (h *RpcHandlers) QueueTrack(w *star.DatastarWriter[QueueTrackSignal], signals *QueueTrackSignal, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)

	spew.Dump(signals)

	// // Get track IDs from both single track_id and multiple track_ids[]
	var trackIDs []spotify.ID
	if singleID := r.FormValue("track_id"); singleID != "" {
		trackIDs = []spotify.ID{spotify.ID(singleID)}
	} else if signals.RecommendedSongs != nil {
		trackIDs = *signals.RecommendedSongs
	} else if signals.RecentSongs != nil {
		trackIDs = *signals.RecentSongs
	}

	if len(trackIDs) == 0 {
		w.Append("#toasts", toast.Toast(toast.Props{
			Title:       "No tracks specified",
			Description: "Please select tracks to queue.",
		}))
		return
	}

	var successCount, failCount int
	var successNames []string

	for _, trackID := range trackIDs {
		track, err := spotifyClient.GetTrack(r.Context(), spotify.ID(trackID))
		if err != nil {
			log.Printf("Failed to get track %s: %v", trackID, err)
			failCount++
			continue
		}

		err = spotifyClient.QueueSong(r.Context(), track.ID)
		if err != nil {
			log.Printf("Failed to queue song %s: %v", track.Name, err)
			failCount++
			continue
		}

		w.Append("#toasts", toast.Toast(toast.Props{
			Title:       "Queued " + track.Name,
			Description: "You can now enjoy this song in your queue.",
		}))

		successCount++
		successNames = append(successNames, track.Name)
	}

	// Show appropriate success/failure message
	if successCount > 0 && failCount == 0 {
		if successCount == 1 {
			w.ShowToast("Queued "+successNames[0], "You can now enjoy this song in your queue.")
		} else {
			w.ShowToast(fmt.Sprintf("Queued %d songs", successCount), "All songs have been added to your queue.")
		}
	} else if successCount > 0 && failCount > 0 {
		w.ShowToast(fmt.Sprintf("Queued %d/%d songs", successCount, successCount+failCount), fmt.Sprintf("%d songs failed to queue", failCount))
	} else {
		w.ShowToast("Failed to queue songs", "All tracks failed to be added to queue.")
	}
}

func (h *RpcHandlers) AddToPlaylist(w *star.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := h.authService.GetSpotifyClient(r)

	// Get track IDs from both single track_id and multiple track_ids[]
	var trackIDs []string
	if singleID := r.FormValue("track_id"); singleID != "" {
		trackIDs = []string{singleID}
	} else {
		trackIDs = r.Form["track_ids[]"]
	}

	if len(trackIDs) == 0 {
		w.ShowToast("No tracks specified", "Please select tracks to add to playlist.")
		return
	}

	// Get current playback context to find the playlist being played
	currentlyPlaying, err := spotifyClient.PlayerCurrentlyPlaying(r.Context())
	if err != nil {
		w.Generator.Redirect("/auth/login")
		log.Printf("Failed to get currently playing: %v", err)
		return
	}

	var targetPlaylistID spotify.ID
	var targetPlaylistName string

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
			w.ShowToast("No playlists found", "Please create a playlist first to add songs to it.")
			return
		}

		// Use the first playlist as fallback
		playlist := playlists.Playlists[0]
		targetPlaylistID = playlist.ID
		targetPlaylistName = playlist.Name
	} else {
		// Extract playlist ID from the URI (format: spotify:playlist:PLAYLIST_ID)
		playlistURI := string(currentlyPlaying.PlaybackContext.URI)
		// Split by ":" to get ["spotify", "playlist", "PLAYLIST_ID"]
		parts := strings.Split(playlistURI, ":")
		if len(parts) != 3 || parts[0] != "spotify" || parts[1] != "playlist" {
			w.ShowToast("Invalid playlist context", "Unable to determine current playlist.")
			return
		}

		targetPlaylistID = spotify.ID(parts[2])

		// Get playlist details for the toast message
		playlist, err := spotifyClient.GetPlaylist(r.Context(), targetPlaylistID)
		if err != nil {
			w.Generator.Redirect("/auth/login")
			log.Printf("Failed to get playlist details: %v", err)
			return
		}
		targetPlaylistName = playlist.Name
	}

	// Process each track
	var successCount, failCount int
	var successNames []string
	var spotifyTrackIDs []spotify.ID

	// First, validate and collect all track IDs
	for _, trackID := range trackIDs {
		track, err := spotifyClient.GetTrack(r.Context(), spotify.ID(trackID))
		if err != nil {
			log.Printf("Failed to get track %s: %v", trackID, err)
			failCount++
			continue
		}
		spotifyTrackIDs = append(spotifyTrackIDs, track.ID)
		successNames = append(successNames, track.Name)
	}

	// Add tracks to playlist in batch if we have valid tracks
	if len(spotifyTrackIDs) > 0 {
		_, err = spotifyClient.AddTracksToPlaylist(r.Context(), targetPlaylistID, spotifyTrackIDs...)
		if err != nil {
			log.Printf("Failed to add tracks to playlist: %v", err)
			// If batch add fails, count all as failures
			failCount += len(spotifyTrackIDs)
			successCount = 0
		} else {
			successCount = len(spotifyTrackIDs)
		}
	}

	// Show appropriate success/failure message
	if successCount > 0 && failCount == 0 {
		if successCount == 1 {
			w.ShowToast("Added "+successNames[0]+" to "+targetPlaylistName, "The song has been added to your playlist.")
		} else {
			w.ShowToast(fmt.Sprintf("Added %d songs to %s", successCount, targetPlaylistName), "All songs have been added to your playlist.")
		}
	} else if successCount > 0 && failCount > 0 {
		w.ShowToast(fmt.Sprintf("Added %d/%d songs to %s", successCount, successCount+failCount, targetPlaylistName), fmt.Sprintf("%d songs failed to add", failCount))
	} else {
		w.ShowToast("Failed to add songs to playlist", "All tracks failed to be added to playlist.")
	}
}

func (h *RpcHandlers) UpdateSelectedSong(w *star.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
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

func (h *RpcHandlers) GetTopSongs(w *star.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
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

func (h *RpcHandlers) GetDetailedTrackInfo(w *star.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
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
