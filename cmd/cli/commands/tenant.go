package commands

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/uuid"

	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	"go-api-starter/internal/pagination"
)

// cliContext bikin context yang bawa penanda pelaku tetap dan gampang
// dikenali (UUID serba nol), dipakai buat operasi lifecycle tenant lewat
// CLI. Gunanya biar kolom audit (created_by/updated_by/deleted_by) nggak
// kosong cuma gara-gara CLI memang nggak punya user yang lagi login.
// Konvensinya dibahas di docs/database-conventions.md.
func cliContext() context.Context {
	return database.WithActor(context.Background(), database.Actor{UserID: uuid.Nil, Scope: "cli"})
}

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
		return nil, nil, fmt.Errorf("gagal terhubung ke database (pool platform): %w", err)
	}
	rawDB, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		platformDB.Close()
		return nil, nil, fmt.Errorf("gagal terhubung ke database (pool mentah): %w", err)
	}

	repo := platformtenant.NewRepository(platformDB, rawDB)
	resolver := middleware.NewTenantResolver(repo, 0) // CLI nggak butuh cache — sekali jalan langsung selesai
	svc := platformtenant.NewService(repo, rawDB, resolver)

	cleanup := func() {
		platformDB.Close()
		rawDB.Close()
	}
	return svc, cleanup, nil
}

func cmdTenantCreate(args []string) error {
	fs := flag.NewFlagSet("tenant create", flag.ExitOnError)
	code := fs.String("code", "", "kode tenant (wajib diisi)")
	name := fs.String("name", "", "nama tenant (wajib diisi)")
	ownerEmail := fs.String("owner-email", "", "email pemilik pertama (wajib diisi)")
	fs.Parse(args)

	if *code == "" || *name == "" || *ownerEmail == "" {
		return fmt.Errorf("--code, --name, dan --owner-email semuanya wajib diisi")
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

	result, err := svc.Provision(cliContext(), platformtenant.ProvisionInput{
		Code: *code, Name: *name, OwnerEmail: *ownerEmail,
	})
	if err != nil {
		return err
	}

	fmt.Println("tenant berhasil disiapkan:")
	fmt.Println("  kode:            ", result.Tenant.Code)
	fmt.Println("  email pemilik:   ", *ownerEmail)
	fmt.Println("  password pemilik:", result.OwnerPassword)
	fmt.Println("(password ini hanya ditampilkan sekali — ganti setelah login pertama)")
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
	code := fs.String("code", "", "kode tenant (wajib diisi)")
	fs.Parse(args)
	if *code == "" {
		return fmt.Errorf("--code wajib diisi")
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

	if err := action(svc, cliContext(), *code); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

func cmdTenantPurge(args []string) error {
	fs := flag.NewFlagSet("tenant purge", flag.ExitOnError)
	code := fs.String("code", "", "kode tenant (wajib diisi)")
	confirm := fs.Bool("confirm", false, "penegasan wajib — tindakan ini tidak bisa dibatalkan")
	fs.Parse(args)
	if *code == "" {
		return fmt.Errorf("--code wajib diisi")
	}
	if !*confirm {
		return fmt.Errorf("sertakan --confirm sebagai tanda kamu paham data tenant ini akan dihapus permanen")
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

	if err := svc.Purge(cliContext(), *code); err != nil {
		return err
	}
	fmt.Println("tenant sudah dihapus permanen")
	return nil
}
