package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	r.Get("/health", healthCheck)
	r.Get("/version", getVersion)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			r.Get("/", listUsers)
			r.Post("/", createUser)
			r.Get("/{user_id}", getUser)
			r.Put("/{user_id}", updateUser)
			r.Delete("/{user_id}", deleteUser)
		})

		r.Route("/products", func(r chi.Router) {
			r.Get("/", listProducts)
			r.Post("/", createProduct)
		})
	})

	http.ListenAndServe(":3000", r)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func getVersion(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"version": "1.0.0"})
}

func listUsers(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "name": "Alice"}})
}

func createUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": 2, "name": "Bob"})
}

func getUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "user_id")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "name": "Alice"})
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"updated": "true"})
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func listProducts(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "name": "Widget"}})
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": 2})
}
