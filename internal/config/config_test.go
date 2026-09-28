package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "a-secret-at-least-thirty-two-bytes-long")
	t.Setenv("STAFF_PROVISIONING_KEY", "a-key-at-least-thirty-two-bytes-long")
	t.Setenv("JWT_TTL", "1h")
	t.Setenv("HIS_TIMEOUT", "5s")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ key, value string }{{"DATABASE_URL", ""}, {"JWT_SECRET", "short"}, {"STAFF_PROVISIONING_KEY", "short"}, {"JWT_TTL", "bad"}, {"JWT_TTL", "0s"}, {"JWT_TTL", "25h"}, {"HIS_TIMEOUT", "16s"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
