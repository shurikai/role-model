package handlers

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/shurikai/role-model/internal/auth"
	"github.com/shurikai/role-model/internal/db"
)

type AuthHandler struct {
	queries   *db.Queries
	jwtSecret string
}

func NewAuthHandler(queries *db.Queries, jwtSecret string) *AuthHandler {
	return &AuthHandler{queries: queries, jwtSecret: jwtSecret}
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string  `json:"token"`
	User  userDTO `json:"user"`
}

// userDTO is the safe, public shape of a user — never includes password_hash.
type userDTO struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "email and password are required")
		return
	}
	if len(req.Password) < 8 {
		WriteError(w, http.StatusBadRequest, "validation_error", "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to process password")
		return
	}
	hashStr := string(hash)

	userID := uuid.New()
	user, err := h.queries.CreateUser(r.Context(), db.CreateUserParams{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: &hashStr,
	})
	if err != nil {
		// Most likely cause: email already exists (unique constraint).
		WriteError(w, http.StatusConflict, "email_taken", "an account with that email already exists")
		return
	}

	token, err := auth.IssueToken(user.ID, h.jwtSecret)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to issue token")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		Token: token,
		User:  userDTO{ID: user.ID, Email: user.Email},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "email and password are required")
		return
	}

	user, err := h.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Same generic error as a bad password — don't reveal which failed.
			WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to process login")
		return
	}

	// An account with no password (future OAuth-only user) can't log in by password.
	if user.PasswordHash == nil {
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	token, err := auth.IssueToken(user.ID, h.jwtSecret)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		Token: token,
		User:  userDTO{ID: user.ID, Email: user.Email},
	})
}
