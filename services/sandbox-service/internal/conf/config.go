package conf

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	_ "github.com/go-kratos/kratos/v2/encoding/yaml"
)

type Config struct {
	Service Service `json:"service" yaml:"service"`
	Server  Server  `json:"server" yaml:"server"`
	Log     Log     `json:"log" yaml:"log"`
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
	Addr          string `json:"addr" yaml:"addr"`
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`
	Timeout       string `json:"timeout" yaml:"timeout"`
}

type GRPC struct {
	Addr          string `json:"addr" yaml:"addr"`
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`
	Timeout       string `json:"timeout" yaml:"timeout"`
}

type Log struct {
	Level string `json:"level" yaml:"level"`
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
	if c.Server.HTTP.AdvertiseAddr == "" {
		c.Server.HTTP.AdvertiseAddr = c.Server.HTTP.Addr
	}
	if c.Server.GRPC.Addr == "" {
		return fmt.Errorf("server.grpc.addr is required")
	}
	if c.Server.GRPC.AdvertiseAddr == "" {
		c.Server.GRPC.AdvertiseAddr = c.Server.GRPC.Addr
	}
	if _, err := time.ParseDuration(c.Server.HTTP.Timeout); err != nil {
		return fmt.Errorf("parse server.http.timeout: %w", err)
	}
	if _, err := time.ParseDuration(c.Server.GRPC.Timeout); err != nil {
		return fmt.Errorf("parse server.grpc.timeout: %w", err)
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
