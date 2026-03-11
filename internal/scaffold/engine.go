package scaffold

import (
	"context"
	"fmt"
	"strings"
	"text/template"
)

// TemplateInfo represents information about a scaffold template
type TemplateInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// GenerateRequest represents a scaffold generation request
type GenerateRequest struct {
	Template string            `json:"template"`
	Vars     map[string]string `json:"vars"`
}

// GenerateResult represents the result of scaffold generation
type GenerateResult struct {
	Files map[string]string `json:"files"`
	Count int               `json:"count"`
}

// Engine handles template scaffolding
type Engine struct {
	templates map[string]*template.Template
}

// NewEngine creates a new scaffold engine
func NewEngine() *Engine {
	e := &Engine{
		templates: make(map[string]*template.Template),
	}
	e.initTemplates()
	return e
}

func (e *Engine) initTemplates() {
	// Initialize built-in templates
	e.templates["go-api"] = template.Must(template.New("go-api").Parse(goAPITemplate))
	e.templates["go-service"] = template.Must(template.New("go-service").Parse(goServiceTemplate))
	e.templates["react-app"] = template.Must(template.New("react-app").Parse(reactAppTemplate))
}

// ListTemplates returns all available templates
func (e *Engine) ListTemplates() []TemplateInfo {
	infos := []TemplateInfo{
		{
			Name:        "go-api",
			Description: "Go REST API with chi router",
			Files:       []string{"main.go", "handlers.go", "models.go"},
		},
		{
			Name:        "go-service",
			Description: "Go microservice with repository pattern",
			Files:       []string{"service.go", "repository.go", "model.go"},
		},
		{
			Name:        "react-app",
			Description: "React application with Vite",
			Files:       []string{"App.tsx", "main.tsx", "index.html"},
		},
	}
	return infos
}

// Generate generates scaffold files from a template
func (e *Engine) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	tmpl, exists := e.templates[req.Template]
	if !exists {
		return nil, fmt.Errorf("template %s not found", req.Template)
	}

	result := &GenerateResult{
		Files: make(map[string]string),
		Count: 0,
	}

	// Parse template and generate content
	var tmplFiles = e.getTemplateFiles(req.Template)
	for _, tmplFile := range tmplFiles {
		content, err := e.executeTemplate(tmpl, tmplFile, req.Vars)
		if err != nil {
			return nil, fmt.Errorf("failed to execute template %s: %w", tmplFile, err)
		}
		result.Files[tmplFile] = content
		result.Count++
	}

	return result, nil
}

func (e *Engine) executeTemplate(tmpl *template.Template, name string, vars map[string]string) (string, error) {
	// Create a template with the specific file name
	t, err := tmpl.Clone()
	if err != nil {
		return "", err
	}

	// Add default values if not provided
	if vars == nil {
		vars = make(map[string]string)
	}
	if _, ok := vars["ProjectName"]; !ok {
		vars["ProjectName"] = "my-project"
	}
	if _, ok := vars["ModuleName"]; !ok {
		vars["ModuleName"] = "github.com/example/my-project"
	}

	// Execute template
	var buf strings.Builder
	err = t.Execute(&buf, vars)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (e *Engine) getTemplateFiles(templateName string) []string {
	switch templateName {
	case "go-api":
		return []string{"main.go", "handlers.go", "go.mod"}
	case "go-service":
		return []string{"service.go", "repository.go", "go.mod"}
	case "react-app":
		return []string{"package.json", "App.tsx", "main.tsx"}
	default:
		return []string{}
	}
}

// Built-in templates
const (
	goAPITemplate = `package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"message\": \"Welcome to {{.ProjectName}}\"}"))
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\": \"healthy\"}"))
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Server stopped")
}
`

	goServiceTemplate = `package main

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/lib/pq"
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

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(connStr string) (*PostgresRepository, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*Model, error) {
	// Implementation
	return nil, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]*Model, error) {
	// Implementation
	return nil, nil
}

func (r *PostgresRepository) Create(ctx context.Context, m *Model) error {
	// Implementation
	return nil
}

func (r *PostgresRepository) Update(ctx context.Context, m *Model) error {
	// Implementation
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	// Implementation
	return nil
}

func main() {
	log.Println("Starting {{.ProjectName}} service")
}
`

	reactAppTemplate = `import React from 'react'
import ReactDOM from 'react-dom/client'

function App() {
  return (
    <div className="app">
      <h1>Welcome to {{.ProjectName}}</h1>
      <p>Getting started with React</p>
    </div>
  )
}

export default App
`

	reactAppMain = `import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
`
)
