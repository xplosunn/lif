package lif

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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

	// make this a temp file
	tempFile, err := os.CreateTemp("", "lif-infrastructure.json")
	if err != nil {
		panic(err)
	}
	println("===================================")
	println(string(jsonBytes))
	println("===================================")
	defer os.Remove(tempFile.Name())

	err = os.WriteFile(tempFile.Name(), jsonBytes, 0644)
	if err != nil {
		panic(err)
	}
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	println("json definition file:")
	println(filepath.Join(path, tempFile.Name()))
}
