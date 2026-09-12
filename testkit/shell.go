package testkit

import (
	"net/http"

	"github.com/housecat-inc/scratch/pkg/ui"
)

func ShellFixture(handler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
	mux.HandleFunc("/app/updates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pending":false}`))
	})
	mux.HandleFunc("/inbox/workflows/running-count", func(w http.ResponseWriter, r *http.Request) {})
	mux.Handle("/", handler)
	return mux
}
