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
	cfg.Service.ID = "sandbox-service-id"
	cfg.Service.Name = "Sandbox Display Name"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func validConfig() Config {
	return Config{
		Service: Service{
			ID:      "sandbox-service",
			Name:    "sandbox-service",
			Version: "dev",
		},
		Server: Server{
			HTTP: HTTP{
				Addr:    "127.0.0.1:8081",
				Timeout: "10s",
			},
			GRPC: GRPC{
				Addr:    "127.0.0.1:9081",
				Timeout: "10s",
			},
		},
	}
}
