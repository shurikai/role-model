package handlers

import (
	"net/http"

	"github.com/shurikai/role-model/internal/career"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
)

type CareerHandler struct {
	reader *career.Reader
}

func NewCareerHandler(queries *db.Queries) *CareerHandler {
	return &CareerHandler{reader: career.NewReader(queries)}
}

// Get returns the user's whole career as one tree: employers, each with its
// positions and their contribution counts.
func (h *CareerHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	tree, err := h.reader.Tree(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch career")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, tree)
}
