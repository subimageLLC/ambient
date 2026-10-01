package cfg

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	RotationSecs int      `toml:"rotation_secs"`
	Screens      []string `toml:"screens"`
	Glyphs       string   `toml:"glyphs"`
}

func Default() Config {
	return Config{
		RotationSecs: 60,
		Glyphs:       "full",
	}
}

func (c *Config) Validate() error {
	if c.RotationSecs < 5 {
		c.RotationSecs = 5
	}
	switch c.Glyphs {
	case "full", "ascii":
	default:
		c.Glyphs = "full"
	}
	return nil
}

func Dir() string {
	if d := os.Getenv("AMBIENT_CONFIG_DIR"); d != "" {
		return d
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "ambient-config"
	}
	return filepath.Join(h, ".config", "ambient")
}

func DataDir() string {
	if d := os.Getenv("AMBIENT_DATA_DIR"); d != "" {
		return d
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "ambient-data"
	}
	return filepath.Join(h, ".local", "share", "ambient")
}

func CacheDir() string {
	if d := os.Getenv("AMBIENT_CACHE_DIR"); d != "" {
		return d
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "ambient-cache"
	}
	return filepath.Join(h, ".cache", "ambient")
}

func Path() string {
	return filepath.Join(Dir(), "ambient.toml")
}

func Load() (Config, error) {
	c := Default()
	b, err := os.ReadFile(Path())
	if err == nil {
		if err := toml.Unmarshal(b, &c); err != nil {
			return c, err
		}
	}
	c.Validate()
	return c, nil
}
