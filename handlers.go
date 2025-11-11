package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"spotifgo/components/dialog"
	"spotifgo/components/toast"
	trackcard "spotifgo/components/track-card"
	"spotifgo/utils"

	"github.com/zmb3/spotify/v2"
)

func GetPlayingSong(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)
	wg := sync.WaitGroup{}

	wg.Go(func() {
		song, err := spotifyClient.PlayerCurrentlyPlaying(r.Context())
		if err != nil {
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

func QueueTrack(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)

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

func UpdateSelectedSong(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)

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

func GetTopSongs(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)

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

func GetDetailedTrackInfo(w *utils.DatastarWriter[SpotigoSignals], signals *SpotigoSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)
	trackID := r.FormValue("track_id")

	if trackID == "" {
		log.Printf("No track ID provided")
		return
	}

	wg := sync.WaitGroup{}
	var track *spotify.FullTrack
	var artist *spotify.FullArtist

	// Get detailed track info
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		track, err = spotifyClient.GetTrack(r.Context(), spotify.ID(trackID))
		if err != nil {
			log.Printf("Failed to get track details: %v", err)
		}
	}()

	wg.Wait()

	if track == nil {
		log.Printf("Failed to get track information")
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
