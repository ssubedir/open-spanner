package grpcadapter

import (
	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/grpc/pb"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"google.golang.org/grpc"
)

func NewServer(usageService appusage.Service, alertService appalert.Service, authService appauth.Service, opts ...grpc.ServerOption) *grpc.Server {
	serverOptions := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(UnaryAuthInterceptor(authService)),
		grpc.ChainStreamInterceptor(StreamAuthInterceptor(authService)),
	}
	serverOptions = append(serverOptions, opts...)

	server := grpc.NewServer(serverOptions...)
	pb.RegisterUsageServiceServer(server, NewUsageServer(usageService, alertService))
	return server
}
