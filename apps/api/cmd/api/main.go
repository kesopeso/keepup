package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/config"
	"keepup/apps/api/internal/database"
	"keepup/apps/api/internal/httpapi"
	"keepup/apps/api/internal/maildelivery"
	"keepup/apps/api/internal/routes"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.App.Env)

	dbPool, err := database.NewPool(ctx, logger, cfg.Database)
	if err != nil {
		logger.Error("initialize database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	routeService := routes.NewService(routes.NewPostgresRepository(dbPool), cfg.Routes.DefaultMaxTrackingMembers, cfg.Routes.PositionValidation)
	handler := httpapi.NewHandler(logger, cfg.App, dbPool, routeService)
	if cfg.Auth.Enabled {
		accountRoutes, routeErr := routes.NewAccountService(dbPool, cfg.Routes.DefaultMaxTrackingMembers, cfg.Routes.PositionValidation, cfg.Auth.Policy.SessionIdle, cfg.Auth.SocketTicketTTL, cfg.Auth.Recovery.Policy.Retention, cfg.Auth.Recovery.EncryptionKey)
		if routeErr != nil {
			logger.Error("initialize account route service")
			os.Exit(1)
		}
		var options []auth.ServiceOption
		options = append(options, auth.WithCleanup(accountRoutes.CleanupTickets), auth.WithSessionRevoker(accountRoutes))
		if len(cfg.Auth.Google.WebClientIDs)+len(cfg.Auth.Google.NativeClientIDs) > 0 {
			presenters := append(append([]string(nil), cfg.Auth.Google.WebClientIDs...), cfg.Auth.Google.NativeClientIDs...)
			options = append(options, auth.WithGoogle(cfg.Auth.Google, auth.NewGoogleVerifier(ctx, cfg.Auth.Google.RequestTimeout, presenters)))
		}
		accountService, initErr := auth.NewService(auth.NewPostgresRepository(dbPool), cfg.Auth.Policy, cfg.Auth.Recovery, options...)
		if initErr != nil {
			logger.Error("initialize account service")
			os.Exit(1)
		}
		var sender maildelivery.Sender
		switch cfg.Auth.MailProvider {
		case "smtp":
			smtpSender, smtpErr := maildelivery.NewSMTP(cfg.Auth.SMTP)
			if smtpErr != nil {
				logger.Error("initialize SMTP provider")
				os.Exit(1)
			}
			sender = smtpSender
		case "fake":
			sender = &maildelivery.Fake{}
		default:
			logger.Error("unsupported account mail provider")
			os.Exit(1)
		}
		mailCtx, cancelMail := context.WithCancel(ctx)
		mailDone := make(chan struct{})
		go func() { defer close(mailDone); accountService.RunMail(mailCtx, sender, logger) }()
		defer func() { cancelMail(); <-mailDone }()
		mux := http.NewServeMux()
		mux.Handle("/auth/", httpapi.NewAccountHandler(logger, cfg.Auth, cfg.App.Env, accountService))
		mux.Handle("/", httpapi.NewHandler(logger, cfg.App, dbPool, routeService, httpapi.WithAccountRoutes(accountRoutes, accountService, cfg.Auth)))
		handler = mux
		stopLive, liveErr := accountRoutes.StartLive(ctx, logger, stop)
		if liveErr != nil {
			logger.Error("initialize account live runtime", "error", liveErr)
			os.Exit(1)
		}
		defer stopLive()
	}

	if err := httpapi.Serve(ctx, logger, cfg.App, handler); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("serve api", "error", err)
		os.Exit(1)
	}
}

func newLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
