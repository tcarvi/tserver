package middleware

import (
	"encoding/json"
	"net/http"

	dtoresponse "github.com/tcarvi/tserver/internal/dto/response"
)

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dtoresponse.Error{Error: message})
}
