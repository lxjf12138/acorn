package observability

type Config struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Environment string `json:"environment" yaml:"environment"`

	Tracing TracingConfig `json:"tracing" yaml:"tracing"`
	Metrics MetricsConfig `json:"metrics" yaml:"metrics"`
}

type TracingConfig struct {
	Enabled      bool    `json:"enabled" yaml:"enabled"`
	Exporter     string  `json:"exporter" yaml:"exporter"`
	OTLPEndpoint string  `json:"otlp_endpoint" yaml:"otlp_endpoint"`
	SampleRatio  float64 `json:"sample_ratio" yaml:"sample_ratio"`
}

type MetricsConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	Exporter     string `json:"exporter" yaml:"exporter"`
	OTLPEndpoint string `json:"otlp_endpoint" yaml:"otlp_endpoint"`
}

const (
	ExporterNoop   = "noop"
	ExporterStdout = "stdout"
	ExporterOTLP   = "otlp"
)

func (c Config) normalized() Config {
	if c.Environment == "" {
		c.Environment = "dev"
	}
	if c.Tracing.Exporter == "" {
		c.Tracing.Exporter = ExporterNoop
	}
	if c.Tracing.SampleRatio < 0 {
		c.Tracing.SampleRatio = 0
	}
	if c.Tracing.SampleRatio > 1 {
		c.Tracing.SampleRatio = 1
	}
	if c.Metrics.Exporter == "" {
		c.Metrics.Exporter = ExporterNoop
	}
	return c
}
