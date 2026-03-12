package config

import (
	"fmt"
	"os"
	"time"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/index"
	"gopkg.in/yaml.v3"
)

// Config is the top-level server configuration.
type Config struct {
	Server   ServerConfig            `yaml:"server"`
	Storage  StorageConfig           `yaml:"storage"`
	Entities map[string]EntityConfig `yaml:"entities"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr string `yaml:"addr"`
}

// StorageConfig holds storage settings.
type StorageConfig struct {
	DataDir string `yaml:"data_dir"`
}

// EntityConfig defines an entity type and its indexes.
type EntityConfig struct {
	Retention string        `yaml:"retention"`  // e.g., "168h", "720h" - empty means keep forever
	NoReindex bool          `yaml:"no_reindex"` // If true, skip automatic reindexing on config changes
	Indexes   []IndexConfig `yaml:"indexes"`
}

// IndexConfig defines a single field index.
type IndexConfig struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"` // int, float, bool, string, string_array, int_array, float_array
}

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set defaults
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Storage.DataDir == "" {
		cfg.Storage.DataDir = "./data"
	}

	return &cfg, nil
}

// BuildRegistry creates an index registry from the config.
func (c *Config) BuildRegistry() (*index.Registry, error) {
	registry := index.NewRegistry()

	for entityType, entityCfg := range c.Entities {
		indexes := make([]index.FieldIndex, len(entityCfg.Indexes))
		for i, idx := range entityCfg.Indexes {
			fieldType, err := parseFieldType(idx.Type)
			if err != nil {
				return nil, fmt.Errorf("entity %q index %q: %w", entityType, idx.Name, err)
			}
			path, err := entity.ParsePath(idx.Name)
			if err != nil {
				return nil, fmt.Errorf("entity %q invalid field path %q: %w", entityType, idx.Name, err)
			}
			indexes[i] = index.FieldIndex{
				Name: idx.Name,
				Type: fieldType,
				Path: path,
			}
		}

		var ttl time.Duration
		if entityCfg.Retention != "" {
			var err error
			ttl, err = time.ParseDuration(entityCfg.Retention)
			if err != nil {
				return nil, fmt.Errorf("entity %q invalid retention %q: %w", entityType, entityCfg.Retention, err)
			}
		}

		registry.Register(entityType, &index.EntityTypeConfig{
			Indexes:   indexes,
			TTL:       ttl,
			NoReindex: entityCfg.NoReindex,
		})
	}

	return registry, nil
}

func parseFieldType(s string) (index.FieldType, error) {
	switch s {
	case "int", "int64":
		return index.FieldTypeInt, nil
	case "float", "float64":
		return index.FieldTypeFloat, nil
	case "bool":
		return index.FieldTypeBool, nil
	case "string":
		return index.FieldTypeString, nil
	case "string_array", "[]string":
		return index.FieldTypeStringArray, nil
	case "int_array", "[]int", "[]int64":
		return index.FieldTypeIntArray, nil
	case "float_array", "[]float", "[]float64":
		return index.FieldTypeFloatArray, nil
	default:
		return 0, fmt.Errorf("unknown field type: %s", s)
	}
}
