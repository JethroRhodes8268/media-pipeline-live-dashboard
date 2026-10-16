package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/example/media-pipeline-live-dashboard/internal/infrai"
	"github.com/example/media-pipeline-live-dashboard/internal/media"
)

const channel = "media-pipeline-operations"

type service struct {
	mu      sync.Mutex
	tracker *media.Tracker
	client  *infrai.Client
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &service{tracker: media.NewTracker(), client: infrai.New(key)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", s.events)
	mux.HandleFunc("POST /dashboard-token", s.dashboardToken)
	mux.HandleFunc("GET /", dashboard)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("media dashboard listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *service) events(w http.ResponseWriter, r *http.Request) {
	var event media.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	snapshot, err := s.tracker.Apply(event)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	points := []infrai.Point{
		{Name: "media.assets.ingested", Value: float64(snapshot.Ingested), Timestamp: event.At.Unix(), Tags: map[string]string{"stage": string(snapshot.Stage)}},
		{Name: "media.jobs.processed", Value: float64(snapshot.Processed), Timestamp: event.At.Unix(), Tags: map[string]string{"stage": string(snapshot.Stage)}},
		{Name: "media.creator.deliveries", Value: float64(snapshot.Delivered), Timestamp: event.At.Unix(), Tags: map[string]string{"stage": string(snapshot.Stage)}},
	}
	if err := s.client.RecordAndPublish(r.Context(), points, channel, snapshot, event.AssetID+":"+string(event.Stage)); err != nil {
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			writeError(w, apiErr.Status, err)
			return
		}
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusAccepted, snapshot)
}

func (s *service) dashboardToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.ClientID == "" {
		writeError(w, http.StatusBadRequest, errors.New("client_id is required"))
		return
	}
	data, err := s.client.IssueDashboardToken(r.Context(), input.ClientID, channel, input.ClientID+":"+time.Now().UTC().Format("200601021504"))
	if err != nil {
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			writeError(w, apiErr.Status, err)
			return
		}
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channel": channel, "credentials": data})
}

func dashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!doctype html><title>Media pipeline live dashboard</title><main><h1>Media pipeline</h1><p>Subscribe to <code>media-pipeline-operations</code> with credentials from <code>POST /dashboard-token</code>.</p><dl><dt>Assets ingested</dt><dd id="ingested">0</dd><dt>Jobs processed</dt><dd id="processed">0</dd><dt>Creator deliveries</dt><dd id="delivered">0</dd></dl></main>`))
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
