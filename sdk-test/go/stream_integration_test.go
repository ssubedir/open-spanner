package sdk_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

func TestGoStreamClientUsageFlow(t *testing.T) {
	httpAddr := freeTCPAddr(t)
	grpcAddr := freeTCPAddr(t)
	baseURL := startOpenSpanner(t, httpAddr, grpcAddr)

	suffix := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	apiKey := createAPIKey(t, baseURL, suffix)
	meterName := "sdk_stream_requests_" + suffix
	createMeter(t, baseURL, apiKey, meterName)

	client, err := stream.NewClient(grpcAddr, apiKey)
	if err != nil {
		t.Fatalf("create stream client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close stream client: %v", err)
		}
	})

	now := time.Now().UTC()
	bulk, err := client.TrackBulk(t.Context(), "sdk-stream-bulk-"+suffix, []stream.Event{
		usageEvent("sdk-stream-bulk-"+suffix+"-1", "org_sdk_stream_"+suffix, meterName, 2, now, map[string]any{"endpoint": "/orders", "status": 200}),
		usageEvent("sdk-stream-bulk-"+suffix+"-2", "org_sdk_stream_"+suffix, meterName, 3, now.Add(time.Second), map[string]any{"endpoint": "/users", "status": 201}),
	})
	if err != nil {
		t.Fatalf("track bulk usage: %v", err)
	}
	if bulk.AcceptedCount != 2 || bulk.DuplicateCount != 0 || bulk.FailedCount != 0 {
		t.Fatalf("bulk result = accepted %d duplicate %d failed %d, want 2/0/0", bulk.AcceptedCount, bulk.DuplicateCount, bulk.FailedCount)
	}

	usageStream, err := client.Stream(t.Context(), "sdk-stream-"+suffix)
	if err != nil {
		t.Fatalf("open usage stream: %v", err)
	}
	if err := usageStream.Track(usageEvent("sdk-stream-"+suffix+"-1", "org_sdk_stream_"+suffix, meterName, 7, now.Add(2*time.Second), map[string]any{"endpoint": "/checkout", "status": 200})); err != nil {
		t.Fatalf("track streamed usage: %v", err)
	}
	streamed, err := usageStream.Close()
	if err != nil {
		t.Fatalf("close usage stream: %v", err)
	}
	if streamed.AcceptedCount != 1 || streamed.DuplicateCount != 0 || streamed.FailedCount != 0 {
		t.Fatalf("stream result = accepted %d duplicate %d failed %d, want 1/0/0", streamed.AcceptedCount, streamed.DuplicateCount, streamed.FailedCount)
	}

	events := listUsageEvents(t, baseURL, apiKey, meterName)
	if len(events.Items) != 3 {
		t.Fatalf("usage events = %d, want 3", len(events.Items))
	}
}

func startOpenSpanner(t *testing.T, httpAddr string, grpcAddr string) string {
	t.Helper()

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	binaryName := "open-spanner-sdk-test"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(t.TempDir(), binaryName)

	var buildLog bytes.Buffer
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/api")
	build.Dir = repoRoot
	build.Stdout = &buildLog
	build.Stderr = &buildLog
	if err := build.Run(); err != nil {
		t.Fatalf("build API binary: %v\n%s", err, buildLog.String())
	}

	var serverLog bytes.Buffer
	cmd := exec.Command(binaryPath)
	cmd.Dir = repoRoot
	cmd.Stdout = &serverLog
	cmd.Stderr = &serverLog
	cmd.Env = append(os.Environ(),
		"OPEN_SPANNER_HTTP_ADDR="+httpAddr,
		"OPEN_SPANNER_GRPC_ADDR="+grpcAddr,
		"OPEN_SPANNER_DB_DRIVER=sqlite",
		"OPEN_SPANNER_SQLITE_PATH="+filepath.Join(t.TempDir(), "open-spanner.db"),
		"OPEN_SPANNER_EXPORT_STORAGE_PATH="+t.TempDir(),
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start API binary: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	t.Cleanup(func() {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return
		}
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("API process did not exit after kill")
		}
	})

	baseURL := "http://" + httpAddr
	waitForReady(t, baseURL, done, &serverLog)
	return baseURL
}

func waitForReady(t *testing.T, baseURL string, done <-chan error, serverLog *bytes.Buffer) {
	t.Helper()

	deadline := time.After(20 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()

	client := http.Client{Timeout: time.Second}
	for {
		select {
		case err := <-done:
			t.Fatalf("API process exited before ready: %v\n%s", err, serverLog.String())
		case <-deadline:
			t.Fatalf("API did not become ready\n%s", serverLog.String())
		case <-tick.C:
			res, err := client.Get(baseURL + "/ready")
			if err == nil {
				_ = res.Body.Close()
				if res.StatusCode == http.StatusNoContent {
					return
				}
			}
		}
	}
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free tcp port: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()
	return listener.Addr().String()
}

func createAPIKey(t *testing.T, baseURL string, suffix string) string {
	t.Helper()

	password := "strong-password"
	email := "sdk-stream+" + suffix + "@example.com"
	client := http.Client{Timeout: 5 * time.Second}

	postJSON(t, &client, baseURL+"/v1/auth/users", map[string]any{
		"email":    email,
		"password": password,
	}, nil, nil, http.StatusCreated, nil)

	sessionRes := postJSON(t, &client, baseURL+"/v1/auth/sessions", map[string]any{
		"email":    email,
		"password": password,
	}, nil, nil, http.StatusCreated, nil)

	var apiKey struct {
		Key string `json:"key"`
	}
	postJSON(t, &client, baseURL+"/v1/auth/api-keys", map[string]any{
		"name": "sdk stream test " + suffix,
	}, nil, sessionRes.Cookies(), http.StatusCreated, &apiKey)
	if apiKey.Key == "" {
		t.Fatalf("api key response did not include key")
	}
	return apiKey.Key
}

func createMeter(t *testing.T, baseURL string, apiKey string, meterName string) {
	t.Helper()

	required := true
	postJSON(t, &http.Client{Timeout: 5 * time.Second}, baseURL+"/v1/meters", map[string]any{
		"name":                 meterName,
		"description":          "SDK stream integration requests",
		"unit":                 "request",
		"aggregation":          "sum",
		"event_retention_days": 30,
		"dimensions": []map[string]any{
			{"name": "endpoint", "type": "string", "required": required},
			{"name": "status", "type": "number", "required": required},
		},
	}, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, nil, http.StatusCreated, nil)
}

func listUsageEvents(t *testing.T, baseURL string, apiKey string, meterName string) usageEventList {
	t.Helper()

	var events usageEventList
	endpoint := baseURL + "/v1/usageevents?meter=" + url.QueryEscape(meterName) + "&limit=10"
	getJSON(t, &http.Client{Timeout: 5 * time.Second}, endpoint, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, http.StatusOK, &events)
	return events
}

func usageEvent(idempotencyKey string, subject string, meterName string, quantity float64, eventTime time.Time, fields map[string]any) stream.Event {
	return stream.Event{
		IdempotencyKey: idempotencyKey,
		Subject:        subject,
		Meter:          meterName,
		Quantity:       quantity,
		Timestamp:      eventTime,
		Metadata:       fields,
	}
}

func postJSON(t *testing.T, client *http.Client, endpoint string, body any, headers map[string]string, cookies []*http.Cookie, wantStatus int, out any) *http.Response {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create post request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return doJSON(t, client, req, headers, cookies, wantStatus, out)
}

func getJSON(t *testing.T, client *http.Client, endpoint string, headers map[string]string, wantStatus int, out any) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("create get request: %v", err)
	}
	doJSON(t, client, req, headers, nil, wantStatus, out)
}

func doJSON(t *testing.T, client *http.Client, req *http.Request, headers map[string]string, cookies []*http.Cookie, wantStatus int, out any) *http.Response {
	t.Helper()

	for key, value := range headers {
		req.Header.Set(key, value)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s: %v", req.Method, req.URL, err)
	}
	if res.StatusCode != wantStatus {
		var errorBody bytes.Buffer
		_, _ = errorBody.ReadFrom(res.Body)
		_ = res.Body.Close()
		t.Fatalf("%s %s status = %d, want %d: %s", req.Method, req.URL, res.StatusCode, wantStatus, errorBody.String())
	}
	if out != nil {
		defer func() {
			_ = res.Body.Close()
		}()
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s response: %v", req.Method, req.URL, err)
		}
		return res
	}
	_ = res.Body.Close()
	return res
}

type usageEventList struct {
	Items []struct {
		Meter    string         `json:"meter"`
		Subject  string         `json:"subject"`
		Quantity float64        `json:"quantity"`
		Metadata map[string]any `json:"metadata"`
	} `json:"items"`
}
