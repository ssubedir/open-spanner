package reconciliation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

func TestWebhookNotifierSignsPayload(t *testing.T) {
	var signature string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signature = r.Header.Get("X-Open-Spanner-Signature")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := NewWebhookNotifier(server.URL, "secret", server.Client()).Notify(context.Background(), appsystem.ReconciliationNotification{WorkspaceID: "workspace", EventType: "drift_detected", Run: appsystem.ReconciliationRun{ID: "run", Status: "drift_detected"}})
	if err != nil || !strings.HasPrefix(signature, "sha256=") {
		t.Fatalf("err=%v signature=%q", err, signature)
	}
}
