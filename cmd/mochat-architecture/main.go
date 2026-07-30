package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"jiyi/mochat-go/internal/architecture"
)

func main() {
	root := flag.String("root", ".", "repository root")
	policyPath := flag.String("policy", "architecture-policy.json", "policy file")
	flag.Parse()

	policy, err := architecture.LoadPolicy(filepath.Join(*root, *policyPath))
	if err != nil {
		log.Fatal(err)
	}
	violations, err := architecture.Audit(*root, policy, time.Now().UTC())
	if err != nil {
		log.Fatal(err)
	}
	for _, violation := range violations {
		fmt.Printf("%s %s: %s\n", violation.RuleID, violation.Path, violation.Detail)
	}
	if len(violations) > 0 {
		os.Exit(1)
	}
	fmt.Println("architecture boundaries passed")
}
