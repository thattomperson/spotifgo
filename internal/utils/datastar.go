package utils

import (
	"net/http"
	"net/url"

	"github.com/a-h/templ"
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

type DatastarWriter[T any] struct {
	Generator *datastar.ServerSentEventGenerator
}

func (r *DatastarWriter[T]) Replace(selector string, component templ.Component) {
	r.Generator.PatchElementTempl(component, datastar.WithSelector(selector))
}

func (r *DatastarWriter[T]) ReplaceInner(selector string, component templ.Component) {
	r.Generator.PatchElementTempl(component, datastar.WithSelector(selector), datastar.WithModeInner())
}

func (r *DatastarWriter[T]) Append(selector string, component templ.Component) {
	r.Generator.PatchElementTempl(component, datastar.WithSelector(selector), datastar.WithModeAppend())
}

func (r *DatastarWriter[T]) UpdateSignals(signals *T) {
	r.Generator.MarshalAndPatchSignals(signals)
}

type StarFunc[T any] func(*DatastarWriter[T], *T, *http.Request)

func Star[T any](fn StarFunc[T]) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		var store = new(T)
		if err := datastar.ReadSignals(r, store); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sse := datastar.NewSSE(w, r)
		response := &DatastarWriter[T]{
			Generator: sse,
		}
		fn(response, store, r)
	})
}

func Rpc(method string, data url.Values) string {
	parsedUrl, err := url.Parse("/rpc/" + method + "?" + data.Encode())
	if err != nil {
		panic(err)
	}

	return "@post('" + parsedUrl.String() + "')"
}
