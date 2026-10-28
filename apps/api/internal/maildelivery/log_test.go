package maildelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
)

func TestLogDelivery(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "preview"
		if cancelled {
			name = "cancelled"
		}
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			sender := NewLog(slog.New(slog.NewJSONHandler(&output, nil)))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			}
			message := Message{ID: "stable-id", To: "dev@example.com", Subject: "Verify your email", Text: "Open this link:\nhttps://keepup.example/auth/verify#token=synthetic\n"}
			err := sender.Send(ctx, message)
			if cancelled {
				if !errors.Is(err, context.Canceled) || output.Len() != 0 {
					t.Fatalf("cancelled delivery logged email or returned unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var record map[string]any
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{"provider": "log", "message_id": message.ID, "to": message.To, "subject": message.Subject, "body": message.Text} {
				if record[key] != want {
					t.Errorf("%s = %v, want %q", key, record[key], want)
				}
			}
		})
	}
}
