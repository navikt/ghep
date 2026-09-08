package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFetchRepositories(t *testing.T) {
	repos := []map[string]any{
		{"name": "regular-repo", "archived": false, "fork": false},
		{"name": "forked-repo", "archived": false, "fork": true},
		{"name": "archived-repo", "archived": true, "fork": false},
		{"name": "blocklisted-repo", "archived": false, "fork": false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[`)
		for i, repo := range repos {
			if i > 0 {
				fmt.Fprint(w, `,`)
			}
			fmt.Fprintf(w, `{"name": %q, "archived": %t, "fork": %t}`, repo["name"], repo["archived"], repo["fork"])
		}
		fmt.Fprint(w, `]`)
	}))
	defer server.Close()

	tests := []struct {
		name        string
		ignoreForks bool
		want        []string
	}{
		{
			name:        "forks included by default",
			ignoreForks: false,
			want:        []string{"regular-repo", "forked-repo"},
		},
		{
			name:        "forks excluded when ignoreForks is set",
			ignoreForks: true,
			want:        []string{"regular-repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetchRepositories(server.URL, "token", []string{"blocklisted-repo"}, tt.ignoreForks)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("fetchRepositories mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
