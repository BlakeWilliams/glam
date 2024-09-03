package example

import (
	"fmt"
	"github.com/blakewilliams/goat/template"
	stdtemplate "html/template"
)

func NewEngine(funcs stdtemplate.FuncMap) (*template.Engine, error) {
	e := template.New(funcs)
	var err error

	err = e.RegisterComponent(&Wrapper{}, "{{.WrapperStart}}\n{{Children}}\n{{.WrapperEnd}}\n")
	if err != nil {
		return nil, fmt.Errorf("failed to register component Wrapper: %w", err)
	}

	err = e.RegisterComponent(&Greeter{}, "<h1>Hi {{.Name}}</h1>\n")
	if err != nil {
		return nil, fmt.Errorf("failed to register component Greeter: %w", err)
	}

	return e, nil
}
