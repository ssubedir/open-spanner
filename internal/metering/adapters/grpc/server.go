package grpcadapter

import (
	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/grpc/pb"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"google.golang.org/grpc"
)

func NewServer(usageService appusage.Service, alertService appalert.Service, entitlementService appentitlement.Service, authService appauth.Service, authorizer appauth.Authorizer, opts ...grpc.ServerOption) *grpc.Server {
	return NewServerWithIngestionLimits(usageService, alertService, entitlementService, authService, authorizer, IngestionLimits{}, opts...)
}

func NewServerWithIngestionLimits(usageService appusage.Service, alertService appalert.Service, entitlementService appentitlement.Service, authService appauth.Service, authorizer appauth.Authorizer, limits IngestionLimits, opts ...grpc.ServerOption) *grpc.Server {
	return NewInstrumentedServerWithIngestionLimits(usageService, alertService, entitlementService, authService, authorizer, limits, nil, opts...)
}

type ServerMetrics interface {
	UnaryServerInterceptor() grpc.UnaryServerInterceptor
	StreamServerInterceptor() grpc.StreamServerInterceptor
}

func NewInstrumentedServerWithIngestionLimits(usageService appusage.Service, alertService appalert.Service, entitlementService appentitlement.Service, authService appauth.Service, authorizer appauth.Authorizer, limits IngestionLimits, metrics ServerMetrics, opts ...grpc.ServerOption) *grpc.Server {
	unaryInterceptors := []grpc.UnaryServerInterceptor{}
	streamInterceptors := []grpc.StreamServerInterceptor{}
	if metrics != nil {
		unaryInterceptors = append(unaryInterceptors, metrics.UnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, metrics.StreamServerInterceptor())
	}
	unaryInterceptors = append(unaryInterceptors, UnaryAuthInterceptor(authService))
	streamInterceptors = append(streamInterceptors, StreamAuthInterceptor(authService))
	serverOptions := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	}
	serverOptions = append(serverOptions, opts...)

	server := grpc.NewServer(serverOptions...)
	pb.RegisterUsageServiceServer(server, NewUsageServer(usageService, alertService, entitlementService, authorizer, limits))
	return server
}
