package trace

import "fmt"

type ExporterKind string

const (
	ExporterKindGrpc ExporterKind = "grpc"
	ExporterKindHttp ExporterKind = "http"
)

type Config struct {
	Name         string       `json:"name"         yaml:"name"         toml:"name"`
	Endpoint     string       `json:"endpoint"     yaml:"endpoint"     toml:"endpoint"`
	Sampler      float64      `json:"sampler"      yaml:"sampler"      toml:"sampler"`
	Exporter     ExporterKind `json:"exporter"     yaml:"exporter"     toml:"exporter"`
	OtlpHttpPath string       `json:"otlpHttpPath" yaml:"otlpHttpPath" toml:"otlpHttpPath"`
}

func (c *Config) normalize() error {
	if c.Sampler <= 0 {
		c.Sampler = 1.0
	}

	if c.Exporter == "" {
		c.Exporter = ExporterKindGrpc
	}
	
	if c.Endpoint == "" {
		return fmt.Errorf("[trace] no endpoint configured")
	}

	return nil
}
