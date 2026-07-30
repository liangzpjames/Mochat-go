package main

import (
	"fmt"
	"os"

	"jiyi/mochat-go/internal/qualitygate"
)

func main() {
	root := "."
	if len(os.Args) == 2 {
		root = os.Args[1]
	} else if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: backendqualitycontract [repository-root]")
		os.Exit(2)
	}

	failures := qualitygate.Validate(root)
	if len(failures) != 0 {
		for _, failure := range failures {
			fmt.Fprintln(os.Stderr, "backend quality gate contract:", failure)
		}
		os.Exit(1)
	}
	fmt.Println("backend quality gate workflow contract passed")
}
