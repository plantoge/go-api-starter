package tenant

type ProvisionInput struct {
	Code       string `json:"code" validate:"required,min=3,max=50"`
	Name       string `json:"name" validate:"required"`
	OwnerEmail string `json:"owner_email" validate:"required,email"`
}

type ProvisionResult struct {
	Tenant        TenantView `json:"tenant"`
	OwnerPassword string     `json:"owner_password"`
}

type TenantView struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
