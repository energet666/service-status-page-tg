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
	} else {
		log.Print("BOT_TOKEN is empty; Telegram bot is disabled")
	}

	handler := httpapi.New(st, notifier, "web/dist")
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown failed: %v", err)
	}
}
