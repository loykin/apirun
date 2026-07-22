// Command mockserver is a tiny local stand-in for httpbin.org, used so the
// stages_embedded example does not depend on external network availability.
// It mimics the subset of httpbin's behavior the example relies on:
// POST/DELETE echo the request body back under a "json" key, and GET /json
// returns the well-known httpbin slideshow payload.
package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var parsed interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err != nil {
			parsed = nil
		}
	}

	resp := map[string]interface{}{
		"url":    r.URL.String(),
		"origin": r.RemoteAddr,
		"json":   parsed,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func jsonHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"slideshow":{"title":"Sample Slide Show","date":"date of publication"}}`))
}

func main() {
	addr := flag.String("addr", ":18080", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/post", echoHandler)
	mux.HandleFunc("/delete", echoHandler)
	mux.HandleFunc("/json", jsonHandler)

	log.Printf("mockserver listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
