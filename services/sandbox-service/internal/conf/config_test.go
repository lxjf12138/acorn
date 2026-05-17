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

func TestConfigValidateDefaultsWorkspaceRoot(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if cfg.Sandbox.WorkspaceRoot != "/tmp/acorn/sandbox/workspaces" {
		t.Fatalf("unexpected workspace root: %q", cfg.Sandbox.WorkspaceRoot)
	}
	if cfg.Sandbox.ResourceBlobRoot != "/tmp/acorn/sandbox/resources" {
		t.Fatalf("unexpected resource blob root: %q", cfg.Sandbox.ResourceBlobRoot)
	}
	if cfg.Sandbox.LocalProcess.DefaultTimeoutSeconds != 30 ||
		cfg.Sandbox.LocalProcess.MaxTimeoutSeconds != 120 ||
		cfg.Sandbox.LocalProcess.MaxStdoutBytes != 1024*1024 ||
		cfg.Sandbox.LocalProcess.MaxStderrBytes != 1024*1024 ||
		cfg.Sandbox.LocalProcess.Enabled {
		t.Fatalf("unexpected local process defaults: %+v", cfg.Sandbox.LocalProcess)
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
