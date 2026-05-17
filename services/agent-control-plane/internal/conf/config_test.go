package conf

import "testing"

func TestConfigValidateRequiresServiceID(t *testing.T) {
	cfg := validConfig()
	cfg.Service.ID = ""
	if err := cfg.Validate(); err == nil || err.Error() != "service.id is required" {
		t.Fatalf("expected service.id validation error, got %v", err)
	}
}

func TestConfigValidateAcceptsServiceIDAndName(t *testing.T) {
	cfg := validConfig()
	cfg.Service.ID = "agent-control-plane-id"
	cfg.Service.Name = "Agent Control Plane"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func validConfig() Config {
	return Config{
		Service: Service{
			ID:      "agent-control-plane",
			Name:    "agent-control-plane",
			Version: "dev",
		},
		Server: Server{
			HTTP: HTTP{
				Addr:    "127.0.0.1:8080",
				Timeout: "10s",
			},
			GRPC: GRPC{
				Addr:    "127.0.0.1:9080",
				Timeout: "10s",
			},
		},
		Sandbox: Sandbox{
			ServiceID:        "sandbox-service",
			GRPCAddr:         "127.0.0.1:9081",
			DefaultProfileID: "local-process-dev",
		},
		Resource: Resource{
			BlobRoot:       "/tmp/acorn/control-plane/resources",
			UploadMaxBytes: 100 * 1024 * 1024,
		},
	}
}
