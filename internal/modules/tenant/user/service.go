package user

import (
	"context"

	"github.com/google/uuid"

	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/pagination"
	"go-api-starter/internal/response"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (View, error) {
	if err := appauth.ValidatePasswordStrength(req.Password); err != nil {
		return View{}, err
	}
	hash, err := appauth.HashPassword(req.Password)
	if err != nil {
		return View{}, err
	}
	u := User{ID: uuid.New(), Email: req.Email, PasswordHash: hash, Name: req.Name, IsActive: true}
	if err := s.repo.Create(ctx, u); err != nil {
		return View{}, err
	}
	return toView(u), nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (View, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return View{}, err
	}
	return toView(u), nil
}

func (s *Service) List(ctx context.Context, p pagination.Params) ([]View, response.Meta, error) {
	users, total, err := s.repo.List(ctx, p.Limit, p.Offset(), p.OrderByClause(Sortable))
	if err != nil {
		return nil, response.Meta{}, err
	}
	views := make([]View, len(users))
	for i, u := range users {
		views[i] = toView(u)
	}
	return views, pagination.BuildMeta(p.Page, p.Limit, total), nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) error {
	return s.repo.Update(ctx, id, req.Name, req.IsActive)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
