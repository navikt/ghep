package main

import (
	"fmt"
	"os"

	"github.com/navikt/ghep/internal/github"
)

func main() {
	// This is a cli that takes a path to a teams config file and validates it
	// Usage: go run validate.go <path-to-config-file>
	path := os.Args[1]
	_, err := github.ParseTeamConfig(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Team config is valid")
}
