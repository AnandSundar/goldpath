package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/planatechnologies/goldpath/internal/ai"
	"github.com/planatechnologies/goldpath/internal/api"
	"github.com/planatechnologies/goldpath/internal/config"
	"github.com/planatechnologies/goldpath/internal/flags"
	"github.com/planatechnologies/goldpath/internal/observability"
	"github.com/planatechnologies/goldpath/internal/scaffold"
	"github.com/spf13/cobra"
)

var (
	version    = "dev"
	commit     = "unknown"
	date       = "unknown"
	configFile string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "goldpath",
	Short: "Goldpath - Internal Developer Platform Tool",
	Long: `Goldpath is an Internal Developer Platform (IDP) tool that provides:
- Feature flags for controlled feature releases
- Golden path scaffolding templates
- AI-driven workflow suggestions
- Observability and SLO tracking`,
	Version: version + " (" + commit + " " + date + ")",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path (default is ./config.yaml)")
	rootCmd.PersistentFlags().StringP("port", "p", "8080", "server port")
	rootCmd.PersistentFlags().StringP("host", "H", "0.0.0.0", "server host")

	// Add the 'new' subcommand
	rootCmd.AddCommand(newCmd)
}

func runServer() error {
	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	// Initialize observability
	metrics := observability.NewMetrics()
	logger := observability.NewLogger()

	// Initialize services
	flagRepo := flags.NewInMemoryRepository()
	flagService := flags.NewService(flagRepo, metrics)

	scaffoldEngine := scaffold.NewEngine()

	aiHandler := ai.NewHandler(cfg.OpenAIAPIKey, cfg.AIEnabled)

	// Setup router
	router := api.NewRouter(api.RouterDeps{
		FlagService:    flagService,
		ScaffoldEngine: scaffoldEngine,
		AIHandler:      aiHandler,
		Metrics:        metrics,
		Logger:         logger,
	})

	// Create HTTP server
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting goldpath server", "address", addr, "version", version)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return err
	}

	logger.Info("Server gracefully stopped")
	return nil
}

// newCmd represents the 'new' command for scaffolding a new service
var newCmd = &cobra.Command{
	Use:   "new [service-name]",
	Short: "Scaffold a new service",
	Long: `Create a new service scaffold with the specified language and cloud provider.

Examples:
  goldpath new my-service --lang go --cloud aws
  goldpath new api-service -l python -c gcp
  goldpath new webapp -l nodejs -c azure`,
	Args: cobra.ExactArgs(1),
	RunE: runNewCommand,
}

// Supported languages and cloud providers
var (
	supportedLanguages = []string{"go", "python", "nodejs", "java"}
	supportedClouds    = []string{"aws", "gcp", "azure"}
)

func init() {
	// Add flags to new command
	newCmd.Flags().StringP("lang", "l", "", "Language/framework (go, python, nodejs, java) (required)")
	newCmd.Flags().StringP("cloud", "c", "", "Cloud provider (aws, gcp, azure) (required)")
	newCmd.MarkFlagRequired("lang")
	newCmd.MarkFlagRequired("cloud")
}

func runNewCommand(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	lang, _ := cmd.Flags().GetString("lang")
	cloud, _ := cmd.Flags().GetString("cloud")

	// Validate language
	if !isValidLanguage(lang) {
		return fmt.Errorf("invalid language: %s. Supported: %v", lang, supportedLanguages)
	}

	// Validate cloud provider
	if !isValidCloud(cloud) {
		return fmt.Errorf("invalid cloud provider: %s. Supported: %v", cloud, supportedClouds)
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create service directory
	serviceDir := filepath.Join(cwd, serviceName)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", serviceName, err)
	}

	// Initialize scaffold engine
	engine := scaffold.NewEngine()

	// Map language to template
	template := getTemplateForLanguage(lang)

	// Prepare template variables
	vars := map[string]string{
		"ProjectName": serviceName,
		"ModuleName":  "github.com/example/" + serviceName,
		"Language":    lang,
		"Cloud":       cloud,
	}

	// Generate scaffold files
	req := scaffold.GenerateRequest{
		Template: template,
		Vars:     vars,
	}

	result, err := engine.Generate(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to generate scaffold: %w", err)
	}

	// Write generated files to directory
	for filename, content := range result.Files {
		filePath := filepath.Join(serviceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filename, err)
		}
		fmt.Printf("Created: %s\n", filepath.Join(serviceName, filename))
	}

	// Generate additional cloud-specific files
	if err := generateCloudFiles(serviceDir, lang, cloud, serviceName); err != nil {
		return fmt.Errorf("failed to generate cloud files: %w", err)
	}

	fmt.Printf("\nSuccessfully scaffolded service '%s' in %s/\n", serviceName, serviceName)
	fmt.Printf("Language: %s, Cloud: %s\n", lang, cloud)
	fmt.Printf("Total files created: %d\n", result.Count+3) // +3 for Dockerfile, k8s, github actions

	return nil
}

func isValidLanguage(lang string) bool {
	for _, l := range supportedLanguages {
		if l == lang {
			return true
		}
	}
	return false
}

func isValidCloud(cloud string) bool {
	for _, c := range supportedClouds {
		if c == cloud {
			return true
		}
	}
	return false
}

func getTemplateForLanguage(lang string) string {
	switch lang {
	case "go":
		return "go-service"
	case "python":
		return "go-service" // Using go-service as placeholder, will be expanded
	case "nodejs":
		return "react-app" // Using react-app as placeholder, will be expanded
	case "java":
		return "go-service" // Using go-service as placeholder, will be expanded
	default:
		return "go-service"
	}
}

func generateCloudFiles(serviceDir, lang, cloud, serviceName string) error {
	// Generate Dockerfile
	dockerfile := getDockerfile(lang)
	dockerfilePath := filepath.Join(serviceDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return err
	}
	fmt.Printf("Created: %s\n", filepath.Join(serviceName, "Dockerfile"))

	// Generate Kubernetes manifest
	k8sManifest := getK8sManifest(serviceName, cloud)
	k8sPath := filepath.Join(serviceDir, "k8s", "deployment.yaml")
	if err := os.MkdirAll(filepath.Dir(k8sPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(k8sPath, []byte(k8sManifest), 0644); err != nil {
		return err
	}
	fmt.Printf("Created: %s\n", filepath.Join(serviceName, "k8s", "deployment.yaml"))

	// Generate GitHub Actions workflow
	githubWorkflow := getGitHubWorkflow(serviceName, lang, cloud)
	githubPath := filepath.Join(serviceDir, ".github", "workflows", "ci.yaml")
	if err := os.MkdirAll(filepath.Dir(githubPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(githubPath, []byte(githubWorkflow), 0644); err != nil {
		return err
	}
	fmt.Printf("Created: %s\n", filepath.Join(serviceName, ".github", "workflows", "ci.yaml"))

	return nil
}

func getDockerfile(lang string) string {
	switch lang {
	case "go":
		return `# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /service .

# Runtime stage
FROM alpine:3.18
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /service .
EXPOSE 8080
CMD ["./service"]
`
	case "python":
		return `# Python runtime
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8080
CMD ["python", "main.py"]
`
	case "nodejs":
		return `# Node.js runtime
FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 8080
CMD ["node", "index.js"]
`
	case "java":
		return `# Java runtime
FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY target/*.jar app.jar
EXPOSE 8080
CMD ["java", "-jar", "app.jar"]
`
	default:
		return ""
	}
}

func getK8sManifest(serviceName, cloud string) string {
	return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ` + serviceName + `
  labels:
    app: ` + serviceName + `
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ` + serviceName + `
  template:
    metadata:
      labels:
        app: ` + serviceName + `
    spec:
      containers:
      - name: ` + serviceName + `
        image: ` + serviceName + `:latest
        ports:
        - containerPort: 8080
        resources:
          limits:
            cpu: "500m"
            memory: "256Mi"
          requests:
            cpu: "200m"
            memory: "128Mi"
---
apiVersion: v1
kind: Service
metadata:
  name: ` + serviceName + `
spec:
  selector:
    app: ` + serviceName + `
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
`
}

func getGitHubWorkflow(serviceName, lang, cloud string) string {
	return `name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v4

    - name: Set up ` + getLanguageSetupStep(lang) + `
      uses: ` + getLanguageAction(lang) + `

    - name: Build
      run: ` + getBuildCommand(lang) + `

    - name: Test
      run: ` + getTestCommand(lang) + `

    - name: Build Docker image
      run: |
        docker build -t ` + serviceName + `:${{ github.sha }} .

    - name: ` + getCloudDeployStep(cloud) + `
      if: github.event_name == 'push'
      uses: ` + getCloudAction(cloud) + `
      with:
        ` + getCloudDeployArgs(cloud) + `
`
}

func getLanguageSetupStep(lang string) string {
	switch lang {
	case "go":
		return "Go"
	case "python":
		return "Python"
	case "nodejs":
		return "Node.js"
	case "java":
		return "JDK"
	default:
		return "language"
	}
}

func getLanguageAction(lang string) string {
	switch lang {
	case "go":
		return "actions/setup-go@v5"
	case "python":
		return "actions/setup-python@v5"
	case "nodejs":
		return "actions/setup-node@v4"
	case "java":
		return "actions/setup-java@v4"
	default:
		return ""
	}
}

func getBuildCommand(lang string) string {
	switch lang {
	case "go":
		return "go build -v ./..."
	case "python":
		return "pip install -r requirements.txt"
	case "nodejs":
		return "npm install"
	case "java":
		return "mvn package"
	default:
		return ""
	}
}

func getTestCommand(lang string) string {
	switch lang {
	case "go":
		return "go test -v ./..."
	case "python":
		return "pytest"
	case "nodejs":
		return "npm test"
	case "java":
		return "mvn test"
	default:
		return ""
	}
}

func getCloudDeployStep(cloud string) string {
	switch cloud {
	case "aws":
		return "Deploy to AWS"
	case "gcp":
		return "Deploy to GCP"
	case "azure":
		return "Deploy to Azure"
	default:
		return "Deploy"
	}
}

func getCloudAction(cloud string) string {
	switch cloud {
	case "aws":
		return "aws-actions/configure-aws-credentials@v4"
	case "gcp":
		return "google-github-actions/auth@v2"
	case "azure":
		return "azure/login@v1"
	default:
		return ""
	}
}

func getCloudDeployArgs(cloud string) string {
	switch cloud {
	case "aws":
		return "aws-region: us-east-1"
	case "gcp":
		return "credentials_json: ${{ secrets.GCP_SA_KEY }}"
	case "azure":
		return "creds: ${{ secrets.AZURE_CREDENTIALS }}"
	default:
		return ""
	}
}
