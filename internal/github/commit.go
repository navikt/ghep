package github

import (
	"regexp"
	"strings"
)

var coAuthorsRegexp = regexp.MustCompile(`Co-authored-by: (.*) <(.*)>`)

// FetchCoAuthors parses Co-authored-by trailers from a commit message and returns
// the resolved authors. Authors that cannot be linked to a GitHub username are
// still returned with an empty Username field — callers should skip those as needed.
func FetchCoAuthors(commitMessage string) []Author {
	var coAuthors []Author

	for _, match := range coAuthorsRegexp.FindAllStringSubmatch(commitMessage, -1) {
		name := match[1]
		email := ""
		if len(match) > 2 {
			email = match[2]
		}

		author := Author{
			Name:  name,
			Email: email,
		}

		if strings.HasPrefix(name, "@") {
			after, _ := strings.CutPrefix(name, "@")
			author.Username = after
		} else if strings.HasSuffix(email, "@users.noreply.github.com") {
			before, _ := strings.CutSuffix(email, "@users.noreply.github.com")
			_, after, found := strings.Cut(before, "+")
			if found {
				author.Username = after
			} else {
				author.Username = before
			}
		} else if name == "GitHub Action user" {
			continue
		}

		coAuthors = append(coAuthors, author)
	}

	return coAuthors
}
