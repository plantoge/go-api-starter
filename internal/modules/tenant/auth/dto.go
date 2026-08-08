package auth

type LoginRequest struct {
	TenantCode string `json:"tenant_code" validate:"required"`
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type RefreshRequest struct {
	TenantCode   string `json:"tenant_code" validate:"required"`
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	TenantCode   string `json:"tenant_code" validate:"required"`
	RefreshToken string `json:"refresh_token" validate:"required"`
}
