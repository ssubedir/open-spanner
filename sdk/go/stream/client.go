package stream

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.UsageServiceClient
	apiKey string
}

type Option func(*clientConfig)

type clientConfig struct {
	dialOptions           []grpc.DialOption
	useDefaultCredentials bool
}

type Event struct {
	IdempotencyKey string
	Subject        string
	Meter          string
	Quantity       float64
	Timestamp      time.Time
	Metadata       map[string]any
}

type RecordedEvent struct {
	ID             string
	IdempotencyKey string
	Subject        string
	Meter          string
	Quantity       float64
	Timestamp      time.Time
	ReceivedAt     time.Time
	Metadata       map[string]any
}

type Failure struct {
	Index   int
	Code    string
	Message string
}

type BulkResult struct {
	AcceptedCount  int
	DuplicateCount int
	FailedCount    int
	Accepted       []RecordedEvent
	Duplicates     []RecordedEvent
	Failed         []Failure
}

func NewClient(addr string, apiKey string, options ...Option) (*Client, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, errors.New("grpc address is required")
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("api key is required")
	}

	cfg := clientConfig{useDefaultCredentials: true}
	for _, option := range options {
		option(&cfg)
	}
	dialOptions := make([]grpc.DialOption, 0, len(cfg.dialOptions)+1)
	if cfg.useDefaultCredentials {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	dialOptions = append(dialOptions, cfg.dialOptions...)

	conn, err := grpc.NewClient(addr, dialOptions...)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: pb.NewUsageServiceClient(conn),
		apiKey: apiKey,
	}, nil
}

func WithDialOptions(options ...grpc.DialOption) Option {
	return func(cfg *clientConfig) {
		cfg.dialOptions = append(cfg.dialOptions, options...)
	}
}

func WithTransportCredentials(creds credentials.TransportCredentials) Option {
	return func(cfg *clientConfig) {
		cfg.useDefaultCredentials = false
		cfg.dialOptions = append(cfg.dialOptions, grpc.WithTransportCredentials(creds))
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Track(ctx context.Context, event Event) (RecordedEvent, error) {
	input, err := eventInput(event)
	if err != nil {
		return RecordedEvent{}, err
	}

	res, err := c.client.CreateUsage(c.authContext(ctx), &pb.CreateUsageRequest{Event: input})
	if err != nil {
		return RecordedEvent{}, err
	}
	return recordedEvent(res.GetEvent())
}

func (c *Client) TrackBulk(ctx context.Context, idempotencyKey string, events []Event) (BulkResult, error) {
	inputs, err := eventInputs(events)
	if err != nil {
		return BulkResult{}, err
	}

	res, err := c.client.CreateUsageBulk(c.authContext(ctx), &pb.CreateUsageBulkRequest{
		IdempotencyKey: idempotencyKey,
		Events:         inputs,
	})
	if err != nil {
		return BulkResult{}, err
	}
	return bulkResult(res.GetAcceptedCount(), res.GetDuplicateCount(), res.GetFailedCount(), res.GetAccepted(), res.GetDuplicates(), res.GetFailed())
}

func (c *Client) Stream(ctx context.Context, idempotencyKey string) (*Stream, error) {
	streamCtx := metadata.AppendToOutgoingContext(c.authContext(ctx), "idempotency-key", idempotencyKey)
	stream, err := c.client.StreamUsage(streamCtx)
	if err != nil {
		return nil, err
	}
	return &Stream{stream: stream}, nil
}

func (c *Client) authContext(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.apiKey)
}

type Stream struct {
	stream pb.UsageService_StreamUsageClient
}

func (s *Stream) Track(event Event) error {
	input, err := eventInput(event)
	if err != nil {
		return err
	}
	return s.stream.Send(&pb.StreamUsageRequest{Event: input})
}

func (s *Stream) Close() (BulkResult, error) {
	res, err := s.stream.CloseAndRecv()
	if err != nil {
		return BulkResult{}, err
	}
	return bulkResult(res.GetAcceptedCount(), res.GetDuplicateCount(), res.GetFailedCount(), res.GetAccepted(), res.GetDuplicates(), res.GetFailed())
}

func eventInputs(events []Event) ([]*pb.UsageEventInput, error) {
	inputs := make([]*pb.UsageEventInput, 0, len(events))
	for _, event := range events {
		input, err := eventInput(event)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func eventInput(event Event) (*pb.UsageEventInput, error) {
	metadata, err := metadataValues(event.Metadata)
	if err != nil {
		return nil, err
	}
	return &pb.UsageEventInput{
		IdempotencyKey: event.IdempotencyKey,
		Subject:        event.Subject,
		Meter:          event.Meter,
		Quantity:       event.Quantity,
		Timestamp:      timestamppb.New(event.Timestamp),
		Metadata:       metadata,
	}, nil
}

func metadataValues(fields map[string]any) (map[string]*structpb.Value, error) {
	values := make(map[string]*structpb.Value, len(fields))
	for key, value := range fields {
		protoValue, err := structpb.NewValue(value)
		if err != nil {
			return nil, err
		}
		values[key] = protoValue
	}
	return values, nil
}

func bulkResult(acceptedCount int32, duplicateCount int32, failedCount int32, accepted []*pb.UsageEvent, duplicates []*pb.UsageEvent, failed []*pb.BulkFailure) (BulkResult, error) {
	acceptedEvents, err := recordedEvents(accepted)
	if err != nil {
		return BulkResult{}, err
	}
	duplicateEvents, err := recordedEvents(duplicates)
	if err != nil {
		return BulkResult{}, err
	}
	return BulkResult{
		AcceptedCount:  int(acceptedCount),
		DuplicateCount: int(duplicateCount),
		FailedCount:    int(failedCount),
		Accepted:       acceptedEvents,
		Duplicates:     duplicateEvents,
		Failed:         failures(failed),
	}, nil
}

func recordedEvents(events []*pb.UsageEvent) ([]RecordedEvent, error) {
	results := make([]RecordedEvent, 0, len(events))
	for _, event := range events {
		result, err := recordedEvent(event)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func recordedEvent(event *pb.UsageEvent) (RecordedEvent, error) {
	if event == nil {
		return RecordedEvent{}, nil
	}
	metadata := make(map[string]any, len(event.GetMetadata()))
	for key, value := range event.GetMetadata() {
		if value == nil {
			metadata[key] = nil
			continue
		}
		metadata[key] = value.AsInterface()
	}
	return RecordedEvent{
		ID:             event.GetId(),
		IdempotencyKey: event.GetIdempotencyKey(),
		Subject:        event.GetSubject(),
		Meter:          event.GetMeter(),
		Quantity:       event.GetQuantity(),
		Timestamp:      timestampTime(event.GetTimestamp()),
		ReceivedAt:     timestampTime(event.GetReceivedAt()),
		Metadata:       metadata,
	}, nil
}

func failures(items []*pb.BulkFailure) []Failure {
	results := make([]Failure, 0, len(items))
	for _, item := range items {
		results = append(results, Failure{
			Index:   int(item.GetIndex()),
			Code:    item.GetCode(),
			Message: item.GetMessage(),
		})
	}
	return results
}

func timestampTime(value *timestamppb.Timestamp) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.AsTime()
}
