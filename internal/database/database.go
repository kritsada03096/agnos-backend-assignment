package database

import (
	"context"
	"fmt"
	"sort"
	"time"

	"agnos-backend-assignment/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL")
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}
func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(2026092801)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", entry.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		body, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Seed only registers hospitals. Patient records are imported through the HIS client.
func Seed(ctx context.Context, p *pgxpool.Pool, aURL, bURL string) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	for _, h := range []struct{ code, name, url string }{{"hospital-a", "Hospital A", aURL}, {"hospital-b", "Hospital B (demo)", bURL}} {
		_, err = tx.Exec(ctx, `INSERT INTO hospitals(code,name,api_url) VALUES($1,$2,$3)
		ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,api_url=EXCLUDED.api_url,updated_at=now()`, h.code, h.name, h.url)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
