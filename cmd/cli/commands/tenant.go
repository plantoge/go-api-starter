package commands

import (
	"context"
	"flag"
	"fmt"

	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	"go-api-starter/internal/pagination"
)

func init() {
	Register("tenant create", cmdTenantCreate)
	Register("tenant list", cmdTenantList)
	Register("tenant suspend", cmdTenantSuspend)
	Register("tenant activate", cmdTenantActivate)
	Register("tenant delete", cmdTenantDelete)
	Register("tenant purge", cmdTenantPurge)
}

func newTenantService(cfg *config.Config) (*platformtenant.Service, func(), error) {
	platformDB, err := database.NewPlatformPool(cfg.DB)
	if err != nil {
		return nil, nil, fmt.Errorf("connect (platform pool): %w", err)
	}
	rawDB, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		platformDB.Close()
		return nil, nil, fmt.Errorf("connect (raw pool): %w", err)
	}

	repo := platformtenant.NewRepository(platformDB, rawDB)
	resolver := middleware.NewTenantResolver(repo, 0) // CLI never needs caching — one-shot process
	svc := platformtenant.NewService(repo, rawDB, resolver)

	cleanup := func() {
		platformDB.Close()
		rawDB.Close()
	}
	return svc, cleanup, nil
}

func cmdTenantCreate(args []string) error {
	fs := flag.NewFlagSet("tenant create", flag.ExitOnError)
	code := fs.String("code", "", "tenant code (required)")
	name := fs.String("name", "", "tenant name (required)")
	ownerEmail := fs.String("owner-email", "", "first owner's email (required)")
	fs.Parse(args)

	if *code == "" || *name == "" || *ownerEmail == "" {
		return fmt.Errorf("--code, --name, and --owner-email are all required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := svc.Provision(context.Background(), platformtenant.ProvisionInput{
		Code: *code, Name: *name, OwnerEmail: *ownerEmail,
	})
	if err != nil {
		return err
	}

	fmt.Println("tenant provisioned:")
	fmt.Println("  code:          ", result.Tenant.Code)
	fmt.Println("  owner email:   ", *ownerEmail)
	fmt.Println("  owner password:", result.OwnerPassword)
	fmt.Println("(this password is shown once — change it after first login)")
	return nil
}

func cmdTenantList(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	views, _, err := svc.List(context.Background(), pagination.Params{
		Page: 1, Limit: 100, Sort: "created_at", Order: "desc",
	})
	if err != nil {
		return err
	}
	for _, v := range views {
		fmt.Printf("%-20s %-30s %s\n", v.Code, v.Name, v.Status)
	}
	return nil
}

func cmdTenantSuspend(args []string) error  { return tenantStatusCommand(args, "tenant suspend", (*platformtenant.Service).Suspend) }
func cmdTenantActivate(args []string) error { return tenantStatusCommand(args, "tenant activate", (*platformtenant.Service).Activate) }
func cmdTenantDelete(args []string) error   { return tenantStatusCommand(args, "tenant delete", (*platformtenant.Service).Delete) }

func tenantStatusCommand(args []string, name string, action func(*platformtenant.Service, context.Context, string) error) error {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	code := fs.String("code", "", "tenant code (required)")
	fs.Parse(args)
	if *code == "" {
		return fmt.Errorf("--code is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := action(svc, context.Background(), *code); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

func cmdTenantPurge(args []string) error {
	fs := flag.NewFlagSet("tenant purge", flag.ExitOnError)
	code := fs.String("code", "", "tenant code (required)")
	confirm := fs.Bool("confirm", false, "required acknowledgement — this is irreversible")
	fs.Parse(args)
	if *code == "" {
		return fmt.Errorf("--code is required")
	}
	if !*confirm {
		return fmt.Errorf("pass --confirm to acknowledge this permanently deletes the tenant's data")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.Purge(context.Background(), *code); err != nil {
		return err
	}
	fmt.Println("tenant permanently purged")
	return nil
}
