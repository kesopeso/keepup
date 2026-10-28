package maildelivery

import (
	"context"
	"log/slog"
)

// Log writes complete email previews, including authentication links, to the
// application logger without network delivery.
type Log struct {
	logger *slog.Logger
}

// NewLog creates an email preview provider without network delivery.
func NewLog(logger *slog.Logger) *Log {
	return &Log{logger: logger}
}

// Send logs one email preview. A successful preview counts as queue delivery.
func (l *Log) Send(ctx context.Context, m Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.logger.InfoContext(ctx, "email preview",
		"provider", "log", "message_id", m.ID, "to", m.To,
		"subject", m.Subject, "body", m.Text)
	return nil
}
