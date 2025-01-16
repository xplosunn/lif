package poc

func main() {
	defer DoneLIF()

	// db exposes url, user, password
	db := NeonPostgres("mydb")

	backend := AwsEC2(
		"backend",
	).openPorts(
		[]string{"8080"},
	).envVars(map[string]string{
		"DB_URL":  db.Exposes.Url,
		"DB_USER": db.Exposes.User,
		"DB_PASS": db.Exposes.Password,
	})

}
