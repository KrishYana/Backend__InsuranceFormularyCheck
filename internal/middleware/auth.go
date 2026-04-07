package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/ent/physician"
	"github.com/kyanaman/formularycheck/internal/response"
)

type contextKey string

const physicianCtxKey contextKey = "physician"

// PhysicianFromCtx extracts the physician from the request context.
// Returns nil, false if no physician is set (e.g. guest mode).
func PhysicianFromCtx(ctx context.Context) (*ent.Physician, bool) {
	p, ok := ctx.Value(physicianCtxKey).(*ent.Physician)
	return p, ok
}

// AuthMiddleware holds dependencies for JWT validation.
type AuthMiddleware struct {
	db        *ent.Client
	jwtSecret []byte
	issuer    string // e.g. "https://your-project.supabase.co/auth/v1"
}

// NewAuthMiddleware creates a new auth middleware instance.
func NewAuthMiddleware(db *ent.Client, supabaseURL string, jwtSecret string) *AuthMiddleware {
	issuer := supabaseURL + "/auth/v1"
	return &AuthMiddleware{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		issuer:    issuer,
	}
}

// RequireAuth rejects requests without a valid Supabase JWT.
func (am *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		phys, err := am.authenticate(r)
		if err != nil {
			response.Unauthorized(w, "Your session has expired. Please sign in again.")
			return
		}
		ctx := context.WithValue(r.Context(), physicianCtxKey, phys)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth extracts the physician from JWT if present, but does not reject.
func (am *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		phys, err := am.authenticate(r)
		if err == nil && phys != nil {
			ctx := context.WithValue(r.Context(), physicianCtxKey, phys)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate validates the Bearer token and returns the Physician.
func (am *AuthMiddleware) authenticate(r *http.Request) (*ent.Physician, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, jwt.ErrTokenNotValidYet
	}

	tokenStr := strings.TrimPrefix(header, "Bearer ")
	if tokenStr == header {
		return nil, jwt.ErrTokenNotValidYet
	}

	// Parse and validate the JWT
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return am.jwtSecret, nil
	}, jwt.WithIssuer(am.issuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenNotValidYet
	}

	sub, _ := claims.GetSubject()
	if sub == "" {
		return nil, jwt.ErrTokenNotValidYet
	}

	// Look up or create physician by Supabase user ID
	phys, err := am.db.Physician.Query().
		Where(physician.SupabaseUserID(sub)).
		Only(r.Context())
	if ent.IsNotFound(err) {
		// Auto-create on first login
		email, _ := claims["email"].(string)
		if email == "" {
			email = sub + "@unknown"
		}
		phys, err = am.db.Physician.Create().
			SetSupabaseUserID(sub).
			SetEmail(email).
			SetDisplayName(email).
			Save(r.Context())
	}
	if err != nil {
		return nil, err
	}

	return phys, nil
}
