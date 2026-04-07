package server

import "os"

// Config holds all server configuration loaded from environment variables.
type Config struct {
	Port              string
	DatabaseURL       string
	SupabaseURL       string
	SupabaseJWTSecret string
	SupabaseAnonKey   string
	AppEnv            string
	CORSOrigins       []string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://formulary:formulary@localhost:5432/formularycheck?sslmode=disable"
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}

	return Config{
		Port:              port,
		DatabaseURL:       dbURL,
		SupabaseURL:       os.Getenv("SUPABASE_URL"),
		SupabaseJWTSecret: os.Getenv("SUPABASE_JWT_SECRET"),
		SupabaseAnonKey:   os.Getenv("SUPABASE_ANON_KEY"),
		AppEnv:            appEnv,
		CORSOrigins:       []string{"*"}, // TODO: restrict in production
	}
}
