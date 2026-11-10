package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// maxJSONBody caps every JSON body; route groups may set a tighter limit with limitBody.
const maxJSONBody = 1 << 20

// decodeJSON reads exactly one JSON value with only known fields, or answers 400.
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeReply(w, replyInvalidRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeReply(w, replyInvalidRequest)
		return false
	}
	return true
}

// limitBody caps request bodies of a route group.
func limitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// noStore keeps private and per-user responses out of caches.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func pathParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
