/*
Copyright 2023 GleSYS AB

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/setoru/external-dns-huaweicloud-webhook/cmd/webhook/init/configuration"
	"github.com/setoru/external-dns-huaweicloud-webhook/pkg/webhook"
)

// Init creates and starts the HTTP server with webhook routes.
func Init(config configuration.Config, p *webhook.Webhook) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", p.Negotiate)
	mux.HandleFunc("GET /records", p.Records)
	mux.HandleFunc("POST /records", p.ApplyChanges)
	mux.HandleFunc("POST /adjustendpoints", p.AdjustEndpoints)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		ReadTimeout:  config.ServerReadTimeout,
		WriteTimeout: config.ServerWriteTimeout,
		Addr:         fmt.Sprintf("%s:%d", config.ServerHost, config.ServerPort),
		Handler:      mux,
	}

	go func() {
		slog.Info("starting server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "addr", srv.Addr, "error", err)
		}
	}()
	return srv
}

// ShutdownGracefully blocks until a termination signal is received, then shuts down the server.
func ShutdownGracefully(srv *http.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-sigCh

	slog.Info("shutting down server", "signal", sig)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("error shutting down server", "error", err)
	}
}
