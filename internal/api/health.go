package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (c *Client) healthGetHandler(w http.ResponseWriter, r *http.Request) {
	pong, err := c.redis.Ping(r.Context()).Result()
	if err != nil {
		c.log.Error("error pinging redis", "error", err)
		http.Error(w, fmt.Sprintf("error pinging redis: %s", err.Error()), http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var teams []string
	for _, team := range c.teamConfig {
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
		c.log.Error("error when encoding response", "error", err)
		http.Error(w, fmt.Sprintf("error encoding response: %s", err.Error()), http.StatusInternalServerError)
		return
	}
}
