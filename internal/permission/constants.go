package permission

const (
	UserView   = "user.view"
	UserCreate = "user.create"
	UserUpdate = "user.update"
	UserDelete = "user.delete"

	RoleView   = "role.view"
	RoleManage = "role.manage"
)

// All lists every permission constant above. Tenant provisioning (Task 14)
// seeds each of these into a brand new tenant's permissions table; Sync
// (sync.go) brings already-provisioned tenants up to date when a new
// constant is added later.
var All = []string{
	UserView, UserCreate, UserUpdate, UserDelete,
	RoleView, RoleManage,
}
