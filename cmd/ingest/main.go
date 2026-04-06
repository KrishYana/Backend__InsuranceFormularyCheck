package main

import (
	"context"
	"log"
	"os"

	"github.com/kyanaman/formularycheck/api_integrations/cmspuf"
	"github.com/kyanaman/formularycheck/api_integrations/openfda"
	"github.com/kyanaman/formularycheck/api_integrations/qhp"
	"github.com/kyanaman/formularycheck/api_integrations/rxnorm"
	"github.com/kyanaman/formularycheck/ent"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://formulary:formulary@localhost:5432/formularycheck?sslmode=disable"
	}

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Run auto-migration to create/update schema
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database schema ready.")

	// Step 1: RxNorm — populate drugs table
	rxnormIngestor := rxnorm.NewIngestor(client)
	if err := rxnormIngestor.Run(ctx); err != nil {
		log.Fatalf("RxNorm ingestion failed: %v", err)
	}

	// Step 2: openFDA — populate NDC-to-drug mappings
	fdaIngestor := openfda.NewIngestor(client)
	if err := fdaIngestor.Run(ctx); err != nil {
		log.Fatalf("openFDA ingestion failed: %v", err)
	}

	// Step 3: CMS Part D PUF — populate plans and formulary entries
	pufURL := os.Getenv("CMS_PUF_URL")
	if pufURL == "" {
		log.Println("SKIP: CMS_PUF_URL not set. Set it to the SPUF ZIP download URL to ingest Part D data.")
	} else {
		pufIngestor := cmspuf.NewIngestor(client, pufURL)
		if err := pufIngestor.Run(ctx); err != nil {
			log.Fatalf("CMS PUF ingestion failed: %v", err)
		}
	}

	// Step 4: QHP — ACA Marketplace plans and formulary entries
	qhpURL := os.Getenv("QHP_MRPUF_URL")
	if qhpURL == "" {
		log.Println("SKIP: QHP_MRPUF_URL not set. Set it to the MR-PUF CSV URL to ingest QHP data.")
	} else {
		qhpIngestor := qhp.NewIngestor(client, qhpURL)
		if err := qhpIngestor.Run(ctx); err != nil {
			log.Fatalf("QHP ingestion failed: %v", err)
		}
	}
}
