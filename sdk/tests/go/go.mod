module github.com/ssubedir/open-spanner/sdk/tests/go

go 1.25.5

require github.com/ssubedir/open-spanner/sdk/go v0.0.0

require (
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250818200422-3122310a409c // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
)

replace github.com/ssubedir/open-spanner/sdk/go => ../../go
