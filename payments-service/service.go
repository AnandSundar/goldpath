package main

import (
	"context"
	"log"
)

type Repository interface {
	Get(ctx context.Context, id string) (*Model, error)
	List(ctx context.Context) ([]*Model, error)
	Create(ctx context.Context, m *Model) error
	Update(ctx context.Context, m *Model) error
	Delete(ctx context.Context, id string) error
}

type Model struct {
	ID        string
	Name      string
	CreatedAt string
	UpdatedAt string
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, id string) (*Model, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*Model, error) {
	return s.repo.List(ctx)
}

func (s *Service) Create(ctx context.Context, m *Model) error {
	return s.repo.Create(ctx, m)
}

func (s *Service) Update(ctx context.Context, m *Model) error {
	return s.repo.Update(ctx, m)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func main() {
	log.Println("Starting payments-service service")
}
