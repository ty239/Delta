package money

import (
	"net/http"
)

// my goal with this is to revice user Money and   get how much they cost

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Amount int64  `json:"amount"` // cents
}

func getUserMoney() (w http.ResponseWriter, r *http.Request) {
	mux := mux.HandleFunc("GET /users/{id}", a.getUser)

}
