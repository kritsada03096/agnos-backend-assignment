package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Address, DatabaseURL, JWTSecret, ProvisioningKey string
	TokenTTL, HISTimeout                             time.Duration
}

func Load() (Config, error) {
	c := Config{Address: ":8080", DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: os.Getenv("JWT_SECRET"), ProvisioningKey: os.Getenv("STAFF_PROVISIONING_KEY"), TokenTTL: time.Hour, HISTimeout: 5 * time.Second}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		c.Address = v
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}
	if len(c.ProvisioningKey) < 32 {
		return c, fmt.Errorf("STAFF_PROVISIONING_KEY must be at least 32 bytes")
	}
	for name, target := range map[string]*time.Duration{"JWT_TTL": &c.TokenTTL, "HIS_TIMEOUT": &c.HISTimeout} {
		if value := os.Getenv(name); value != "" {
			d, err := time.ParseDuration(value)
			if err != nil || d <= 0 {
				return c, fmt.Errorf("%s must be a positive duration", name)
			}
			*target = d
		}
	}
	if c.TokenTTL > 24*time.Hour || c.HISTimeout > 15*time.Second {
		return c, fmt.Errorf("JWT_TTL must be <=24h and HIS_TIMEOUT <=15s")
	}
	return c, nil
}
