module github.com/lxjf12138/acorn/services/sandbox-service

go 1.26

require (
	github.com/go-kratos/kratos/v2 v2.9.2
	github.com/lxjf12138/acorn/packages/api v0.0.0
	github.com/lxjf12138/acorn/packages/servicekit v0.0.0
)

require (
	dario.cat/mergo v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-kratos/aegis v0.2.0 // indirect
	github.com/go-playground/form/v4 v4.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/grpc v1.67.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/lxjf12138/acorn/packages/api => ../../packages/api

replace github.com/lxjf12138/acorn/packages/servicekit => ../../packages/servicekit
