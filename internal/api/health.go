package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

func (c *Client) healthGetHandler(w http.ResponseWriter, r *http.Request) {
	pong, err := c.redis.Ping(r.Context()).Result()
	if err != nil {
		slog.Error("Error when pinging redis", "error", err)
		fmt.Fprintf(w, "Error pinging redis: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var teams []string
	for _, team := range c.teams {
		teams = append(teams, team.Name)
	}

	payload := struct {
		Health string `json:"health"`
		Redis  string `json:"redis"`
		Teams  string `json:"teams"`
	}{
		Health: "ok",
		Redis:  pong,
		Teams:  strings.Join(teams, ", "),
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(payload)
	if err != nil {
		slog.Error("Error when encoding response", "error", err)
		fmt.Fprintf(w, "Error encoding response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
