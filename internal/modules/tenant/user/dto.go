package user

type CreateRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
}

type UpdateRequest struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

type View struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

func toView(u User) View {
	return View{ID: u.ID.String(), Email: u.Email, Name: u.Name, IsActive: u.IsActive}
}
