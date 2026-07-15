package fileexport

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func TestFileStoreContract(t *testing.T) {
	testStoreContract(t, NewStore(t.TempDir()))
}

func testStoreContract(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()
	artifact, err := store.Write(ctx, "artifact.csv", func(writer io.Writer) error {
		_, err := io.WriteString(writer, "header\nvalue\n")
		return err
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if artifact.Name != "artifact.csv" || artifact.Size != 13 {
		t.Fatalf("artifact = %#v", artifact)
	}
	object, err := store.Open(ctx, artifact.Name)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	data, readErr := io.ReadAll(object.Body)
	closeErr := object.Body.Close()
	if readErr != nil || closeErr != nil || string(data) != "header\nvalue\n" || object.Artifact.Size != artifact.Size {
		t.Fatalf("read data=%q artifact=%#v readErr=%v closeErr=%v", data, object.Artifact, readErr, closeErr)
	}
	if err := store.Remove(ctx, artifact.Name); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := store.Open(ctx, artifact.Name); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("open removed error = %v", err)
	}
	if err := store.Remove(ctx, artifact.Name); err != nil {
		t.Fatalf("idempotent remove: %v", err)
	}
}

func TestFileStoreRejectsUnsafeNames(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Write(context.Background(), "../artifact.csv", func(io.Writer) error { return nil }); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("write error = %v", err)
	}
}
