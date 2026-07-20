package usage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/fileexport"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
)

func TestServeExportArtifactClearsWriteDeadline(t *testing.T) {
	data := []byte("0123456789")
	store := &downloadTestStore{data: data, delay: 100 * time.Millisecond}
	handler := &Handler{exportStore: store}
	job := appusage.ExportJobResult{ID: "slow", ArtifactPath: "slow.csv"}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.serveExportArtifact(w, r, job)
	}))
	server.Config.WriteTimeout = 25 * time.Millisecond
	server.Start()
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("slow export request: %v", err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read slow export: %v", err)
	}
	if response.StatusCode != http.StatusOK || string(got) != string(data) {
		t.Fatalf("status=%d body=%q", response.StatusCode, got)
	}
}

func TestServeExportArtifactSupportsHeadAndRanges(t *testing.T) {
	data := []byte("0123456789")
	handler := &Handler{exportStore: &downloadTestStore{data: data}}
	job := appusage.ExportJobResult{ID: "range", ArtifactPath: "range.csv"}

	head := httptest.NewRecorder()
	handler.serveExportArtifact(head, httptest.NewRequest(http.MethodHead, "/download", nil), job)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != "10" || head.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("HEAD status=%d headers=%v body=%q", head.Code, head.Header(), head.Body.String())
	}

	rangeRequest := httptest.NewRequest(http.MethodGet, "/download", nil)
	rangeRequest.Header.Set("Range", "bytes=2-5")
	partial := httptest.NewRecorder()
	handler.serveExportArtifact(partial, rangeRequest, job)
	if partial.Code != http.StatusPartialContent || partial.Body.String() != "2345" || partial.Header().Get("Content-Range") != "bytes 2-5/10" || partial.Header().Get("Content-Length") != "4" {
		t.Fatalf("range status=%d headers=%v body=%q", partial.Code, partial.Header(), partial.Body.String())
	}

	invalidRequest := httptest.NewRequest(http.MethodGet, "/download", nil)
	invalidRequest.Header.Set("Range", "bytes=20-30")
	invalid := httptest.NewRecorder()
	handler.serveExportArtifact(invalid, invalidRequest, job)
	if invalid.Code != http.StatusRequestedRangeNotSatisfiable || invalid.Header().Get("Content-Range") != "bytes */10" {
		t.Fatalf("invalid range status=%d headers=%v", invalid.Code, invalid.Header())
	}
}

func TestServeExportArtifactCancellationClosesBody(t *testing.T) {
	store := &downloadTestStore{data: []byte("blocked"), block: true, opened: make(chan struct{}), closed: make(chan struct{})}
	handler := &Handler{exportStore: store}
	job := appusage.ExportJobResult{ID: "cancel", ArtifactPath: "cancel.csv"}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/download", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		handler.serveExportArtifact(httptest.NewRecorder(), request, job)
		close(done)
	}()
	<-store.opened
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled download did not return")
	}
	select {
	case <-store.closed:
	default:
		t.Fatal("canceled download body was not closed")
	}
}

type downloadTestStore struct {
	data   []byte
	delay  time.Duration
	block  bool
	opened chan struct{}
	closed chan struct{}
}

func (s *downloadTestStore) Write(context.Context, string, func(io.Writer) error) (fileexport.Artifact, error) {
	return fileexport.Artifact{}, errors.New("not implemented")
}

func (s *downloadTestStore) Stat(_ context.Context, name string) (fileexport.Artifact, error) {
	return fileexport.Artifact{Name: name, Size: int64(len(s.data)), ModTime: time.Unix(1, 0).UTC()}, nil
}

func (s *downloadTestStore) Open(ctx context.Context, name string) (fileexport.Object, error) {
	return s.object(ctx, name, s.data), nil
}

func (s *downloadTestStore) OpenRange(ctx context.Context, name string, byteRange fileexport.ByteRange) (fileexport.Object, error) {
	return s.object(ctx, name, s.data[byteRange.Start:byteRange.End+1]), nil
}

func (s *downloadTestStore) Remove(context.Context, string) error { return nil }

func (s *downloadTestStore) object(ctx context.Context, name string, data []byte) fileexport.Object {
	if s.opened != nil {
		close(s.opened)
		s.opened = nil
	}
	body := &downloadTestBody{ctx: ctx, data: data, delay: s.delay, block: s.block, closed: s.closed}
	return fileexport.Object{Body: body, Artifact: fileexport.Artifact{Name: name, Size: int64(len(data))}}
}

type downloadTestBody struct {
	ctx     context.Context
	data    []byte
	offset  int
	delay   time.Duration
	delayed bool
	block   bool
	closed  chan struct{}
	once    sync.Once
}

func (b *downloadTestBody) Read(p []byte) (int, error) {
	if b.block {
		<-b.ctx.Done()
		return 0, b.ctx.Err()
	}
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	if b.offset > 0 && !b.delayed && b.delay > 0 {
		b.delayed = true
		time.Sleep(b.delay)
	}
	limit := len(b.data)
	if b.offset == 0 && len(b.data) > 1 {
		limit = len(b.data) / 2
	}
	n := copy(p, b.data[b.offset:limit])
	b.offset += n
	return n, nil
}

func (b *downloadTestBody) Close() error {
	if b.closed != nil {
		b.once.Do(func() { close(b.closed) })
	}
	return nil
}
