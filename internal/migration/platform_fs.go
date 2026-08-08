package migration

import "embed"

//go:embed platform/*.sql
var PlatformFS embed.FS
