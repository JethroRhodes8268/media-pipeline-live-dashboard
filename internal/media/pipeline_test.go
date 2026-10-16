package media

import (
	"testing"
	"time"
)

func TestTrackerPipelineDecision(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	tests := []struct {
		name      string
		events    []Event
		wantStage Stage
		wantCount int
		wantErr   bool
	}{
		{"complete delivery", []Event{{"asset-7", Ingested, 0, now}, {"asset-7", Processed, 840, now}, {"asset-7", Delivered, 0, now}}, Delivered, 1, false},
		{"reject delivery before processing", []Event{{"asset-8", Ingested, 0, now}, {"asset-8", Delivered, 0, now}}, "", 0, true},
		{"reject duplicate ingestion", []Event{{"asset-9", Ingested, 0, now}, {"asset-9", Ingested, 0, now}}, "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker()
			var got Snapshot
			var err error
			for _, event := range tt.events {
				got, err = tracker.Apply(event)
				if err != nil {
					break
				}
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v", err)
			}
			if !tt.wantErr && (got.Stage != tt.wantStage || got.Delivered != tt.wantCount) {
				t.Fatalf("snapshot = %#v", got)
			}
		})
	}
}
