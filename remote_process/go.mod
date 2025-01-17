module dev.rubentxu.devops-platform/remote_process

go 1.23.4

require (
	dev.rubentxu.devops-platform/protos v0.0.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/shirou/gopsutil/v3 v3.24.5
	google.golang.org/grpc v1.69.4
)

require (
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.9.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/protobuf v1.36.3 // indirect
)


replace (
    google.golang.org/genproto => google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f
    google.golang.org/genproto/googleapis/rpc => google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f
)
replace dev.rubentxu.devops-platform/protos => ../protos
