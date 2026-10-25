package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"grantplatform/internal/grants"
	"grantplatform/internal/httpapi"
)

func main() {
	if os.Getenv("APP_MODE") != "local_prototype" {
		log.Fatal("Set APP_MODE=local_prototype. This build is not approved for public use.")
	}
	token := os.Getenv("ADMIN_TOKEN")
	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if len(token) < 32 && adminHash == "" {
		log.Fatal("Configure ADMIN_TOKEN (at least 32 characters) or ADMIN_PASSWORD_HASH")
	}
	capacity, err := strconv.Atoi(os.Getenv("BENEFICIARY_CAPACITY"))
	if err != nil || capacity < 1 {
		log.Fatal("Set BENEFICIARY_CAPACITY to a positive funded beneficiary count")
	}
	path := os.Getenv("DATA_FILE")
	if path == "" {
		path = "data/grants.json"
	}
	service, err := grants.New(path, capacity)
	if err != nil {
		log.Fatal("Could not open data store: ", err)
	}
	srv := &http.Server{Addr: "127.0.0.1:8080", Handler: httpapi.New(service, token), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Print("Grant prototype on http://127.0.0.1:8080 — fictional data only; SMS and transfers disabled")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
