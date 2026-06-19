package grpcadapter

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/grpc/pb"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type alertEnqueuer interface {
	EnqueueForUsageEvents(ctx context.Context, events []appalert.UsageEvent) error
}

type UsageServer struct {
	pb.UnimplementedUsageServiceServer

	service    appusage.Service
	alerts     alertEnqueuer
	authorizer appauth.Authorizer
}

func NewUsageServer(service appusage.Service, alerts alertEnqueuer, authorizer appauth.Authorizer) *UsageServer {
	return &UsageServer{service: service, alerts: alerts, authorizer: authorizer}
}

func (s *UsageServer) CreateUsage(ctx context.Context, req *pb.CreateUsageRequest) (*pb.CreateUsageResponse, error) {
	cmd, err := commandFromProto(req.GetEvent(), 0)
	if err != nil {
		return nil, serviceError(err)
	}
	if err := s.authorizeUsageCommands(ctx, []appusage.CreateCommand{cmd}); err != nil {
		return nil, serviceError(err)
	}

	event, err := s.service.Create(ctx, cmd)
	if err != nil {
		return nil, serviceError(err)
	}
	if err := s.recordIngestion(ctx, "single", 1, 0, 0); err != nil {
		return nil, serviceError(err)
	}
	s.enqueueAlerts(ctx, []appusage.Result{event})

	res, err := resultToProto(event)
	if err != nil {
		return nil, serviceError(err)
	}
	return &pb.CreateUsageResponse{Event: res}, nil
}

func (s *UsageServer) CreateUsageBulk(ctx context.Context, req *pb.CreateUsageBulkRequest) (*pb.CreateUsageBulkResponse, error) {
	commands, err := commandsFromProto(req.GetEvents())
	if err != nil {
		return nil, serviceError(err)
	}
	if err := s.authorizeUsageCommands(ctx, commands); err != nil {
		return nil, serviceError(err)
	}

	result, err := s.service.CreateBulk(ctx, req.GetIdempotencyKey(), commands)
	if err != nil {
		return nil, serviceError(err)
	}
	if err := s.recordBulkIngestion(ctx, "bulk", result); err != nil {
		return nil, serviceError(err)
	}
	s.enqueueAlerts(ctx, result.Accepted)

	res, err := bulkResponseFromResult(result)
	if err != nil {
		return nil, serviceError(err)
	}
	return res, nil
}

func (s *UsageServer) StreamUsage(stream grpc.ClientStreamingServer[pb.StreamUsageRequest, pb.StreamUsageResponse]) error {
	commands := []appusage.CreateCommand{}
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return s.closeUsageStream(stream, commands)
		}
		if err != nil {
			return serviceError(err)
		}
		cmd, err := commandFromProto(req.GetEvent(), len(commands))
		if err != nil {
			return serviceError(err)
		}
		commands = append(commands, cmd)
		if len(commands) > appusage.MaxBulkEvents {
			return serviceError(fmt.Errorf("%w: bulk usage event limit is %d", domain.ErrInvalidInput, appusage.MaxBulkEvents))
		}
	}
}

func (s *UsageServer) closeUsageStream(stream grpc.ClientStreamingServer[pb.StreamUsageRequest, pb.StreamUsageResponse], commands []appusage.CreateCommand) error {
	if err := s.authorizeUsageCommands(stream.Context(), commands); err != nil {
		return serviceError(err)
	}

	result, err := s.service.CreateBulk(stream.Context(), idempotencyKeyFromMetadata(stream.Context()), commands)
	if err != nil {
		return serviceError(err)
	}
	if err := s.recordBulkIngestion(stream.Context(), "stream", result); err != nil {
		return serviceError(err)
	}
	s.enqueueAlerts(stream.Context(), result.Accepted)

	res, err := streamResponseFromResult(result)
	if err != nil {
		return serviceError(err)
	}
	return stream.SendAndClose(res)
}

func (s *UsageServer) recordBulkIngestion(ctx context.Context, kind string, result appusage.BulkResult) error {
	return s.recordIngestion(ctx, kind, len(result.Accepted), len(result.Duplicates), len(result.Failed))
}

func (s *UsageServer) recordIngestion(ctx context.Context, kind string, accepted int, duplicates int, failed int) error {
	_, err := s.service.RecordIngestion(ctx, appusage.IngestionCommand{
		Kind:       kind,
		Accepted:   accepted,
		Duplicates: duplicates,
		Failed:     failed,
	})
	return err
}

func (s *UsageServer) enqueueAlerts(ctx context.Context, events []appusage.Result) {
	if s.alerts == nil || len(events) == 0 {
		return
	}
	alertEvents := make([]appalert.UsageEvent, 0, len(events))
	for _, event := range events {
		alertEvents = append(alertEvents, appalert.UsageEvent{
			Subject:  event.Subject,
			Meter:    event.MeterName,
			Metadata: event.Metadata,
		})
	}
	if err := s.alerts.EnqueueForUsageEvents(ctx, alertEvents); err != nil {
		log.Printf("alert enqueue failed: %v", err)
	}
}

func (s *UsageServer) authorizeUsageCommands(ctx context.Context, commands []appusage.CreateCommand) error {
	if s.authorizer == nil {
		return nil
	}
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}
	checked := map[string]struct{}{}
	for _, command := range commands {
		key := command.MeterName + "\x00" + command.Subject
		if _, ok := checked[key]; ok {
			continue
		}
		checked[key] = struct{}{}
		if err := s.authorizer.Can(ctx, principal, appauth.ActionUsageWrite, appauth.Resource{
			Type:    appauth.ResourceUsage,
			Meter:   command.MeterName,
			Subject: command.Subject,
		}); err != nil {
			return err
		}
	}
	return nil
}

func commandsFromProto(events []*pb.UsageEventInput) ([]appusage.CreateCommand, error) {
	commands := make([]appusage.CreateCommand, 0, len(events))
	for index, event := range events {
		cmd, err := commandFromProto(event, index)
		if err != nil {
			return nil, err
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}

func commandFromProto(event *pb.UsageEventInput, index int) (appusage.CreateCommand, error) {
	if event == nil {
		return appusage.CreateCommand{}, fmt.Errorf("%w: usage event is required", domain.ErrInvalidInput)
	}

	eventTime := time.Time{}
	if event.Timestamp != nil {
		if err := event.Timestamp.CheckValid(); err != nil {
			return appusage.CreateCommand{}, fmt.Errorf("%w: invalid timestamp: %v", domain.ErrInvalidInput, err)
		}
		eventTime = event.Timestamp.AsTime()
	}

	return appusage.CreateCommand{
		Index:          index,
		IdempotencyKey: event.GetIdempotencyKey(),
		Subject:        event.GetSubject(),
		MeterName:      event.GetMeter(),
		Quantity:       event.GetQuantity(),
		EventTime:      eventTime,
		Metadata:       metadataFromProto(event.GetMetadata()),
	}, nil
}

func metadataFromProto(metadata map[string]*structpb.Value) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	values := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if value == nil {
			values[key] = nil
			continue
		}
		values[key] = value.AsInterface()
	}
	return values
}

func resultToProto(event appusage.Result) (*pb.UsageEvent, error) {
	metadata, err := metadataToProto(event.Metadata)
	if err != nil {
		return nil, err
	}

	return &pb.UsageEvent{
		Id:             event.ID,
		IdempotencyKey: event.IdempotencyKey,
		Subject:        event.Subject,
		Meter:          event.MeterName,
		Quantity:       event.Quantity,
		Timestamp:      timestamppb.New(event.EventTime),
		ReceivedAt:     timestamppb.New(event.ReceivedAt),
		Metadata:       metadata,
	}, nil
}

func metadataToProto(metadata map[string]any) (map[string]*structpb.Value, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	values := make(map[string]*structpb.Value, len(metadata))
	for key, value := range metadata {
		protoValue, err := structpb.NewValue(value)
		if err != nil {
			return nil, err
		}
		values[key] = protoValue
	}
	return values, nil
}

func bulkResponseFromResult(result appusage.BulkResult) (*pb.CreateUsageBulkResponse, error) {
	accepted, err := resultsToProto(result.Accepted)
	if err != nil {
		return nil, err
	}
	duplicates, err := resultsToProto(result.Duplicates)
	if err != nil {
		return nil, err
	}

	return &pb.CreateUsageBulkResponse{
		AcceptedCount:  int32(len(accepted)),
		DuplicateCount: int32(len(duplicates)),
		FailedCount:    int32(len(result.Failed)),
		Accepted:       accepted,
		Duplicates:     duplicates,
		Failed:         failuresToProto(result.Failed),
	}, nil
}

func streamResponseFromResult(result appusage.BulkResult) (*pb.StreamUsageResponse, error) {
	accepted, err := resultsToProto(result.Accepted)
	if err != nil {
		return nil, err
	}
	duplicates, err := resultsToProto(result.Duplicates)
	if err != nil {
		return nil, err
	}

	return &pb.StreamUsageResponse{
		AcceptedCount:  int32(len(accepted)),
		DuplicateCount: int32(len(duplicates)),
		FailedCount:    int32(len(result.Failed)),
		Accepted:       accepted,
		Duplicates:     duplicates,
		Failed:         failuresToProto(result.Failed),
	}, nil
}

func resultsToProto(events []appusage.Result) ([]*pb.UsageEvent, error) {
	results := make([]*pb.UsageEvent, 0, len(events))
	for _, event := range events {
		result, err := resultToProto(event)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func failuresToProto(failures []appusage.BulkFailureResult) []*pb.BulkFailure {
	results := make([]*pb.BulkFailure, 0, len(failures))
	for _, failure := range failures {
		results = append(results, &pb.BulkFailure{
			Index:   int32(failure.Index),
			Code:    failure.Code,
			Message: failure.Message,
		})
	}
	return results
}
