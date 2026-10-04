package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func respondWithError(w http.ResponseWriter, statusCode int, handlerName string, err error, lgr *slog.Logger) {
	lgr.Error("request failed", slog.String("handler", handlerName), slog.String("error", err.Error()))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(`{"error":"Error occurred"}`))
}

func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}, lgr *slog.Logger) {
	response, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "respondWithJSON", err, lgr)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(response)
}
