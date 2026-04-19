package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"service-status-page/internal/bot"
	"service-status-page/internal/checks"
	"service-status-page/internal/config"
	"service-status-page/internal/httpapi"
	"service-status-page/internal/store"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("failed to load .env: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	st, err := store.Open(cfg.DataFile)
	if err != nil {
		log.Fatal(err)
	}

	checker, err := checks.New(cfg.ChecksFile)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var notifier httpapi.ReportNotifier
	var tb *bot.Bot
	if cfg.BotToken != "" {
		tb, err = bot.New(cfg, st)
		if err != nil {
			log.Fatal(err)
		}
		notifier = tb
		go tb.Start()
		defer tb.Stop()
		if cfg.CheckInterval > 0 {
			monitor := checks.NewMonitor(checker, tb, cfg.CheckInterval)
			go monitor.Run(ctx)
			log.Printf("availability monitor interval is %s", cfg.CheckInterval)
		} else {
			log.Print("availability monitor is disabled")
		}
	} else {
		log.Print("BOT_TOKEN is empty; Telegram bot is disabled")
	}

	handler := httpapi.New(st, notifier, checker, "web/dist")
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	handler.Shutdown()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown failed: %v", err)
	}
}
