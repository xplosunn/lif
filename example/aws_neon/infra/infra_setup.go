package main

import (
	"github.com/xplosunn/lif/lib/lif"
)

func main() {
	defer lif.LifBuild()

	// db exposes url, user, password
	dbExposes := lif.NewNeonPostgres("mydb")

	// backend
	_, err := lif.NewAwsEC2(
		"backend",
	).OpenPorts([]string{
		"8080",
	}).WithEnvVars(map[string]string{
		"DB_URL":  dbExposes.Url,
		"DB_USER": dbExposes.User,
		"DB_PASS": dbExposes.Password,
	}).PathToDockerfile("../backend/Dockerfile")

	if err != nil {
		panic(err)
	}
}
