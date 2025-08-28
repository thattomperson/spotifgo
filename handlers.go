package main

import (
	"net/http"
	"spotifgo/components/toast"
	trackcard "spotifgo/components/track-card"

	datastar "github.com/starfederation/datastar-go/datastar"
	"github.com/zmb3/spotify/v2"
)

func GetPlayingSong(w http.ResponseWriter, r *http.Request) {
	spotifyClient := getSpotifyClient(r)
	store := getStore[TemplCounterSignals](w, r)
	sse := datastar.NewSSE(w, r)

	song, err := spotifyClient.PlayerCurrentlyPlaying(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if store.SelectedSong == "" {
		store.SelectedSong = song.Item.ID.String()
	}

	sse.PatchElementTempl(trackcard.TrackCard(trackcard.Props{
		ID:    "playing-song",
		Track: song.Item.SimpleTrack,
	}), datastar.WithSelectorID("playing-song"))

	songs, err := spotifyClient.PlayerRecentlyPlayed(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sse.PatchElementTempl(trackcard.List(trackcard.ListProps{
		ID: "recent-songs",
		Tracks: mapSlice(songs, func(item spotify.RecentlyPlayedItem) spotify.SimpleTrack {
			return item.Track
		}),
	}), datastar.WithSelectorID("recent-songs"))

	sse.MarshalAndPatchSignals(store)
}

func QueueTrack(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	spotifyClient := getSpotifyClient(r)
	sse := datastar.NewSSE(w, r)

	track, err := spotifyClient.GetTrack(r.Context(), spotify.ID(r.FormValue("track_id")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = spotifyClient.QueueSong(r.Context(), track.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sse.PatchElementTempl(toast.Toast(toast.Props{
		ID:    "queue-toast",
		Title: "Queued " + track.Name,
	}), datastar.WithSelectorID("toasts"), datastar.WithModeAppend())
}

func UpdateSelectedSong(w http.ResponseWriter, r *http.Request) {
	spotifyClient := getSpotifyClient(r)
	store := getStore[TemplCounterSignals](w, r)
	sse := datastar.NewSSE(w, r)

	song, _ := spotifyClient.GetTrack(r.Context(), spotify.ID(store.SelectedSong))

	sse.PatchElementTempl(trackcard.TrackCard(trackcard.Props{
		ID:    "selected-song",
		Track: song.SimpleTrack,
	}), datastar.WithSelectorID("selected-song"))

	tracks, _ := spotifyClient.GetRecommendations(r.Context(), spotify.Seeds{
		Tracks: []spotify.ID{spotify.ID(store.SelectedSong)},
	}, nil)

	sse.PatchElementTempl(trackcard.List(trackcard.ListProps{
		ID:     "recommended-songs",
		Tracks: tracks.Tracks,
	}), datastar.WithSelectorID("recommended-songs"))
}
