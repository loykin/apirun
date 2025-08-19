package task

import "github.com/loykin/apimigrate/pkg/env"

type Down struct {
	Name    string   `yaml:"name" json:"name"`
	Auth    string   `yaml:"auth" json:"auth"`
	Env     env.Env  `yaml:"env" json:"env"`
	Headers []Header `yaml:"headers" json:"headers"`
	Queries []Query  `yaml:"queries" json:"queries"`
	Body    string   `yaml:"body" json:"body"`
}
