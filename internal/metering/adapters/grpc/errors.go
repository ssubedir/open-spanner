package grpcadapter

import (
	"errors"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func serviceError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrRateLimited):
		retryAfter := time.Second
		var limited interface{ RetryAfter() time.Duration }
		if errors.As(err, &limited) && limited.RetryAfter() > 0 {
			retryAfter = limited.RetryAfter()
		}
		st := status.New(codes.ResourceExhausted, err.Error())
		withDetails, detailErr := st.WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(retryAfter)})
		if detailErr == nil {
			return withDetails.Err()
		}
		return st.Err()
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
