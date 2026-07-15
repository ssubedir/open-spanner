package fileexport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const (
	DriverFilesystem = "filesystem"
	DriverS3         = "s3"
)

type Options struct {
	Driver            string
	FilesystemPath    string
	S3Bucket          string
	S3Region          string
	S3Endpoint        string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3SessionToken    string
	S3Prefix          string
	S3ForcePathStyle  bool
}

func New(ctx context.Context, options Options) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(options.Driver)) {
	case "", DriverFilesystem:
		return NewStore(options.FilesystemPath), nil
	case DriverS3:
		return NewS3Store(ctx, options)
	default:
		return nil, fmt.Errorf("%w: unsupported export storage driver %q", domain.ErrInvalidInput, options.Driver)
	}
}

type s3ObjectClient interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type s3Uploader interface {
	UploadObject(context.Context, *transfermanager.UploadObjectInput, ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error)
}

type S3Store struct {
	bucket   string
	prefix   string
	client   s3ObjectClient
	uploader s3Uploader
}

func NewS3Store(ctx context.Context, options Options) (Store, error) {
	if strings.TrimSpace(options.S3Bucket) == "" || strings.TrimSpace(options.S3Region) == "" {
		return nil, fmt.Errorf("%w: S3 bucket and region are required", domain.ErrInvalidInput)
	}
	if (options.S3AccessKeyID == "") != (options.S3SecretAccessKey == "") {
		return nil, fmt.Errorf("%w: S3 access key and secret key must be set together", domain.ErrInvalidInput)
	}
	if options.S3SessionToken != "" && options.S3AccessKeyID == "" {
		return nil, fmt.Errorf("%w: S3 session token requires static credentials", domain.ErrInvalidInput)
	}
	if options.S3Endpoint != "" {
		endpoint, err := url.ParseRequestURI(options.S3Endpoint)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return nil, fmt.Errorf("%w: S3 endpoint must be an absolute HTTP(S) URL", domain.ErrInvalidInput)
		}
	}
	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(options.S3Region)}
	if options.S3AccessKeyID != "" || options.S3SecretAccessKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(options.S3AccessKeyID, options.S3SecretAccessKey, options.S3SessionToken)))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(s3Options *s3.Options) {
		if options.S3Endpoint != "" {
			s3Options.BaseEndpoint = aws.String(options.S3Endpoint)
		}
		s3Options.UsePathStyle = options.S3ForcePathStyle
	})
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := client.HeadBucket(checkCtx, &s3.HeadBucketInput{Bucket: aws.String(options.S3Bucket)}); err != nil {
		return nil, fmt.Errorf("verify S3 export bucket access: %w", err)
	}
	return &S3Store{bucket: options.S3Bucket, prefix: strings.Trim(strings.TrimSpace(options.S3Prefix), "/"), client: client, uploader: transfermanager.New(client)}, nil
}

func (s *S3Store) Write(ctx context.Context, name string, write func(io.Writer) error) (Artifact, error) {
	if err := validateName(name); err != nil {
		return Artifact{}, err
	}
	if write == nil {
		return Artifact{}, fmt.Errorf("%w: export writer is required", domain.ErrInvalidInput)
	}
	reader, writer := io.Pipe()
	type writeResult struct {
		size int64
		err  error
	}
	written := make(chan writeResult, 1)
	go func() {
		counter := &countingWriter{writer: writer}
		err := write(counter)
		if err != nil {
			_ = writer.CloseWithError(err)
		} else {
			err = writer.Close()
		}
		written <- writeResult{size: counter.size, err: err}
	}()

	key := s.key(name)
	_, uploadErr := s.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: reader, ContentType: aws.String("text/csv")})
	if uploadErr != nil {
		_ = reader.CloseWithError(uploadErr)
	} else {
		_ = reader.Close()
	}
	result := <-written
	if result.err != nil || uploadErr != nil {
		s.removePartial(ctx, name)
		return Artifact{}, errors.Join(result.err, uploadErr)
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		s.removePartial(ctx, name)
		return Artifact{}, fmt.Errorf("verify uploaded export artifact: %w", err)
	}
	if head.ContentLength == nil || *head.ContentLength != result.size {
		s.removePartial(ctx, name)
		return Artifact{}, fmt.Errorf("verify uploaded export artifact: size is %d, want %d", aws.ToInt64(head.ContentLength), result.size)
	}
	modified := time.Now().UTC()
	if head.LastModified != nil {
		modified = head.LastModified.UTC()
	}
	return Artifact{Name: name, Size: result.size, ModTime: modified}, nil
}

func (s *S3Store) Open(ctx context.Context, name string) (Object, error) {
	if err := validateName(name); err != nil {
		return Object{}, err
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.key(name))})
	if err != nil {
		if isS3NotFound(err) {
			return Object{}, errors.Join(domain.ErrNotFound, err)
		}
		return Object{}, err
	}
	modified := time.Time{}
	if output.LastModified != nil {
		modified = output.LastModified.UTC()
	}
	size := int64(-1)
	if output.ContentLength != nil {
		size = *output.ContentLength
	}
	return Object{Body: output.Body, Artifact: Artifact{Name: name, Size: size, ModTime: modified}}, nil
}

func (s *S3Store) Remove(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.key(name))})
	return err
}

func (s *S3Store) key(name string) string {
	if s.prefix == "" {
		return name
	}
	return path.Join(s.prefix, name)
}

func (s *S3Store) removePartial(ctx context.Context, name string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	_ = s.Remove(cleanupCtx, name)
}

func isS3NotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchObject"
}

type countingWriter struct {
	writer io.Writer
	size   int64
}

func (w *countingWriter) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	w.size += int64(n)
	return n, err
}
