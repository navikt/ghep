package api

import (
	"fmt"
	"html/template"
	"net/http"
)

var indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>The inner sanctuary</title>
</head>
<body>
    <h1>Teams with repositories!</h1>
    {{- range . }}
        <h2 id="{{ .Name }}"><a href="https://github.com/orgs/navikt/teams/{{ .Name }}/repositories">{{ .Name }}</a>:</h2>
        <ul>
            {{ range .Repositories }}
                <li>{{ . }}</li>
            {{ end }}
        </ul>
    {{- end }}
</body>
</html>
`

func (c *Client) frontendGetHandler(w http.ResponseWriter, r *http.Request) {
	type Team struct {
		Name         string
		Repositories []string
	}

	tmpl, err := template.New("index.html").Parse(indexHTML)
	if err != nil {
		c.log.Error("error parsing index.html template", "error", err)
		http.Error(w, fmt.Sprintf("error parsing index.html template: %s", err.Error()), http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teamNames, err := c.db.ListTeams(r.Context())
	if err != nil {
		c.log.Error("error listing teams from database", "error", err)
		http.Error(w, fmt.Sprintf("error listing teams from database: %s", err.Error()), http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teams := make([]Team, len(teamNames))
	fmt.Println(len(teamNames), len(teams), teamNames)
	for i, teamName := range teamNames {
		repositories, err := c.db.ListTeamRepositories(r.Context(), teamName)
		if err != nil {
			c.log.Error(fmt.Sprintf("error listing repositories for %s from database", teamName), "error", err)
			http.Error(w, fmt.Sprintf("error listing teams from database: %s", err.Error()), http.StatusInternalServerError)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		repos := make([]string, len(repositories))
		for j, repository := range repositories {
			repos[j] = repository.Name
		}

		teams[i] = Team{
			Name:         teamName,
			Repositories: repos,
		}
	}

	fmt.Println(teams)
	tmpl.Execute(w, teams)
}
