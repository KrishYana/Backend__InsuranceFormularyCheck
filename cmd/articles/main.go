package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/kyanaman/formularycheck/api_integrations/articles"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/article"

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

	// Run auto-migration
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Step 1: FDA Drug Safety Communications
	fdaIngestor := articles.NewFDAIngestor(client)
	if err := fdaIngestor.Run(ctx); err != nil {
		log.Printf("FDA ingestion failed: %v", err)
	}

	// Step 2: PubMed articles
	ncbiKey := os.Getenv("NCBI_API_KEY")
	pubmedIngestor := articles.NewPubMedIngestor(client, ncbiKey)
	if err := pubmedIngestor.Run(ctx); err != nil {
		log.Printf("PubMed ingestion failed: %v", err)
	}

	// Step 3: Deactivate articles older than 90 days
	cutoff := time.Now().AddDate(0, 0, -90)
	count, err := client.Article.Update().
		Where(article.PublishedAtLT(cutoff), article.IsActive(true)).
		SetIsActive(false).
		Save(ctx)
	if err != nil {
		log.Printf("Failed to deactivate old articles: %v", err)
	} else {
		log.Printf("Deactivated %d articles older than 90 days", count)
	}

	log.Println("Article ingestion complete.")
}
