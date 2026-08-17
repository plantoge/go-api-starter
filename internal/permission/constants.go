package permission

const (
	UserView   = "user.view"
	UserCreate = "user.create"
	UserUpdate = "user.update"
	UserDelete = "user.delete"

	RoleView   = "role.view"
	RoleManage = "role.manage"
)

// All berisi semua konstanta permission di atas. Saat tenant baru
// disiapkan (Task 14), semuanya ditanam ke tabel permissions milik tenant
// itu. Sedangkan Sync (sync.go) yang nyusulin tenant lama kalau nanti ada
// konstanta baru ditambahin di sini.
var All = []string{
	UserView, UserCreate, UserUpdate, UserDelete,
	RoleView, RoleManage,
}
