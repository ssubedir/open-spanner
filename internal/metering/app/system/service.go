package system

import (
	"context"
	"time"
)

type Service interface {
	Stats(ctx context.Context) (StatsResult, error)
}

type service struct {
	repo Repository
}

type Repository interface {
	FindStats(ctx context.Context) (StatsResult, error)
}

type StatsResult struct {
	Meters       int
	UsageEvents  int
	PruneRuns    int
	LastPruneRun LastPruneRunResult
}

type LastPruneRunResult struct {
	ID        string
	Deleted   int
	DryRun    bool
	CreatedAt time.Time
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Stats(ctx context.Context) (StatsResult, error) {
	return s.repo.FindStats(ctx)
}
