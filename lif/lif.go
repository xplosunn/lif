package lif

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

//BLABLABLABLABLABLABLABLABLA

var (
	resources = make(map[string]any)
	// stores concrete values for compose local deployment
	composeResources = make(map[string]map[string]any)
)

type AwsEC2 struct {
	name       string
	ports      []string
	envVars    map[string]string
	dockerfile string
}

type AwsEC2WithPorts struct {
	instance *AwsEC2
}

type AwsEC2WithEnvVars struct {
	instance *AwsEC2
}

type AwsEC2WithDockerfile struct {
	instance *AwsEC2
}

type AwsEC2Exposes struct {
}

func NewAwsEC2(name string) *AwsEC2 {
	return &AwsEC2{
		name:       name,
		ports:      make([]string, 0),
		envVars:    make(map[string]string),
		dockerfile: "",
	}
}

func (e *AwsEC2) OpenPorts(ports []string) *AwsEC2WithPorts {
	e.ports = ports
	return &AwsEC2WithPorts{
		instance: e,
	}
}

func (e *AwsEC2WithPorts) WithEnvVars(envVars map[string]string) *AwsEC2WithEnvVars {
	e.instance.envVars = envVars

	return &AwsEC2WithEnvVars{
		instance: e.instance,
	}
}

func (e *AwsEC2WithEnvVars) PathToDockerfile(path string) (*AwsEC2WithDockerfile, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	e.instance.dockerfile = absPath

	resources[e.instance.name] = map[string]any{
		"type":       "ec2",
		"ports":      e.instance.ports,
		"env_vars":   e.instance.envVars,
		"dockerfile": e.instance.dockerfile,
	}

	return &AwsEC2WithDockerfile{
		instance: e.instance,
	}, nil
}

type NeonPostgres struct {
	Exposes *NeonPostgresExposes
}

type NeonPostgresExposes struct {
	Url      string
	User     string
	Password string
}

func NewNeonPostgres(name string) NeonPostgresExposes {
	exposes := NeonPostgresExposes{
		Url:      "ref:" + name + ":url",
		User:     "ref:" + name + ":user",
		Password: "ref:" + name + ":password",
	}

	resources[name] = map[string]any{
		"type":    "postgres",
		"exposes": exposes,
	}

	return exposes
}

func LifBuild() {
	err := os.MkdirAll(".lif", 0755)
	if err != nil {
		panic(err)
	}

	infra := map[string]any{
		"resources": resources,
	}

	jsonBytes, err := json.MarshalIndent(infra, "", "  ")
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(".lif/infrastructure.json", jsonBytes, 0644)
	if err != nil {
		panic(err)
	}

	// Generate the docker compose file
	dockerComposeBytes, err := GenerateDockerCompose(jsonBytes)
	if err != nil {
		panic(err)
	}

	// create temp directory for docker-compose
	tempDir, err := os.MkdirTemp("", "lif-compose-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	composePath := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, dockerComposeBytes, 0644); err != nil {
		panic(err)
	}

	// Run docker compose up
	cmd := exec.Command("docker", "compose", "-f", composePath, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(err)
	}
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

	println("dbname", dbname)
	println("user", user)
	println("password", password)

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
	for k, v := range envVars {
		ref := v
		resolvedEnvVars[k] = resolveLocalRef(ref)

		// if this env var references another resource, it's a dependency
		parts := strings.Split(ref, ":")
		if len(parts) == 3 && parts[0] == "ref" {
			dependencies = append(dependencies, parts[1])
		}
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

func resolveLocalRef(ref string) string {
	// ref format is "ref:resourceName:property"
	parts := strings.Split(ref, ":")
	if len(parts) != 3 || parts[0] != "ref" {
		return ref
	}

	resourceName := parts[1]
	property := parts[2]

	// Check composeResources first
	if composeRes, exists := composeResources[resourceName]; exists {
		if val, ok := composeRes[property]; ok {
			return val.(string)
		}
	}

	// Then check regular resources
	if res, exists := resources[resourceName]; exists {
		if resourceSpecs, ok := res.(map[string]any); ok && resourceSpecs["type"] == "postgres" {
			exposes := resourceSpecs["exposes"].(NeonPostgresExposes)

			switch property {
			case "url":
				return exposes.Url
			case "user":
				return exposes.User
			case "password":
				return exposes.Password
			}
		}
	}

	// if we can't resolve it, return the original ref??
	return ref
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
