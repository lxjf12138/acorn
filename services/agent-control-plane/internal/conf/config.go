package conf

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	_ "github.com/go-kratos/kratos/v2/encoding/yaml"
)

type Config struct {
	Service         Service         `json:"service" yaml:"service"`
	Server          Server          `json:"server" yaml:"server"`
	Resource        Resource        `json:"resource" yaml:"resource"`
	Sandbox         Sandbox         `json:"sandbox" yaml:"sandbox"`
	SandboxPolicies SandboxPolicies `json:"sandbox_policies" yaml:"sandbox_policies"`
	Log             Log             `json:"log" yaml:"log"`
}

type Service struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

type Server struct {
	HTTP HTTP `json:"http" yaml:"http"`
	GRPC GRPC `json:"grpc" yaml:"grpc"`
}

type HTTP struct {
	Addr    string `json:"addr" yaml:"addr"`
	Timeout string `json:"timeout" yaml:"timeout"`
}

type GRPC struct {
	Addr    string `json:"addr" yaml:"addr"`
	Timeout string `json:"timeout" yaml:"timeout"`
}

type Log struct {
	Level string `json:"level" yaml:"level"`
}

type Resource struct {
	BlobRoot       string `json:"blob_root" yaml:"blob_root"`
	UploadMaxBytes int64  `json:"upload_max_bytes" yaml:"upload_max_bytes"`
}

type Sandbox struct {
	ServiceID        string `json:"service_id" yaml:"service_id"`
	GRPCAddr         string `json:"grpc_addr" yaml:"grpc_addr"`
	DefaultProfileID string `json:"default_profile_id" yaml:"default_profile_id"`
}

type SandboxPolicies struct {
	Global  SandboxPolicyConfig            `json:"global" yaml:"global"`
	Tenants map[string]SandboxPolicyConfig `json:"tenants" yaml:"tenants"`
	Users   map[string]SandboxPolicyConfig `json:"users" yaml:"users"`
}

type SandboxPolicyConfig struct {
	DefaultProfileID  string   `json:"default_profile_id" yaml:"default_profile_id"`
	AllowedProfileIDs []string `json:"allowed_profile_ids" yaml:"allowed_profile_ids"`
}

func Load(path string) (*Config, error) {
	c := config.New(config.WithSource(file.NewSource(path)))
	defer c.Close()

	if err := c.Load(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	var cfg Config
	if err := c.Scan(&cfg); err != nil {
		return nil, fmt.Errorf("scan config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Service.ID == "" {
		return fmt.Errorf("service.id is required")
	}
	if c.Service.Name == "" {
		return fmt.Errorf("service.name is required")
	}
	if c.Service.Version == "" {
		return fmt.Errorf("service.version is required")
	}
	if c.Server.HTTP.Addr == "" {
		return fmt.Errorf("server.http.addr is required")
	}
	if c.Server.GRPC.Addr == "" {
		return fmt.Errorf("server.grpc.addr is required")
	}
	if _, err := time.ParseDuration(c.Server.HTTP.Timeout); err != nil {
		return fmt.Errorf("parse server.http.timeout: %w", err)
	}
	if _, err := time.ParseDuration(c.Server.GRPC.Timeout); err != nil {
		return fmt.Errorf("parse server.grpc.timeout: %w", err)
	}
	if c.Sandbox.ServiceID == "" {
		return fmt.Errorf("sandbox.service_id is required")
	}
	if c.Sandbox.GRPCAddr == "" {
		return fmt.Errorf("sandbox.grpc_addr is required")
	}
	if c.Sandbox.DefaultProfileID == "" {
		return fmt.Errorf("sandbox.default_profile_id is required")
	}
	if c.Resource.BlobRoot == "" {
		c.Resource.BlobRoot = "/tmp/acorn/control-plane/resources"
	}
	if c.Resource.UploadMaxBytes <= 0 {
		c.Resource.UploadMaxBytes = 100 * 1024 * 1024
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	return nil
}

func (h HTTP) TimeoutDuration() time.Duration {
	return mustParseDuration(h.Timeout)
}

func (g GRPC) TimeoutDuration() time.Duration {
	return mustParseDuration(g.Timeout)
}

func mustParseDuration(value string) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		panic(err)
	}
	return d
}
