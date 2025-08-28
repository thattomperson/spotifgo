package utils

import (
	"net/http"

	datastar "github.com/starfederation/datastar-go/datastar"
)

func ReadSignals[T any](w http.ResponseWriter, r *http.Request) *T {
	var store = new(T)
	if err := datastar.ReadSignals(r, store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}
	return store
}
