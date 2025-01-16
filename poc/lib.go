package poc

type Resource[Exposes any] struct {
	Id                         string
	ResourceSpecificParameters map[string]string
	Exposes                    Exposes
}

type NeonPostgresExposes struct {
	Url      string
	User     string
	Password string
}

func NeonPostgres(id string) Resource[NeonPostgresExposes] {
	return Resource[NeonPostgresExposes]{
		Id:                         id, // used as project name when creating
		ResourceSpecificParameters: map[string]string{},
		Exposes:                    NeonPostgresExposes{},
	}
}

type AwsEC2Exposes struct {
}

func AwsEC2(id string) Resource[AwsEC2Exposes] {
	return Resource[any]{
		Id:                         id, // used as project name when creating
		ResourceSpecificParameters: map[string]string{},
	}
}

func (r Resource[AwsEC2Exposes]) openPorts(ports []string) Resource[AwsEC2Exposes] {
	return r
}

func (r Resource[AwsEC2Exposes]) envVars(environmentVariables map[string]string) Resource[AwsEC2Exposes] {
	return r
}

func DoneLIF() {

}
