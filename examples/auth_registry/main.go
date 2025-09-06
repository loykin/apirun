package main

import (
	"context"
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	"github.com/loykin/apimigrate"
)

type demoConfig struct {
	Value string `mapstructure:"value"`
	Name  string `mapstructure:"name"`
}

type demoMethod struct{ c demoConfig }

func (d demoMethod) Acquire(_ context.Context) (string, error) {
	return d.c.Value, nil
}

func demoFactory(spec map[string]interface{}) (apimigrate.AuthMethod, error) {
	var c demoConfig
	if err := mapstructure.Decode(spec, &c); err != nil {
		return nil, err
	}
	return demoMethod{c: c}, nil
}

func main() {
	// Register our custom provider under type key "demo".
	apimigrate.RegisterAuthProvider("demo", demoFactory)
	fmt.Println("registered provider: demo")

	// Prepare a spec map that would typically come from a migration config.
	spec := map[string]interface{}{
		"value": "hello",
	}

	a := &apimigrate.Auth{Type: "demo", Name: "my-demo", Methods: apimigrate.NewAuthSpecFromMap(spec)}
	v, err := a.Acquire(context.Background(), nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("acquired: value=%s, name=%s\n", v, "my-demo")
}
