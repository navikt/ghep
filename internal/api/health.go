package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (c *Client) healthGetHandler(w http.ResponseWriter, r *http.Request) {
	teams, err := c.db.ListTeams(r.Context())
	if err != nil {
		c.log.Error("error listing teams from database", "error", err)
		http.Error(w, fmt.Sprintf("error listing teams from database: %s", err.Error()), http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	payload := struct {
		Health string `json:"health"`
		Teams  string `json:"teams"`
	}{
		Health: "ok",
		Teams:  strings.Join(teams, ", "),
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(payload)
	if err != nil {
		c.log.Error("error when encoding response", "error", err)
		http.Error(w, fmt.Sprintf("error encoding response: %s", err.Error()), http.StatusInternalServerError)
		return
	}
}
