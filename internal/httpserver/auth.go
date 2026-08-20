package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Muddyscode/ledger-api/internal/auth"
	"github.com/Muddyscode/ledger-api/internal/ledger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type creds struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowRegister {
		writeError(w, http.StatusForbidden, "register_disabled", "registration is disabled")
		return
	}
	var in creds
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "invalid_email", "email is required")
		return
	}
	if len(in.Password) < 12 {
		writeError(w, http.StatusBadRequest, "weak_password", "password must be at least 12 characters")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	id, err := uuid.NewV7()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	op, err := s.store.CreateOperator(r.Context(), s.store.Pool, id, email, hash)
	if err != nil {
		var pgErr *pgconn.PgError
		if asPg(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "email_taken", "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	token, exp, err := auth.IssueToken(s.cfg.JWTSecret, op.Email, op.ID, 8*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token": token, "expires_at": exp.UTC().Format(time.RFC3339), "operator": opJSON(op),
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in creds
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	op, err := s.store.GetOperatorByEmail(r.Context(), s.store.Pool, email)
	if err != nil || !auth.VerifyPassword(in.Password, op.PasswordHash) {
		writeError(w, http.StatusUnauthorized, ledger.CodeUnauthorized, "invalid credentials")
		return
	}
	token, exp, err := auth.IssueToken(s.cfg.JWTSecret, op.Email, op.ID, 8*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token, "expires_at": exp.UTC().Format(time.RFC3339), "operator": opJSON(op),
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"operator": opJSON(s.operator(r))})
}

func (s *Server) authn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerOrCookie(r)
		if raw == "" {
			writeError(w, http.StatusUnauthorized, ledger.CodeUnauthorized, "missing token")
			return
		}
		id, _, err := auth.ParseToken(s.cfg.JWTSecret, raw)
		if err != nil {
			writeError(w, http.StatusUnauthorized, ledger.CodeUnauthorized, "invalid token")
			return
		}
		op, err := s.store.GetOperatorByID(r.Context(), s.store.Pool, id)
		if err != nil {
			writeError(w, http.StatusUnauthorized, ledger.CodeUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), operatorKey, op)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerOrCookie(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	c, err := r.Cookie("ledger_token")
	if err != nil {
		return ""
	}
	return c.Value
}

func asPg(err error, target **pgconn.PgError) bool {
	var e *pgconn.PgError
	if errors.As(err, &e) {
		*target = e
		return true
	}
	return false
}
