package main

import (
	"log"
	"net/http"
	"spotifgo/components/toast"
	trackcard "spotifgo/components/track-card"
	"spotifgo/utils"
	"sync"

	datastar "github.com/starfederation/datastar-go/datastar"
	"github.com/zmb3/spotify/v2"
)

func GetPlayingSong(sse *datastar.ServerSentEventGenerator, signals *TemplCounterSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)
	wg := sync.WaitGroup{}

	wg.Go(func() {
		song, err := spotifyClient.PlayerCurrentlyPlaying(r.Context())
		if err != nil {
			log.Printf("Failed to get currently playing song: %v", err)
			return
		}

		if song.Item == nil {
			sse.PatchElementTempl(trackcard.TrackCard(trackcard.Props{
				ID:    "playing-song",
				Track: spotify.SimpleTrack{Name: "No song playing"},
			}), datastar.WithSelectorID("playing-song"))

			return
		}

		if signals.SelectedSong == "" {
			signals.SelectedSong = song.Item.ID.String()
		}

		track := song.Item.SimpleTrack
		track.Album = song.Item.Album

		sse.PatchElementTempl(trackcard.TrackCard(trackcard.Props{
			ID:    "playing-song",
			Track: track,
		}), datastar.WithSelectorID("playing-song"))
	})
	wg.Go(func() {
		songs, err := spotifyClient.PlayerRecentlyPlayed(r.Context())
		if err != nil {
			log.Printf("Failed to get recently played songs: %v", err)
			return
		}

		sse.PatchElementTempl(trackcard.List(trackcard.ListProps{
			ID: "recent-songs",
			Tracks: utils.MapSlice(songs, func(item spotify.RecentlyPlayedItem) spotify.SimpleTrack {
				return item.Track
			}),
		}), datastar.WithSelectorID("recent-songs"))
	})

	wg.Wait()
	sse.MarshalAndPatchSignals(signals)
}

func QueueTrack(sse *datastar.ServerSentEventGenerator, signals *TemplCounterSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)

	track, err := spotifyClient.GetTrack(r.Context(), spotify.ID(r.FormValue("track_id")))
	if err != nil {
		log.Printf("Failed to get track: %v", err)
		return
	}

	err = spotifyClient.QueueSong(r.Context(), track.ID)
	if err != nil {
		log.Printf("Failed to queue song: %v", err)
		return
	}

	sse.PatchElementTempl(toast.Toast(toast.Props{
		Title:       "Queued " + track.Name,
		Description: "You can now enjoy this song in your queue.",
	}), datastar.WithSelectorID("toasts"), datastar.WithModeAppend())
}

func UpdateSelectedSong(sse *datastar.ServerSentEventGenerator, signals *TemplCounterSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)

	song, _ := spotifyClient.GetTrack(r.Context(), spotify.ID(signals.SelectedSong))
	track := song.SimpleTrack
	track.Album = song.Album

	sse.PatchElementTempl(trackcard.TrackCard(trackcard.Props{
		ID:    "selected-song",
		Track: track,
	}), datastar.WithSelectorID("selected-song"))

	tracks, _ := spotifyClient.GetRecommendations(r.Context(), spotify.Seeds{
		Tracks: []spotify.ID{spotify.ID(signals.SelectedSong)},
	}, nil)

	sse.PatchElementTempl(trackcard.List(trackcard.ListProps{
		ID:     "recommended-songs",
		Tracks: tracks.Tracks,
	}), datastar.WithSelectorID("recommended-songs"))
}

func GetTopSongs(sse *datastar.ServerSentEventGenerator, signals *TemplCounterSignals, r *http.Request) {
	spotifyClient := getSpotifyClient(r)

	songs, err := spotifyClient.CurrentUsersTopTracks(r.Context())
	if err != nil {
		log.Printf("Failed to get top songs: %v", err)
		return
	}

	sse.PatchElementTempl(trackcard.List(trackcard.ListProps{
		ID: "top-songs",
		Tracks: utils.MapSlice(songs.Tracks, func(item spotify.FullTrack) spotify.SimpleTrack {
			track := item.SimpleTrack
			track.Album = item.Album
			return track
		}),
	}), datastar.WithSelectorID("top-songs"))
}
