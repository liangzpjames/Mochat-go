package main

import (
	"flag"
	"log"

	"jiyi/mochat-go/internal/inventory"
)

func main() {
	sourceRoot := flag.String("source-root", "../mochat", "MoChat PHP source root")
	outDir := flag.String("out", "../docs/migration", "output directory")
	sourceRevision := flag.String("source-revision", "unknown", "upstream source revision")
	flag.Parse()

	report, err := inventory.Scan(*sourceRoot, *sourceRevision)
	if err != nil {
		log.Fatalf("scan inventory: %v", err)
	}
	if err := inventory.WriteMarkdownReports(report, *outDir); err != nil {
		log.Fatalf("write reports: %v", err)
	}
	log.Printf("routes=%d tables=%d crontabs=%d events=%d queues=%d",
		len(report.Routes),
		len(report.Tables),
		len(report.Crontabs),
		len(report.EventHandlers),
		len(report.AsyncQueues),
	)
}
