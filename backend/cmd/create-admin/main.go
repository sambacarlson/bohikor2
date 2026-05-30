package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Iknite-Space/bohikor2/internal/authpassword"
	"github.com/Iknite-Space/bohikor2/internal/config"
	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

func main() {
	email := flag.String("email", "", "Admin email address")
	password := flag.String("password", "", "Admin password")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: create-admin --email=<email> --password=<password>")
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

	admin, err := queries.CreateAdmin(ctx, db.CreateAdminParams{
		Email:        *email,
		PasswordHash: hashed,
	})
	if err != nil {
		log.Fatalf("create admin: %v", err)
	}

	fmt.Printf("Admin created: id=%s email=%s\n", admin.ID, admin.Email)
}
