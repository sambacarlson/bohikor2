package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authpassword"
	"github.com/Iknite-Space/bohikor2/internal/config"
)

// create-platform-admin bootstraps the first super-admin. Platform admins are
// global (no company) and provision companies + their first admins via the API.
func main() {
	email := flag.String("email", "", "Platform admin email address")
	password := flag.String("password", "", "Platform admin password")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: create-platform-admin --email=<email> --password=<password>")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	hasher := authpassword.NewBcryptHasher()

	hashed, err := hasher.Hash(*password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	pa, err := queries.CreatePlatformAdmin(ctx, db.CreatePlatformAdminParams{
		Email:        *email,
		PasswordHash: hashed,
	})
	if err != nil {
		log.Fatalf("create platform admin: %v", err)
	}

	fmt.Printf("Platform admin created: id=%s email=%s\n", pa.ID, pa.Email)
}
