// Command bootstrap provisions the first tenant and admin accounts. It is the only way to
// create them: there is deliberately no HTTP endpoint, so there is no public path to admin.
//
//	bootstrap tenant -slug taruna -name "Taruna Eduka"
//	BOOTSTRAP_PASSWORD=... bootstrap admin -tenant taruna -name "Siti" -email siti@example.com
//	BOOTSTRAP_PASSWORD=... bootstrap admin -platform -name "Root" -email root@example.com
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: bootstrap tenant|admin [flags]")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := infra.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	tenants := service.NewTenantService(repository.NewTenantRepository(pool))
	users := service.NewUserService(repository.NewUserRepository(pool))

	switch args[0] {
	case "tenant":
		fs := flag.NewFlagSet("tenant", flag.ContinueOnError)
		slug := fs.String("slug", "", "tenant slug, immutable once created")
		name := fs.String("name", "", "display name")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		t, err := tenants.Provision(ctx, *slug, *name)
		if err != nil {
			return err
		}
		fmt.Printf("tenant %s created (%s)\n", t.Slug, t.ID)
		return nil

	case "admin":
		fs := flag.NewFlagSet("admin", flag.ContinueOnError)
		slug := fs.String("tenant", "", "tenant slug for a tenant admin")
		platform := fs.Bool("platform", false, "create a Platform Admin instead")
		name := fs.String("name", "", "full name")
		email := fs.String("email", "", "login email")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *platform == (*slug != "") {
			return errors.New("pass exactly one of -tenant or -platform")
		}
		// Read from the environment, never a flag, so it stays out of shell history and ps.
		password := os.Getenv("BOOTSTRAP_PASSWORD")

		var tenantID *uuid.UUID
		if !*platform {
			t, err := tenants.BySlug(ctx, *slug)
			if err != nil {
				return err
			}
			tenantID = &t.ID
		}
		created, err := users.ProvisionAdmin(ctx, tenantID, *name, *email, password)
		if err != nil {
			return err
		}
		fmt.Printf("%s %s created (%s)\n", created.Role, *email, created.ID)
		return nil

	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
