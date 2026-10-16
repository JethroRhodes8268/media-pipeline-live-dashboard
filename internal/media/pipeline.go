package media

import (
	"errors"
	"time"
)

type Stage string

const (
	Ingested  Stage = "asset_ingested"
	Processed Stage = "job_processed"
	Delivered Stage = "creator_delivered"
)

type Event struct {
	AssetID  string    `json:"asset_id"`
	Stage    Stage     `json:"stage"`
	Duration int64     `json:"duration_ms"`
	At       time.Time `json:"at"`
}

type Snapshot struct {
	AssetID      string    `json:"asset_id"`
	Stage        Stage     `json:"stage"`
	Ingested     int       `json:"assets_ingested"`
	Processed    int       `json:"jobs_processed"`
	Delivered    int       `json:"creator_deliveries"`
	ProcessingMS int64     `json:"processing_ms"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type Tracker struct {
	states map[string]Stage
	counts Snapshot
}

func NewTracker() *Tracker { return &Tracker{states: make(map[string]Stage)} }

func (t *Tracker) Apply(e Event) (Snapshot, error) {
	if e.AssetID == "" || e.At.IsZero() {
		return Snapshot{}, errors.New("asset_id and at are required")
	}
	previous := t.states[e.AssetID]
	switch e.Stage {
	case Ingested:
		if previous != "" {
			return Snapshot{}, errors.New("asset has already entered the pipeline")
		}
		t.counts.Ingested++
	case Processed:
		if previous != Ingested {
			return Snapshot{}, errors.New("processing requires an ingested asset")
		}
		t.counts.Processed++
		t.counts.ProcessingMS = e.Duration
	case Delivered:
		if previous != Processed {
			return Snapshot{}, errors.New("delivery requires a processed asset")
		}
		t.counts.Delivered++
	default:
		return Snapshot{}, errors.New("unknown pipeline stage")
	}
	t.states[e.AssetID] = e.Stage
	s := t.counts
	s.AssetID, s.Stage, s.RecordedAt = e.AssetID, e.Stage, e.At
	return s, nil
}
