package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

// assetURLTTL is how long a presigned download link works; the client asks again instead of keeping it.
const assetURLTTL = 60 * time.Second

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

	store, err := newStorage(ctx, cfg)
	if err != nil {
		return err
	}

	e := newEcho(cfg, logger, pool, rdb, notifications, store)

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
func newEcho(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, rdb *redis.Client, notifications *service.NotificationService, store service.ObjectStore) *echo.Echo {
	tenants := service.NewTenantService(repository.NewTenantRepository(pool))
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
		Links:         service.WebLinks{BaseURL: cfg.AppURL},
		Secret:        cfg.JWTSecret,
		Logger:        logger,
	})
	curriculum := service.NewCurriculumService(pool, repository.NewCurriculumRepository(pool))
	questions := service.NewQuestionService(pool, repository.NewQuestionRepository(pool))
	tryouts := service.NewTryoutService(pool, repository.NewTryoutRepository(pool))
	products := service.NewProductService(pool, repository.NewProductRepository(pool))
	wallets := service.NewWalletService(pool, repository.NewWalletRepository(pool))
	assets := service.NewAssetService(pool, repository.NewAssetRepository(pool), store, assetURLTTL)
	orders := service.NewOrderService(pool, repository.NewOrderRepository(pool), repository.NewProductRepository(pool), wallets, assets, notifications,
		service.BankAccount{Bank: cfg.Bank.Name, Number: cfg.Bank.AccountNumber, Holder: cfg.Bank.AccountHolder})
	topups := service.NewTopupService(pool, repository.NewTopupRepository(pool), wallets, assets, notifications)
	runtimeRepo := repository.NewRuntimeRepository(pool)
	runtime := service.NewRuntimeService(pool, runtimeRepo)
	results := service.NewResultService(runtime, repository.NewResultRepository(pool), runtimeRepo)
	monitoring := service.NewMonitoringService(runtime, repository.NewMonitoringRepository(pool))
	itemPackages := service.NewItemPackageService(pool, repository.NewItemPackageRepository(pool), assets)
	kecermatan := service.NewKecermatanService(pool, repository.NewKecermatanRepository(pool), repository.NewItemPackageRepository(pool))
	kecermatanRuntime := service.NewKecermatanRuntimeService(pool, repository.NewKecermatanRuntimeRepository(pool), kecermatan, repository.NewKecermatanRepository(pool))
	h := handler.New(logger, handler.NewHealthHandler(pool), auth, account, curriculum, questions, tryouts, products, wallets, assets, orders, runtime, results, monitoring, itemPackages, kecermatan, handler.AuthOptions{
		SecureCookies: !cfg.IsDevelopment(),
		AllowedOrigin: httpx.OriginPolicy(cfg.CORSOrigins, cfg.IsDevelopment()),
	}).WithKecermatanRuntime(kecermatanRuntime).WithTopups(topups)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = httpx.ErrorHandler(logger)
	e.Use(httpx.RequestID())
	e.Use(httpx.Recover(logger))
	e.Use(httpx.RequestLogger(logger))
	e.Use(httpx.CORS(cfg.CORSOrigins, cfg.IsDevelopment()))
	h.Register(e, handler.ResolveHostScope(tenants, cfg.TenantSlug))

	return e
}

// newStorage is nil without S3_ENDPOINT, which turns the asset routes into 503s and breaks nothing else.
func newStorage(ctx context.Context, cfg config.Config) (service.ObjectStore, error) {
	if cfg.S3.Endpoint == "" {
		return nil, nil
	}
	storage, err := infra.NewStorage(cfg.S3)
	if err != nil {
		return nil, err
	}
	if err := storage.EnsureBucket(ctx, cfg.IsDevelopment()); err != nil {
		return nil, err
	}
	return storage, nil
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
