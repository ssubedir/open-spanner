package meter

import (
	"context"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type UpdateCommand struct {
	ID          string
	Description string
}

type DeleteCommand struct {
	ID string
}

func (s *service) Update(ctx context.Context, cmd UpdateCommand) (Result, error) {
	meters, err := s.repo.Find(ctx, domainmeter.Query{ID: cmd.ID})
	if err != nil {
		return Result{}, err
	}
	if len(meters) == 0 {
		return Result{}, domain.ErrNotFound
	}

	meter, err := s.repo.Save(ctx, meters[0].WithDescription(cmd.Description))
	if err != nil {
		return Result{}, err
	}

	return resultFromDomain(meter), nil
}

func (s *service) Delete(ctx context.Context, cmd DeleteCommand) error {
	return s.repo.Delete(ctx, domainmeter.Query{ID: cmd.ID})
}
