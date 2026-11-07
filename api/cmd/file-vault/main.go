package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bcollard/file-vault/api/internal/blob"
	"github.com/bcollard/file-vault/api/internal/config"
	"github.com/bcollard/file-vault/api/internal/server"
	"github.com/bcollard/file-vault/api/internal/store"
	"github.com/bcollard/file-vault/api/internal/webui"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()

	var (
		st    store.Store
		bl    blob.Blobs
		local http.Handler
	)
	if cfg.LocalMode {
		dir := cfg.LocalDir
		if dir == "" {
			dir = filepath.Join(os.TempDir(), "file-vault-local")
		}
		lb, err := blob.NewLocal(dir)
		if err != nil {
			return err
		}
		st, bl, local = store.NewMemory(), lb, lb.Handler()
		slog.Warn("LOCAL_MODE: in-memory store, blobs on local disk — not for production", "dir", dir, "origin", cfg.PublicOrigin)
	} else {
		fs, err := store.NewFirestore(ctx, cfg.ProjectID, cfg.FirestoreDatabase)
		if err != nil {
			return err
		}
		gcs, err := blob.NewGCS(ctx, cfg.BucketName, cfg.SignerServiceAccount, cfg.PublicOrigin)
		if err != nil {
			return err
		}
		st, bl = fs, gcs
	}
	defer st.Close()
	defer bl.Close()

	srv := server.New(cfg, st, bl, webui.Handler(), local)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		slog.Info("shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	slog.Info("listening", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
