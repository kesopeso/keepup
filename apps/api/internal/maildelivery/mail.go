// Package maildelivery supplies interchangeable delivery for account email.
package maildelivery

import (
	"context"
	"sync"
)

// Message is provider-independent; ID remains stable across delivery retries.
type Message struct{ ID, To, Subject, Text string }

// Sender delivers one message. Implementations must honor context cancellation.
// Delivery is at least once: SMTP acceptance followed by a lost acknowledgment
// can cause a retry. Tokens remain single use independently of delivery.
type Sender interface {
	Send(context.Context, Message) error
}

// Fake records successful deliveries without contacting external services.
type Fake struct {
	mu       sync.Mutex
	messages []Message
	err      error
}

// SetError makes subsequent deliveries fail, without recording them.
func (f *Fake) SetError(err error) { f.mu.Lock(); defer f.mu.Unlock(); f.err = err }

// Messages returns a copy for tests; no runtime endpoint exposes these secrets.
func (f *Fake) Messages() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.messages...)
}

// Send implements Sender.
func (f *Fake) Send(ctx context.Context, m Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.messages = append(f.messages, m)
	return nil
}
