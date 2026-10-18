package handler

import (
	"log/slog"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// AuthOptions are the two transport decisions the auth routes need; every rule lives in the service.
type AuthOptions struct {
	// SecureCookies marks the refresh cookie Secure outside development.
	SecureCookies bool
	// AllowedOrigin is the CORS decision; refresh and logout require it in addition to the request host.
	AllowedOrigin func(origin string) bool
}

type Handler struct {
	logger        *slog.Logger
	health        *HealthHandler
	auth          *service.AuthService
	account       *service.AccountService
	curriculum    *service.CurriculumService
	questions     *service.QuestionService
	tryouts       *service.TryoutService
	products      *service.ProductService
	wallets       *service.WalletService
	secureCookies bool
	allowedOrigin func(origin string) bool
}

func New(logger *slog.Logger, health *HealthHandler, auth *service.AuthService, account *service.AccountService, curriculum *service.CurriculumService, questions *service.QuestionService, tryouts *service.TryoutService, products *service.ProductService, wallets *service.WalletService, opts AuthOptions) *Handler {
	return &Handler{
		logger:        logger,
		health:        health,
		auth:          auth,
		account:       account,
		curriculum:    curriculum,
		questions:     questions,
		tryouts:       tryouts,
		products:      products,
		wallets:       wallets,
		secureCookies: opts.SecureCookies,
		allowedOrigin: opts.AllowedOrigin,
	}
}

func (h *Handler) Register(e *echo.Echo, apiMiddleware ...echo.MiddlewareFunc) {
	e.GET("/healthz", h.health.Live)
	e.GET("/readyz", h.health.Ready)

	v1 := e.Group("/api/v1")
	v1.Use(apiMiddleware...)
	v1.GET("/tenant", h.Tenant)

	// NoStore is first on every auth route so a rejection carries it too.
	v1.POST("/auth/login", h.Login, httpx.NoStore())
	v1.POST("/auth/refresh", h.Refresh, httpx.NoStore(), h.requireAcceptedOrigin)
	v1.POST("/auth/logout", h.Logout, httpx.NoStore(), h.requireAcceptedOrigin)
	// None of these act on a cookie alone, so like login they need no Origin check.
	v1.POST("/auth/register", h.SignUp, httpx.NoStore())
	v1.POST("/auth/password/forgot", h.ForgotPassword, httpx.NoStore())
	v1.POST("/auth/password/reset", h.ResetPassword, httpx.NoStore())
	v1.POST("/me/email/verify", h.VerifyEmail, httpx.NoStore(), Authenticate(h.auth))
	v1.POST("/me/email/resend", h.ResendEmailCode, httpx.NoStore(), Authenticate(h.auth))
	v1.GET("/me", h.Me, httpx.NoStore(), Authenticate(h.auth))

	// Curriculum: operators read the tree for their dropdowns, only admins change it (FR-ADM-PRG-01).
	v1.GET("/programs", h.ListPrograms, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/programs", h.CreateProgram, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin))
	v1.PUT("/programs/:id", h.ReplaceProgram, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin))
	v1.DELETE("/programs/:id", h.DeleteProgram, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin))

	// Questions are tenant content, so an Operator writes them too; a Student never sees the keys.
	v1.GET("/questions", h.ListQuestions, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	// Import is static before the :id routes, so a template or preview is never read as a question id.
	v1.GET("/questions/import/template", h.QuestionImportTemplate, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/questions/import/preview", h.PreviewQuestionImport, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/questions/import/commit", h.CommitQuestionImport, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.GET("/questions/:id", h.GetQuestion, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/questions", h.CreateQuestion, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.PUT("/questions/:id", h.UpdateQuestion, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.DELETE("/questions/:id", h.DeleteQuestion, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))

	// Tryouts are tenant content too; a Student never reaches the builder (D02 pending, role-only).
	v1.GET("/tryouts", h.ListTryouts, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/tryouts", h.CreateTryout, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.GET("/tryouts/:id", h.GetTryout, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.PUT("/tryouts/:id", h.UpdateTryout, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.DELETE("/tryouts/:id", h.DeleteTryout, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/tryouts/:id/publish", h.PublishTryout, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/tryouts/:id/unpublish", h.UnpublishTryout, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))

	// Products are tenant content like tryouts, so an Operator writes them too.
	v1.GET("/products", h.ListProducts, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.POST("/products", h.CreateProduct, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.GET("/products/:id", h.GetProduct, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.PUT("/products/:id", h.UpdateProduct, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.DELETE("/products/:id", h.DeleteProduct, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))

	// The catalog is anonymous and host-resolved; no-store because the tenant is in Host, not the path.
	v1.GET("/catalog/products", h.CatalogProducts, httpx.NoStore())
	v1.GET("/catalog/products/:id", h.CatalogProduct, httpx.NoStore())
	v1.GET("/catalog/summary", h.CatalogSummary, httpx.NoStore())

	// A wallet belongs to one student, so only that student reaches /wallet (D02 pending, role-only).
	v1.GET("/wallet", h.Wallet, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleStudent))
	v1.GET("/wallet/entries", h.WalletEntries, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleStudent))
	v1.GET("/students", h.ListStudents, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.GET("/students/:id/wallet", h.StudentWallet, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	v1.GET("/students/:id/wallet/entries", h.StudentWalletEntries, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin, model.RoleOperator))
	// Only an admin adjusts a balance; an operator reads it (FR-PAY-W-06, D02 pending).
	v1.POST("/students/:id/wallet/adjustments", h.AdjustStudentWallet, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin))
}
