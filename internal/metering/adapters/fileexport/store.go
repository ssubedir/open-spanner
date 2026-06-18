package fileexport

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

var artifactNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Artifact struct {
	Name string
	Size int64
}

type Store struct {
	root string
}

func NewStore(root string) Store {
	return Store{root: root}
}

func (s Store) Write(ctx context.Context, name string, write func(io.Writer) error) (Artifact, error) {
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
	return Artifact{Name: name, Size: info.Size()}, nil
}

func (s Store) Open(name string) (*os.File, os.FileInfo, error) {
	path, err := s.path(name)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

func (s Store) path(name string) (string, error) {
	if s.root == "" {
		return "", fmt.Errorf("%w: export storage path is required", domain.ErrInvalidInput)
	}
	if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || !artifactNamePattern.MatchString(name) {
		return "", fmt.Errorf("%w: invalid export artifact name", domain.ErrInvalidInput)
	}
	return filepath.Join(s.root, name), nil
}
