package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

// CORS returns a configured CORS middleware handler.
func CORS(origins []string) func(http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Guest-Mode"},
		AllowCredentials: true,
		MaxAge:           86400,
	})
	return c.Handler
}
