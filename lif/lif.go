package lif

import (
	"encoding/json"
	"os"
)

var resources = make(map[string]any)

type AwsEC2 struct {
	name    string
	ports   []string
	envVars map[string]string
}

type AwsEC2WithPorts struct {
	instance *AwsEC2
}

type AwsEC2WithEnvVars struct {
	instance *AwsEC2
}

type AwsEC2Exposes struct {
}

func NewAwsEC2(name string) *AwsEC2 {
	return &AwsEC2{
		name:    name,
		ports:   make([]string, 0),
		envVars: make(map[string]string),
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

	resources[e.instance.name] = map[string]any{
        "type": "ec2",
        "ports": e.instance.ports,
        "env_vars": e.instance.envVars,
    }

	return &AwsEC2WithEnvVars{
		instance: e.instance,
	}
}

type NeonPostgres struct {
	name    string
	Exposes NeonPostgresExposes
}

type NeonPostgresExposes struct {
	Url      string
	User     string
	Password string
}

func NewNeonPostgres(name string) *NeonPostgres {
	db := &NeonPostgres{
		name: name,
		Exposes: NeonPostgresExposes{
			Url:      "postgres://localhost:5432",
			User:     "postgres",
			Password: "postgres",
		},
	}

	resources[name] = map[string]any{
		"type": "postgres",
		"exposes": db.Exposes,
	}

	return db
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
}
