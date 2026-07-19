package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/api"
	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/config"
	"github.com/kachofugetsu09/nemeton/internal/gitrepo"
	"github.com/kachofugetsu09/nemeton/internal/project"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

func Run(ctx context.Context, configuration config.Config) error {
	// 1. Establish the exclusive local persistence boundary.
	if err := configuration.Prepare(); err != nil {
		return err
	}
	lock, err := acquireLock(configuration.LockPath)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := gitrepo.CheckVersion(ctx); err != nil {
		return err
	}
	if err := prepareSocket(configuration.SocketPath); err != nil {
		return err
	}

	// 2. Open durable owners and classify startup health.
	database, err := store.Open(ctx, configuration.DatabasePath, configuration.BackupRoot)
	if err != nil {
		return err
	}
	defer database.Close()
	artifacts := artifact.New(configuration.ArtifactRoot)
	service := project.NewService(database, artifacts, configuration.WorktreesRoot)
	reconcile, err := service.Reconcile(ctx)
	if err != nil {
		return err
	}
	state := api.NewRuntimeState(reconcile)

	// 3. Serve the single Unix Socket until graceful cancellation.
	listener, err := net.Listen("unix", configuration.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on Unix Socket %s: %w", configuration.SocketPath, err)
	}
	defer listener.Close()
	defer os.Remove(configuration.SocketPath)
	if err := os.Chmod(configuration.SocketPath, 0o600); err != nil {
		return fmt.Errorf("set Unix Socket permissions: %w", err)
	}
	server := &http.Server{
		Handler:           api.NewServer(service, state, configuration.DataDir, configuration.WorktreesRoot).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()
	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve nemetond API: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down nemetond: %w", err)
		}
		if err := <-serveResult; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("finish nemetond API: %w", err)
		}
		return nil
	}
}

func prepareSocket(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Unix Socket path %s: %w", path, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refuse to remove non-socket object at %s", path)
	}
	connection, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
	if dialErr == nil {
		connection.Close()
		return fmt.Errorf("a live service already owns Unix Socket %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale Unix Socket %s: %w", path, err)
	}
	return nil
}
