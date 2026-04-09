package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/kyanaman/formularycheck/api_integrations/cmspuf"
	"github.com/kyanaman/formularycheck/api_integrations/openfda"
	"github.com/kyanaman/formularycheck/api_integrations/qhp"
	"github.com/kyanaman/formularycheck/api_integrations/rxnorm"
	"github.com/kyanaman/formularycheck/ent"

	_ "github.com/lib/pq"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://formulary:formulary@localhost:5432/formularycheck?sslmode=disable"
	}

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	ctx := context.Background()

	// Run auto-migration to create/update schema
	if err := client.Schema.Create(ctx); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database schema ready")

	// Step 1: RxNorm — populate drugs table
	rxnormIngestor := rxnorm.NewIngestor(client)
	if err := rxnormIngestor.Run(ctx); err != nil {
		slog.Error("RxNorm ingestion failed", "error", err)
		os.Exit(1)
	}

	// Step 2: openFDA — populate NDC-to-drug mappings
	fdaIngestor := openfda.NewIngestor(client)
	if err := fdaIngestor.Run(ctx); err != nil {
		slog.Error("openFDA ingestion failed", "error", err)
		os.Exit(1)
	}

	// Step 3: CMS Part D PUF — populate plans and formulary entries
	pufURL := os.Getenv("CMS_PUF_URL")
	if pufURL == "" {
		slog.Info("SKIP: CMS_PUF_URL not set — set it to the SPUF ZIP download URL to ingest Part D data")
	} else {
		pufIngestor := cmspuf.NewIngestor(client, pufURL)
		if err := pufIngestor.Run(ctx); err != nil {
			slog.Error("CMS PUF ingestion failed", "error", err)
			os.Exit(1)
		}
	}

	// Step 4: QHP — ACA Marketplace plans and formulary entries
	qhpURL := os.Getenv("QHP_MRPUF_URL")
	if qhpURL == "" {
		slog.Info("SKIP: QHP_MRPUF_URL not set — set it to the MR-PUF CSV URL to ingest QHP data")
	} else {
		qhpIngestor := qhp.NewIngestor(client, qhpURL)
		if err := qhpIngestor.Run(ctx); err != nil {
			slog.Error("QHP ingestion failed", "error", err)
			os.Exit(1)
		}
	}
}
