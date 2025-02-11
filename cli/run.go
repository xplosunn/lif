package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// stores concrete values for compose local deployment
	composeResources = make(map[string]map[string]any)
)

func run(pathToInfraJsonFile string) {
	jsonBytes, err := os.ReadFile("/tmp/dat")
	if err != nil {
		panic(err)
	}

	// Generate the docker compose file
	dockerComposeBytes, err := GenerateDockerCompose(jsonBytes)
	if err != nil {
		panic(err)
	}

	// create temp directory for docker-compose
	// i want to see the generated docker compose file in the terminal
	fmt.Println(string(dockerComposeBytes))

	tempFile, err := os.CreateTemp("", "lif-compose.yml")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tempFile.Name())

	if err := os.WriteFile(tempFile.Name(), dockerComposeBytes, 0644); err != nil {
		panic(err)
	}

	downCmd := exec.Command("docker", "compose", "-f", tempFile.Name(), "down")
	if err := downCmd.Run(); err != nil {
		// it's normal to get errors if no containers exist
		fmt.Printf("Warning during compose down: %v\n", err)
	}

	// Then run the existing compose up command
	cmd := exec.Command("docker", "compose", "-f", tempFile.Name(), "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	//TODO make this run again
	//if err := cmd.Run(); err != nil {
	//	panic(err)
	//}
}

type NeonPostgresExposes struct {
	Url      string
	User     string
	Password string
}

type DockerCompose struct {
	Version  string                    `yaml:"version"`
	Services map[string]map[string]any `yaml:"services"`
}

func GenerateDockerCompose(jsonBytes []byte) ([]byte, error) {
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

	// first we process postgres resources because the user and password (generated randomly) need to be created
	// before the EC2 compose section can include them
	for resourceName, resourceSpecs := range resources {
		if resourceSpecs.(map[string]any)["type"] == "postgres" {
			composeMap, err := GeneratePostgresComposeMap(resourceName, resourceSpecs.(map[string]any))
			if err != nil {
				return nil, err
			}
			compose.Services[resourceName] = composeMap
		}
	}

	// Then process EC2 resources. This depends on the postgres user and password that were randomly
	// generated in GeneratePostgresComposeMap above. Maybe we could just have hardcoded values for the
	// user and password and then we wouldn't have to care about the order of operations?
	for resourceName, resourceSpecs := range resources {
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
		"url":      "postgres://" + user + ":" + password + "@mydb:5432/" + dbname,
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
	envVars := resourceMap["env_vars"].(map[string]string)
	resolvedEnvVars := make(map[string]string)
	dependencies := make([]string, 0)
	dockerfilePath := resourceMap["dockerfile"].(string)
	buildContext := filepath.Dir(dockerfilePath)
	dockerfileName := filepath.Base(dockerfilePath)

	// resolve references and collect dependencies
	for _, v := range envVars {
		ref, err := resolveLocalRef(v)
		if err != nil {
			return nil, err
		}

		// if this env var references another resource, it's a dependency
		parts := strings.Split(ref, ":")
		if len(parts) == 3 && parts[0] == "ref" {
			dependencies = append(dependencies, parts[1])
		}
	}

	ports := resourceMap["ports"].([]string)
	for i, port := range ports {
		ports[i] = port + ":" + port
	}
	compose := map[string]any{
		"environment": resolvedEnvVars,
		"ports":       resourceMap["ports"],
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
		return "", ErrInvalidRefFormat(ref)
	}

	resourceName := parts[1]
	property := parts[2]

	if value, err := resolveComposeResource(resourceName, property); err == nil {
		return value, nil
	}

	// return "", ErrResourceNotFound(resourceName)

	return resolveRegularResource(resourceName, property)
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

func resolveRegularResource(resourceName, property string) (string, error) {
	res, exists := resources[resourceName]
	if !exists {
		return "", ErrResourceNotFound(resourceName)
	}

	resourceSpecs, ok := res.(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid resource specification for %q", resourceName)
	}

	if resourceSpecs["type"] != "postgres" {
		return "", fmt.Errorf("unsupported resource type for %q", resourceName)
	}

	exposes, ok := resourceSpecs["exposes"].(NeonPostgresExposes)
	if !ok {
		return "", fmt.Errorf("invalid exposes field for postgres resource %q", resourceName)
	}

	switch property {
	case "url":
		return exposes.Url, nil
	case "user":
		return exposes.User, nil
	case "password":
		return exposes.Password, nil
	default:
		return "", ErrPropertyNotFound(resourceName, property)
	}
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
