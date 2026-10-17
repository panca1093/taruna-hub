package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/handler"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := infra.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := infra.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer rdb.Close()

	notifications, err := newNotifications(cfg, logger, pool)
	if err != nil {
		return err
	}

	e := newEcho(cfg, logger, pool, rdb, notifications)

	go func() {
		logger.Info("server listening", slog.String("port", cfg.Port), slog.String("env", cfg.Env))
		if err := e.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", slog.String("error", err.Error()))
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
	defer cancel()
	return e.Shutdown(shutdownCtx)
}

// newEcho is the single assembly point, so tests run the real middleware chain.
func newEcho(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, rdb *redis.Client, notifications *service.NotificationService) *echo.Echo {
	tenants := service.NewTenantService(repository.NewTenantRepository(pool), cfg.BaseDomain)
	users := repository.NewUserRepository(pool)
	sessions := repository.NewSessionRepository(pool)
	throttles := repository.NewThrottleRepository(rdb)
	auth := service.NewAuthService(users, sessions, throttles, cfg.JWTSecret, logger)
	account := service.NewAccountService(service.AccountDeps{
		DB:            pool,
		Users:         users,
		Sessions:      sessions,
		Throttles:     throttles,
		Notifications: notifications,
		Auth:          auth,
		Links:         service.WebLinks{Scheme: cfg.WebScheme, BaseDomain: cfg.BaseDomain, Port: cfg.WebPort},
		Secret:        cfg.JWTSecret,
		Logger:        logger,
	})
	curriculum := service.NewCurriculumService(pool, repository.NewCurriculumRepository(pool))
	questions := service.NewQuestionService(pool, repository.NewQuestionRepository(pool))
	tryouts := service.NewTryoutService(pool, repository.NewTryoutRepository(pool))
	h := handler.New(logger, handler.NewHealthHandler(pool), auth, account, curriculum, questions, tryouts, handler.AuthOptions{
		SecureCookies: !cfg.IsDevelopment(),
		AllowedOrigin: httpx.OriginPolicy(cfg.CORSOrigins, cfg.IsDevelopment()),
	})

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = httpx.ErrorHandler(logger)
	e.Use(httpx.RequestID())
	e.Use(httpx.Recover(logger))
	e.Use(httpx.RequestLogger(logger))
	e.Use(httpx.CORS(cfg.CORSOrigins, cfg.IsDevelopment()))
	h.Register(e, handler.ResolveHostScope(tenants, cfg.TrustForwardedHost))

	return e
}

// newNotifications is the P02 wiring: one service over the insert-only River client.
func newNotifications(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) (*service.NotificationService, error) {
	inserter, err := infra.NewRiverInserter(pool, logger)
	if err != nil {
		return nil, err
	}
	return service.NewNotificationService(
		repository.NewNotificationRepository(pool),
		inserter,
		infra.NewSenders(cfg.SMTP, logger),
		cfg.NotificationSealKey,
		logger,
	)
}

func newLogger(cfg config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
