package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"

	"jiyi/mochat-go/internal/aiproviderconfig"
	"jiyi/mochat-go/internal/config"
	aiinsight "jiyi/mochat-go/internal/modules/ai-insight"
	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/outboundhttp"
	"jiyi/mochat-go/internal/store"
)

func parseScope(args []string) (int64, int64, error) {
	flags := flag.NewFlagSet("mochat-ai-insight-run", flag.ContinueOnError)
	tenantID := flags.Int64("tenant-id", 0, "authorized tenant id")
	corpID := flags.Int64("corp-id", 0, "authorized corp id")
	if err := flags.Parse(args); err != nil {
		return 0, 0, err
	}
	if *tenantID <= 0 || *corpID <= 0 || flags.NArg() != 0 {
		return 0, 0, errors.New("--tenant-id and --corp-id must be positive and no backfill arguments are allowed")
	}
	return *tenantID, *corpID, nil
}

func main() {
	tenantID, corpID, err := parseScope(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	cipher, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: cfg.AIProviderCredentialEncryptionKey, EncryptionKeys: cfg.AIProviderCredentialEncryptionKeys, EncryptionKeyID: cfg.AIProviderCredentialEncryptionKeyID, RequireEncryption: cfg.AIProviderCredentialRequireEncryption})
	if err != nil {
		log.Fatal("AI provider credential protection unavailable")
	}
	db, err := mysqlconn.Open(cfg.MySQLDSN)
	if err != nil {
		log.Fatal("AI insight database unavailable")
	}
	defer db.Close()
	mysqlStore := store.NewMySQLStore(db).WithAIProviderCredentialCipher(cipher).WithAIProviderOutboundGuard(outboundhttp.MustDefaultGuard())
	runner := aiinsight.NewConversationAnalysisRunner(aiinsight.NewSQLRepository(db), mysqlStore.TenantAIProviderResolver(), aiinsight.RunnerConfig{}, log.Default())
	if err := runner.RunCorp(context.Background(), tenantID, corpID); err != nil {
		log.Fatal("AI insight scoped run failed")
	}
}
