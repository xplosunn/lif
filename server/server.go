package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// stores concrete values for compose local deployment
	composeResources = make(map[string]map[string]any)
	// server status
	serverStatus = "idle"
)

// Server represents the infrastructure deployment server
type Server struct {
	Port int
}

// NewServer creates a new server instance
func NewServer(port int) *Server {
	return &Server{
		Port: port,
	}
}

// Start starts the server
func (s *Server) Start() error {
	http.HandleFunc("/status", s.handleStatus)
	http.HandleFunc("/deploy", s.handleDeploy)
	
	serverAddr := fmt.Sprintf(":%d", s.Port)
	fmt.Printf("Starting server on %s\n", serverAddr)
	serverStatus = "running"
	
	return http.ListenAndServe(serverAddr, nil)
}

// handleStatus returns the current server status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Server status: %s", serverStatus)
}

// handleDeploy processes the JSON infrastructure definition and deploys it locally
func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the JSON payload
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	// Generate Docker Compose from JSON
	dockerComposeBytes, err := GenerateDockerCompose(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate Docker Compose: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Create temp file for Docker Compose
	tempFile, err := os.CreateTemp("", "lif-compose.yml")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	
	if err := os.WriteFile(tempFile.Name(), dockerComposeBytes, 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write Docker Compose file: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Run docker-compose down
	downCmd := exec.Command("docker", "compose", "-f", tempFile.Name(), "down")
	if err := downCmd.Run(); err != nil {
		log.Printf("Warning during compose down: %v", err)
	}
	
	// Run docker-compose up
	upCmd := exec.Command("docker", "compose", "-f", tempFile.Name(), "up", "-d")
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	if err := upCmd.Run(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start Docker Compose: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Send success response with Docker Compose YAML
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Successfully deployed infrastructure:\n\n%s", string(dockerComposeBytes))
}

// DockerCompose represents the structure of a Docker Compose file
type DockerCompose struct {
	Version  string                    `yaml:"version"`
	Services map[string]map[string]any `yaml:"services"`
}

// GenerateDockerCompose converts the infrastructure JSON into Docker Compose YAML
func GenerateDockerCompose(jsonBytes []byte) ([]byte, error) {
	// Reset compose resources for each deployment
	composeResources = make(map[string]map[string]any)
	
	// parse the json
	var config struct {
		Resources map[string]any `json:"resources"`
	}

	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return nil, err
	}

	compose := DockerCompose{
		Version:  "3",
		Services: make(map[string]map[string]any),
	}

	// first we process postgres resources
	for resourceName, resourceSpecs := range config.Resources {
		if resourceSpecs.(map[string]any)["type"] == "postgres" {
			composeMap, err := GeneratePostgresComposeMap(resourceName, resourceSpecs.(map[string]any))
			if err != nil {
				return nil, err
			}
			compose.Services[resourceName] = composeMap
		}
	}

	// Then process EC2 resources
	for resourceName, resourceSpecs := range config.Resources {
		if resourceSpecs.(map[string]any)["type"] == "ec2" {
			composeMap, err := GenerateEC2ComposeMap(resourceSpecs.(map[string]any))
			if err != nil {
				return nil, err
			}
			compose.Services[resourceName] = composeMap
		}
	}

	return yaml.Marshal(compose)
}

// generate a docker compose builder for the postgres resource
func GeneratePostgresComposeMap(dbname string, resourceFields map[string]any) (map[string]any, error) {
	user := generateRandomString(16)
	password := generateRandomString(16)

	// Store concrete values in composeResources
	composeResources[dbname] = map[string]any{
		"url":      "postgres://" + user + ":" + password + "@" + dbname + ":5432/" + dbname,
		"user":     user,
		"password": password,
	}

	compose := map[string]any{
		"image": "postgres:latest",
		"environment": map[string]string{
			"POSTGRES_USER":     user,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       dbname,
		},
		"ports": []string{"5432:5432"},
		"healthcheck": map[string]any{
			"test":     []string{"CMD-SHELL", fmt.Sprintf("psql -U %s -d %s -c 'SELECT 1;'", user, dbname)},
			"interval": "5s",
			"timeout":  "5s",
			"retries":  5,
		},
	}

	return compose, nil
}

func GenerateEC2ComposeMap(resourceMap map[string]any) (map[string]any, error) {
	envVars := resourceMap["env_vars"].(map[string]any)
	resolvedEnvVars := make(map[string]string)
	dependencies := make([]string, 0)
	dockerfilePath := resourceMap["dockerfile"].(string)
	buildContext := filepath.Dir(dockerfilePath)
	dockerfileName := filepath.Base(dockerfilePath)

	// resolve references and collect dependencies
	for k, v := range envVars {
		strValue, ok := v.(string)
		if !ok {
			// Skip non-string values
			continue
		}
		
		ref, err := resolveLocalRef(strValue)
		if err != nil {
			return nil, err
		}
		
		resolvedEnvVars[k] = ref

		// if this env var references another resource, it's a dependency
		parts := strings.Split(strValue, ":")
		if len(parts) == 3 && parts[0] == "ref" {
			dependencies = append(dependencies, parts[1])
		}
	}

	portsAny := resourceMap["ports"].([]any)
	ports := make([]string, len(portsAny))
	for i, portAny := range portsAny {
		port, ok := portAny.(string)
		if !ok {
			// Try to convert to string if it's not already
			port = fmt.Sprintf("%v", portAny)
		}
		ports[i] = port + ":" + port
	}
	
	compose := map[string]any{
		"environment": resolvedEnvVars,
		"ports":       ports,
		"depends_on":  makeHealthcheckDependencies(dependencies),
		"build": map[string]string{
			"context":    buildContext,
			"dockerfile": dockerfileName,
		},
	}

	return compose, nil
}

var (
	ErrInvalidRefFormat = func(ref string) error {
		return fmt.Errorf("invalid ref format: %q, expected format 'ref:resourceName:property'", ref)
	}
	ErrResourceNotFound = func(resourceName string) error {
		return fmt.Errorf("resource not found: %q", resourceName)
	}
	ErrPropertyNotFound = func(resourceName, property string) error {
		return fmt.Errorf("property %q not found for resource %q", property, resourceName)
	}
)

func resolveLocalRef(ref string) (string, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 3 || parts[0] != "ref" {
		return ref, nil // Not a reference, return as is
	}

	resourceName := parts[1]
	property := parts[2]

	if value, err := resolveComposeResource(resourceName, property); err == nil {
		return value, nil
	}

	return "", ErrResourceNotFound(resourceName)
}

func resolveComposeResource(resourceName, property string) (string, error) {
	composeRes, exists := composeResources[resourceName]
	if !exists {
		return "", ErrResourceNotFound(resourceName)
	}

	val, ok := composeRes[property]
	if !ok {
		return "", ErrPropertyNotFound(resourceName, property)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("property %q for resource %q is not a string", property, resourceName)
	}

	return str, nil
}

// creates condition-based dependencies that wait for health checks
func makeHealthcheckDependencies(services []string) map[string]map[string]string {
	deps := make(map[string]map[string]string)
	for _, service := range services {
		deps[service] = map[string]string{
			"condition": "service_healthy",
		}
	}
	return deps
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)

	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}

	return string(b)
}