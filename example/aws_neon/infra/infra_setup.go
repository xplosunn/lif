package main

import (
	"github.com/xplosunn/lif/lif"
)

func main() {
	defer lif.LifBuild()

	// db exposes url, user, password
	db := lif.NewNeonPostgres("mydb")

	// backend
	_ = lif.NewAwsEC2(
		"backend",
	).OpenPorts([]string{
		"8080",
	}).WithEnvVars(map[string]string{
		"DB_URL":  db.Exposes.Url,
		"DB_USER": db.Exposes.User,
		"DB_PASS": db.Exposes.Password,
	})
}
