package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/wecomarchivedemo"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: wecom-archive-demo <keygen|serve|healthcheck|sdkcheck>")
		return 2
	}
	switch args[0] {
	case "keygen":
		flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
		flags.SetOutput(stderr)
		output := flags.String("output", "", "output directory")
		publicURL := flags.String("public-url", "", "public base URL")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		if _, err := wecomarchivedemo.GenerateConfig(*output, *publicURL); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		absolute, _ := filepath.Abs(*output)
		fmt.Fprintf(stdout, "configuration generated at %s\n", absolute)
		return 0
	case "serve":
		flags := flag.NewFlagSet("serve", flag.ContinueOnError)
		flags.SetOutput(stderr)
		configPath := flags.String("config", "/config/config.json", "configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		if err := serve(*configPath); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "healthcheck":
		flags := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
		flags.SetOutput(stderr)
		url := flags.String("url", "http://127.0.0.1:8080/healthz", "health URL")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get(*url)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			fmt.Fprintf(stderr, "health status %d\n", response.StatusCode)
			return 1
		}
		return 0
	case "sdkcheck":
		if err := wecomarchivedemo.CheckFinanceSDKLibrary(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "WeCom Finance SDK loaded and required symbols resolved")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func serve(configPath string) error {
	config, err := wecomarchivedemo.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	store, err := wecomarchivedemo.NewEvidenceStore(config.DataDir)
	if err != nil {
		return fmt.Errorf("open evidence store: %w", err)
	}
	var sdk wecomarchivedemo.FinanceSDK
	var archive *wecomarchivedemo.ArchiveService
	if config.CorpID != "" && config.ArchiveSecret != "" {
		sdk, err = wecomarchivedemo.NewFinanceSDK(config.CorpID, config.ArchiveSecret)
		if err != nil {
			return fmt.Errorf("initialize Finance SDK: %w", err)
		}
		defer sdk.Close()
		archive, err = wecomarchivedemo.NewArchiveService(sdk, config.RSAPrivateKey, store, config.PullLimit, config.TimeoutSeconds)
		if err != nil {
			return err
		}
	}
	publicHandler := wecomarchivedemo.NewPublicHandler(config, store, sdk != nil)
	adminHandler := wecomarchivedemo.NewAdminHandler(config, store, archive)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return wecomarchivedemo.RunServers(ctx, config, publicHandler, adminHandler)
}
