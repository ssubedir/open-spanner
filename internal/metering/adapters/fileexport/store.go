package fileexport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

var artifactNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Artifact struct {
	Name    string
	Size    int64
	ModTime time.Time
}

type Object struct {
	Body     io.ReadCloser
	Artifact Artifact
}

// ByteRange identifies an inclusive segment of an export artifact.
type ByteRange struct {
	Start int64
	End   int64
}

type Store interface {
	Write(ctx context.Context, name string, write func(io.Writer) error) (Artifact, error)
	Stat(ctx context.Context, name string) (Artifact, error)
	Open(ctx context.Context, name string) (Object, error)
	OpenRange(ctx context.Context, name string, byteRange ByteRange) (Object, error)
	Remove(ctx context.Context, name string) error
}

type FileStore struct {
	root string
}

func NewStore(root string) Store {
	return FileStore{root: root}
}

func (s FileStore) Write(ctx context.Context, name string, write func(io.Writer) error) (Artifact, error) {
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}
	if write == nil {
		return Artifact{}, fmt.Errorf("%w: export writer is required", domain.ErrInvalidInput)
	}
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return Artifact{}, err
	}

	finalPath, err := s.path(name)
	if err != nil {
		return Artifact{}, err
	}
	tmp, err := os.CreateTemp(s.root, name+".*.tmp")
	if err != nil {
		return Artifact{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := write(tmp); err != nil {
		_ = tmp.Close()
		return Artifact{}, err
	}
	if err := tmp.Close(); err != nil {
		return Artifact{}, err
	}
	if err := os.Remove(finalPath); err != nil && !os.IsNotExist(err) {
		return Artifact{}, err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return Artifact{}, err
	}

	info, err := os.Stat(finalPath)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{Name: name, Size: info.Size(), ModTime: info.ModTime().UTC()}, nil
}

func (s FileStore) Open(ctx context.Context, name string) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	path, err := s.path(name)
	if err != nil {
		return Object{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Object{}, errors.Join(domain.ErrNotFound, err)
		}
		return Object{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return Object{}, err
	}
	return Object{Body: file, Artifact: Artifact{Name: name, Size: info.Size(), ModTime: info.ModTime().UTC()}}, nil
}

func (s FileStore) Stat(ctx context.Context, name string) (Artifact, error) {
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}
	path, err := s.path(name)
	if err != nil {
		return Artifact{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Artifact{}, errors.Join(domain.ErrNotFound, err)
		}
		return Artifact{}, err
	}
	return Artifact{Name: name, Size: info.Size(), ModTime: info.ModTime().UTC()}, nil
}

func (s FileStore) OpenRange(ctx context.Context, name string, byteRange ByteRange) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	if byteRange.Start < 0 || byteRange.End < byteRange.Start {
		return Object{}, fmt.Errorf("%w: invalid export artifact byte range", domain.ErrInvalidInput)
	}
	path, err := s.path(name)
	if err != nil {
		return Object{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Object{}, errors.Join(domain.ErrNotFound, err)
		}
		return Object{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return Object{}, err
	}
	if byteRange.End >= info.Size() {
		_ = file.Close()
		return Object{}, fmt.Errorf("%w: export artifact byte range exceeds object size", domain.ErrInvalidInput)
	}
	length := byteRange.End - byteRange.Start + 1
	body := &sectionReadCloser{Reader: io.NewSectionReader(file, byteRange.Start, length), Closer: file}
	return Object{Body: body, Artifact: Artifact{Name: name, Size: length, ModTime: info.ModTime().UTC()}}, nil
}

func (s FileStore) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s FileStore) path(name string) (string, error) {
	if s.root == "" {
		return "", fmt.Errorf("%w: export storage path is required", domain.ErrInvalidInput)
	}
	if err := validateName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.root, name), nil
}

func validateName(name string) error {
	if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || !artifactNamePattern.MatchString(name) {
		return fmt.Errorf("%w: invalid export artifact name", domain.ErrInvalidInput)
	}
	return nil
}

type sectionReadCloser struct {
	io.Reader
	io.Closer
}
