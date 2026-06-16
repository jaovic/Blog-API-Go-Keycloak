package users

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joao.martins/blog/internal/auth"
)

type Handler struct {
	kc *KeycloakClient
}

func NewHandler(kc *KeycloakClient) *Handler {
	return &Handler{kc: kc}
}

func (h *Handler) Routes(protected func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// público
	r.Post("/register", h.register)

	// protegido + role admin
	r.Group(func(r chi.Router) {
		r.Use(protected)
		r.Use(auth.RequireRole("admin"))
		r.Get("/", h.list)
		r.Get("/{id}", h.get)
		r.Patch("/{id}/roles", h.assignRoles)
		r.Delete("/{id}", h.delete)
	})

	return r
}

// POST /users/register — público
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if msg := input.Validate(); msg != "" {
		jsonError(w, msg, http.StatusUnprocessableEntity)
		return
	}

	userID, err := h.kc.CreateUser(input)
	if err != nil {
		if err.Error() == "username or email already exists" {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		jsonError(w, "failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// atribui role "author" por padrão
	if err := h.kc.AssignRoles(userID, []string{"author"}); err != nil {
		jsonError(w, "user created but failed to assign role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{
		"id":      userID,
		"message": "user created successfully",
	})
}

// GET /users — admin
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.kc.GetUsers()
	if err != nil {
		jsonError(w, "failed to list users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, users)
}

// GET /users/:id — admin
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.kc.GetUser(id)
	if err != nil {
		jsonError(w, "failed to get user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}
	writeJSON(w, user)
}

// PATCH /users/:id/roles — admin
func (h *Handler) assignRoles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input AssignRolesInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(input.Roles) == 0 {
		jsonError(w, "roles cannot be empty", http.StatusUnprocessableEntity)
		return
	}

	// remove todas as roles atuais e atribui as novas
	currentRoles, err := h.kc.GetUserRoles(id)
	if err == nil && len(currentRoles) > 0 {
		h.kc.RemoveRoles(id, currentRoles) //nolint:errcheck
	}

	if err := h.kc.AssignRoles(id, input.Roles); err != nil {
		jsonError(w, "failed to assign roles: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"id":    id,
		"roles": input.Roles,
	})
}

// DELETE /users/:id — admin
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.kc.DeleteUser(id); err != nil {
		jsonError(w, "failed to delete user: "+err.Error(), http.StatusInternalServerError)
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
