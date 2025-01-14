module dev.rubentxu.devops-platform/remote_process

go 1.23.4

require (
	dev.rubentxu.devops-platform/protos v0.0.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	google.golang.org/grpc v1.69.4
)

require (
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/protobuf v1.36.2 // indirect
)

replace dev.rubentxu.devops-platform/protos => ../protos
