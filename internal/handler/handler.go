package handler

import "github.com/kyanaman/formularycheck/ent"

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db *ent.Client
}

// New creates a new Handler with the given Ent client.
func New(db *ent.Client) *Handler {
	return &Handler{db: db}
}
