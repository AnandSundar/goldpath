package main

import (
	"context"
)

// Model and Repository are defined in service.go

type PostgresRepository struct {
	connStr string
}

func NewPostgresRepository(connStr string) (*PostgresRepository, error) {
	return &PostgresRepository{connStr: connStr}, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*Model, error) {
	// TODO: Implement database query
	return nil, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]*Model, error) {
	// TODO: Implement database query
	return nil, nil
}

func (r *PostgresRepository) Create(ctx context.Context, m *Model) error {
	// TODO: Implement database insert
	return nil
}

func (r *PostgresRepository) Update(ctx context.Context, m *Model) error {
	// TODO: Implement database update
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	// TODO: Implement database delete
	return nil
}
