package posts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/joao.martins/blog/internal/auth"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Routes(protected func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// public
	r.Get("/", h.list)
	r.Get("/{id}", h.get)

	// protected (require valid JWT)
	r.Group(func(r chi.Router) {
		r.Use(protected)
		r.Post("/", h.create)
		r.Put("/{id}", h.update)
		r.Delete("/{id}", h.delete)
		r.Post("/{id}/submit", h.submit)
		r.With(auth.RequireRole("editor")).Post("/{id}/approve", h.approve)
		r.With(auth.RequireRole("editor")).Post("/{id}/reject", h.reject)
	})

	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, title, content, author_id, status, created_at, updated_at
		 FROM posts WHERE status = 'published' ORDER BY created_at DESC`)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var result []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		result = append(result, p)
	}
	if result == nil {
		result = []Post{}
	}
	writeJSON(w, result)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	var p Post
	err = h.db.QueryRowContext(r.Context(),
		`SELECT id, title, content, author_id, status, created_at, updated_at FROM posts WHERE id = $1`, id).
		Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	if p.Status != StatusPublished {
		claims, ok := auth.GetClaims(r)
		if !ok || (claims.Sub != p.AuthorID && !claims.HasRole("editor") && !claims.HasRole("admin")) {
			jsonError(w, "not found", http.StatusNotFound)
			return
		}
	}

	writeJSON(w, p)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r)

	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if input.Title == "" || input.Content == "" {
		jsonError(w, "title and content are required", http.StatusUnprocessableEntity)
		return
	}

	var p Post
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO posts (title, content, author_id, status) VALUES ($1, $2, $3, 'draft')
		 RETURNING id, title, content, author_id, status, created_at, updated_at`,
		input.Title, input.Content, claims.Sub).
		Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, p)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r)
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	var existing Post
	err = h.db.QueryRowContext(r.Context(), `SELECT author_id, status FROM posts WHERE id = $1`, id).
		Scan(&existing.AuthorID, &existing.Status)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if existing.AuthorID != claims.Sub {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}
	if existing.Status == StatusPublished {
		jsonError(w, "published posts cannot be edited", http.StatusUnprocessableEntity)
		return
	}

	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}

	var p Post
	err = h.db.QueryRowContext(r.Context(),
		`UPDATE posts SET title = $1, content = $2, updated_at = NOW()
		 WHERE id = $3 RETURNING id, title, content, author_id, status, created_at, updated_at`,
		input.Title, input.Content, id).
		Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, p)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r)
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	var authorID string
	err = h.db.QueryRowContext(r.Context(), `SELECT author_id FROM posts WHERE id = $1`, id).Scan(&authorID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if authorID != claims.Sub && !claims.HasRole("admin") {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}

	h.db.ExecContext(r.Context(), `DELETE FROM posts WHERE id = $1`, id) //nolint:errcheck
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, StatusDraft, StatusPendingReview)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, StatusPendingReview, StatusPublished)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, StatusPendingReview, StatusDraft)
}

func (h *Handler) changeStatus(w http.ResponseWriter, r *http.Request, from, to Status) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	res, err := h.db.ExecContext(r.Context(),
		`UPDATE posts SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3`, to, id, from)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonError(w, "post not found or wrong status", http.StatusUnprocessableEntity)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"error":"` + msg + `"}`)) //nolint:errcheck
}
