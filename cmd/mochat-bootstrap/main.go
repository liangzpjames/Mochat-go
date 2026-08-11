package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/saasauth"
	"jiyi/mochat-go/internal/store"
)

type bootstrapOptions struct {
	DSN        string
	RequestKey string
	LoginName  string
	Phone      string
	Name       string
	Timeout    time.Duration
}

type bootstrapResult struct {
	UserID    int
	LoginName string
}

func main() {
	options := bootstrapOptions{}
	flag.StringVar(&options.DSN, "dsn", os.Getenv("MOCHAT_MYSQL_DSN"), "MySQL DSN; defaults to MOCHAT_MYSQL_DSN")
	flag.StringVar(&options.RequestKey, "request-key", envDefault("MOCHAT_BOOTSTRAP_REQUEST_KEY", ""), "idempotency key; defaults to MOCHAT_BOOTSTRAP_REQUEST_KEY")
	flag.StringVar(&options.LoginName, "login-name", envDefault("MOCHAT_BOOTSTRAP_SAAS_ADMIN_LOGIN", ""), "SaaS admin login name")
	flag.StringVar(&options.Phone, "phone", envDefault("MOCHAT_BOOTSTRAP_SAAS_ADMIN_PHONE", ""), "optional SaaS admin phone")
	flag.StringVar(&options.Name, "name", envDefault("MOCHAT_BOOTSTRAP_SAAS_ADMIN_NAME", ""), "SaaS admin display name")
	flag.DurationVar(&options.Timeout, "timeout", 2*time.Minute, "bootstrap timeout")
	flag.Parse()

	if err := validateBootstrapOptions(options); err != nil {
		log.Fatal(err)
	}
	password, err := readBootstrapSaaSAdminPassword(os.Getenv("MOCHAT_BOOTSTRAP_SAAS_ADMIN_PASSWORD_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	passwordHash, err := saasauth.HashPassword(password)
	if err != nil {
		log.Fatal("hash bootstrap password")
	}

	db, err := mysqlconn.Open(options.DSN)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()
	identity, err := saasauth.NewService(store.NewSaaSIdentityStore(db)).Bootstrap(ctx, saasauth.BootstrapSaaSAdmin{
		RequestKey:   options.RequestKey,
		LoginName:    options.LoginName,
		Phone:        options.Phone,
		Name:         options.Name,
		PasswordHash: passwordHash,
	})
	if err != nil {
		log.Fatalf("bootstrap SaaS admin failed: %v", err)
	}
	printResult(bootstrapResult{UserID: identity.ID, LoginName: identity.LoginName})
}

func validateBootstrapOptions(options bootstrapOptions) error {
	if strings.TrimSpace(options.DSN) == "" {
		return fmt.Errorf("MOCHAT_MYSQL_DSN or -dsn is required")
	}
	if strings.TrimSpace(options.RequestKey) == "" {
		return fmt.Errorf("MOCHAT_BOOTSTRAP_REQUEST_KEY or -request-key is required")
	}
	if strings.TrimSpace(options.LoginName) == "" {
		return fmt.Errorf("MOCHAT_BOOTSTRAP_SAAS_ADMIN_LOGIN or -login-name is required")
	}
	if strings.TrimSpace(options.Name) == "" {
		return fmt.Errorf("MOCHAT_BOOTSTRAP_SAAS_ADMIN_NAME or -name is required")
	}
	if options.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

func printResult(result bootstrapResult) {
	fmt.Printf("saas_admin_user_id\t%d\n", result.UserID)
	fmt.Printf("saas_admin_login_name\t%s\n", result.LoginName)
}

func readBootstrapSaaSAdminPassword(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("MOCHAT_BOOTSTRAP_SAAS_ADMIN_PASSWORD_FILE is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("read bootstrap password file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("bootstrap password file must not be a symlink")
	}
	if err := validateBootstrapPasswordFilePermissions(path, info); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read bootstrap password file: %w", err)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("bootstrap password file is empty")
	}
	return password, nil
}

func envDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
