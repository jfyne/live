package live

import (
	"encoding/json"
	"testing"
)

func TestEventParams(t *testing.T) {
	e := Event{}
	p, err := e.Params()
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	if len(p) != 0 {
		t.Fatal("expected zero length map, got", p)
	}

	e.Data = []byte("wrong")
	_, err = e.Params()
	if err != ErrMessageMalformed {
		t.Error("expected ErrMessageMalformed, got", err)
	}
}

func TestEventIslandField(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected string
	}{
		{
			name: "event with island field",
			event: Event{
				T:      EventPatch,
				ID:     1,
				Island: "counter-1",
				Data:   json.RawMessage(`{"value": 42}`),
			},
			expected: `{"t":"patch","i":1,"island":"counter-1","d":{"value": 42}}`,
		},
		{
			name: "event without island field",
			event: Event{
				T:    EventConnect,
				ID:   2,
				Data: json.RawMessage(`{}`),
			},
			expected: `{"t":"connect","i":2,"d":{}}`,
		},
		{
			name: "event with empty island field",
			event: Event{
				T:      EventError,
				ID:     3,
				Island: "",
			},
			expected: `{"t":"err","i":3}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}

			// Unmarshal and re-marshal for consistent comparison
			var got map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			var expected map[string]interface{}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("failed to unmarshal expected: %v", err)
			}

			// Check that island field is present or absent as expected
			if tt.event.Island != "" {
				gotIsland, ok := got["island"]
				if !ok {
					t.Error("island field missing in serialized event")
				} else if gotIsland != tt.event.Island {
					t.Errorf("island field mismatch: got %v, want %v", gotIsland, tt.event.Island)
				}
			} else {
				if _, ok := got["island"]; ok {
					t.Error("island field should not be present when empty")
				}
			}

			// Verify event can be unmarshaled correctly
			var unmarshaled Event
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("failed to unmarshal into Event: %v", err)
			}

			if unmarshaled.Island != tt.event.Island {
				t.Errorf("unmarshaled island mismatch: got %q, want %q", unmarshaled.Island, tt.event.Island)
			}
		})
	}
}
