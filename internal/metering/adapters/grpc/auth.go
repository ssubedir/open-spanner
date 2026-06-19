package grpcadapter

import (
	"context"
	"strings"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type userContextKey struct{}

func UnaryAuthInterceptor(service appauth.Service) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		nextCtx, err := authenticateContext(ctx, service)
		if err != nil {
			return nil, serviceError(err)
		}
		return handler(nextCtx, req)
	}
}

func StreamAuthInterceptor(service appauth.Service) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := authenticateContext(stream.Context(), service)
		if err != nil {
			return serviceError(err)
		}
		return handler(srv, serverStreamWithContext{ServerStream: stream, ctx: ctx})
	}
}

func UserFromContext(ctx context.Context) (appauth.UserResult, bool) {
	user, ok := ctx.Value(userContextKey{}).(appauth.UserResult)
	return user, ok
}

func authenticateContext(ctx context.Context, service appauth.Service) (context.Context, error) {
	token := apiKeyFromMetadata(ctx)
	if token == "" {
		return ctx, domain.ErrUnauthorized
	}

	user, err := service.AuthenticateAPIKey(ctx, token)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, userContextKey{}, user), nil
}

func apiKeyFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	for _, key := range []string{"x-open-spanner-api-key", "x-api-key"} {
		if token := firstMetadataValue(md, key); token != "" {
			return token
		}
	}

	auth := firstMetadataValue(md, "authorization")
	if auth == "" {
		return ""
	}
	if token, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(token)
	}
	if token, ok := strings.CutPrefix(auth, "bearer "); ok {
		return strings.TrimSpace(token)
	}
	return ""
}

func idempotencyKeyFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range []string{"idempotency-key", "x-open-spanner-idempotency-key"} {
		if value := firstMetadataValue(md, key); value != "" {
			return value
		}
	}
	return ""
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s serverStreamWithContext) Context() context.Context {
	return s.ctx
}
