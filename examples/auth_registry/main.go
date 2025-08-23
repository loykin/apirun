package main

import (
	"context"
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	"github.com/loykin/apimigrate"
)

type demoConfig struct {
	Header string `mapstructure:"header"`
	Value  string `mapstructure:"value"`
	Name   string `mapstructure:"name"`
}

type demoMethod struct{ c demoConfig }

func (d demoMethod) Name() string {
	if d.c.Name != "" {
		return d.c.Name
	}
	return "demo"
}

func (d demoMethod) Acquire(_ context.Context) (string, string, error) {
	header := d.c.Header
	if header == "" {
		header = "X-Demo"
	}
	return header, d.c.Value, nil
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
		"header": "X-Demo",
		"value":  "hello",
		"name":   "my-demo",
	}

	h, v, name, err := apimigrate.AcquireAuthByProviderSpec(context.Background(), "demo", spec)
	if err != nil {
		panic(err)
	}
	fmt.Printf("acquired: header=%s, value=%s, name=%s\n", h, v, name)
}
