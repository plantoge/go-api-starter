package migration

import "embed"

//go:embed tenant/*.sql
var TenantFS embed.FS
