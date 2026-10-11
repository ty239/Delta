package money

import (
	"encoding/json"
	"net/http"
)

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Amount int64  `json:"amount"` // cents
}

type store struct {
	users map[string]User
}

type api struct {
	store *store
}

// handler: receives (w, r), returns nothing
func (a *api) getUserMoney(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	u, ok := a.store.users[id]
	if !ok {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

// registration: separate function, lists the whole API
func (a *api) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}/money", a.getUserMoney)
	return mux
}
