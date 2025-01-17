module dev.rubentxu.devops-platform/worker

go 1.23.4

require (
	dev.rubentxu.devops-platform/protos v0.0.0-00010101000000-000000000000
	github.com/gorilla/websocket v1.5.3
	google.golang.org/grpc v1.69.4
)

require (
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/protobuf v1.36.3 // indirect
)

replace dev.rubentxu.devops-platform/protos => ../protos
