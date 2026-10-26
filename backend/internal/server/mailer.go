package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"time"
)

type Message struct {
	To      string
	Subject string
	HTML    string
}
type Mailer interface {
	Send(context.Context, Message) error
}
type SenderMailer struct {
	APIKey    string
	FromEmail string
	FromName  string
	Client    *http.Client
	Endpoint  string
}

func (m SenderMailer) Send(ctx context.Context, message Message) error {
	endpoint := m.Endpoint
	if endpoint == "" {
		endpoint = "https://api.sender.net/v2/message/send"
	}
	body, err := json.Marshal(map[string]any{"from": map[string]string{"email": m.FromEmail, "name": m.FromName}, "to": map[string]string{"email": message.To}, "subject": message.Subject, "html": message.HTML})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sender unavailable")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8192))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("sender returned HTTP %d", response.StatusCode)
	}
	return nil
}

// At-least-once delivery. Crash after provider acceptance may duplicate an invitation.
// Outbox contents are purged after acceptance; pending invitations expire after 7 days.
func (s *Server) DeliverNext(ctx context.Context) (bool, error) {
	if s.Mailer == nil {
		return false, nil
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// Revoked shares cannot produce a pending invitation.
	_, err = tx.Exec(ctx, `delete from catalog.email_outbox o where o.delivered_at is not null or o.created_at<now()-interval '7 days' or not exists(select 1 from catalog.members m where m.catalog_id=o.catalog_id and m.email=o.recipient)`)
	if err != nil {
		return false, err
	}
	var id string
	var message Message
	err = tx.QueryRow(ctx, `select id::text,recipient,subject,html from catalog.email_outbox where delivered_at is null and next_attempt<=now() order by created_at for update skip locked limit 1`).Scan(&id, &message.To, &message.Subject, &message.HTML)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if sendErr := s.Mailer.Send(ctx, message); sendErr != nil {
		_, err = tx.Exec(ctx, `update catalog.email_outbox set attempts=attempts+1,next_attempt=now()+make_interval(secs=>least(3600,30*power(2,least(attempts,7)))::integer) where id=$1`, id)
	} else {
		_, err = tx.Exec(ctx, `delete from catalog.email_outbox where id=$1`, id)
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
