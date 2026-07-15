package reconciliation

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type WebhookNotifier struct {
	url    string
	secret string
	client *http.Client
}

func NewWebhookNotifier(url, secret string, client *http.Client) *WebhookNotifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebhookNotifier{url: strings.TrimSpace(url), secret: secret, client: client}
}

func (n *WebhookNotifier) Notify(ctx context.Context, workspaceID string, run appsystem.ReconciliationRun) error {
	payload, err := json.Marshal(struct {
		Type             string                          `json:"type"`
		WorkspaceID      string                          `json:"workspace_id"`
		RunID            string                          `json:"run_id"`
		Status           string                          `json:"status"`
		IssueCount       int                             `json:"issue_count"`
		DecisionsChecked int                             `json:"decisions_checked"`
		CountersChecked  int                             `json:"counters_checked"`
		DurationMS       int64                           `json:"duration_ms"`
		Fingerprint      string                          `json:"fingerprint"`
		Issues           []appsystem.ReconciliationIssue `json:"issues"`
		CreatedAt        time.Time                       `json:"created_at"`
	}{Type: "reconciliation.drift_detected", WorkspaceID: workspaceID, RunID: run.ID, Status: run.Status, IssueCount: run.IssueCount, DecisionsChecked: run.DecisionsChecked, CountersChecked: run.CountersChecked, DurationMS: run.Duration.Milliseconds(), Fingerprint: run.Fingerprint, Issues: run.Issues, CreatedAt: run.CreatedAt})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.secret != "" {
		mac := hmac.New(sha256.New, []byte(n.secret))
		_, _ = mac.Write(payload)
		req.Header.Set("X-Open-Spanner-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("reconciliation webhook returned %s", res.Status)
	}
	return nil
}
