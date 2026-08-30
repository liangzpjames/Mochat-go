package main

import (
	"context"
	"fmt"
	"os"

	"jiyi/mochat-go/internal/aiinsight0165cli"
)

func main() {
	if err := aiinsight0165cli.Run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "0165 controlled migration failed:", err)
		os.Exit(1)
	}
}
