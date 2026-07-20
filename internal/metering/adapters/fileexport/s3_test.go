package fileexport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type fakeS3 struct {
	mu       sync.Mutex
	objects  map[string][]byte
	modified map[string]time.Time
	deleted  []string
}

func newFakeS3() *fakeS3 {
	return &fakeS3{objects: map[string][]byte{}, modified: map[string]time.Time{}}
}

func (f *fakeS3) UploadObject(_ context.Context, input *transfermanager.UploadObjectInput, _ ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[aws.ToString(input.Key)] = data
	f.modified[aws.ToString(input.Key)] = time.Now().UTC()
	return &transfermanager.UploadObjectOutput{}, nil
}

func (f *fakeS3) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, fakeAPIError{code: "NotFound"}
	}
	size, modified := int64(len(data)), f.modified[aws.ToString(input.Key)]
	return &s3.HeadObjectOutput{ContentLength: &size, LastModified: &modified}, nil
}

func (f *fakeS3) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, fakeAPIError{code: "NoSuchKey"}
	}
	if input.Range != nil {
		var start, end int
		if _, err := fmt.Sscanf(aws.ToString(input.Range), "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || end >= len(data) {
			return nil, fakeAPIError{code: "InvalidRange"}
		}
		data = data[start : end+1]
	}
	size, modified := int64(len(data)), f.modified[aws.ToString(input.Key)]
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data)), ContentLength: &size, LastModified: &modified}, nil
}

func (f *fakeS3) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := aws.ToString(input.Key)
	delete(f.objects, key)
	delete(f.modified, key)
	f.deleted = append(f.deleted, key)
	return &s3.DeleteObjectOutput{}, nil
}

type fakeAPIError struct{ code string }

func (e fakeAPIError) Error() string                 { return e.code }
func (e fakeAPIError) ErrorCode() string             { return e.code }
func (e fakeAPIError) ErrorMessage() string          { return e.code }
func (e fakeAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestS3StoreContract(t *testing.T) {
	fake := newFakeS3()
	testStoreContract(t, &S3Store{bucket: "exports", prefix: "tenant/exports", client: fake, uploader: fake})
	if len(fake.deleted) != 2 || fake.deleted[0] != "tenant/exports/artifact.csv" {
		t.Fatalf("deleted keys = %#v", fake.deleted)
	}
}

func TestS3StoreCleansUpFailedWrites(t *testing.T) {
	fake := newFakeS3()
	store := &S3Store{bucket: "exports", prefix: "prefix", client: fake, uploader: fake}
	wantErr := errors.New("render failed")
	_, err := store.Write(context.Background(), "failed.csv", func(writer io.Writer) error {
		_, _ = io.WriteString(writer, "partial")
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("write error = %v", err)
	}
	if _, exists := fake.objects["prefix/failed.csv"]; exists {
		t.Fatal("partial object was not deleted")
	}
}

var _ s3ObjectClient = (*fakeS3)(nil)
var _ s3Uploader = (*fakeS3)(nil)
