package example

import (
	"fmt"
	"html/template"
)

// What
//
//goat:component wrapper.goat.html
type Wrapper struct {
	Tag      string
	Children template.HTML
}

type (
	// Another field to exercise the parser
	Foo interface {
		Bar()
	}

	// Wow
	//
	//goat:component greeter.goat.html
	Greeter struct {
		Name string
	}
)

func (w *Wrapper) WrapperStart() template.HTML {
	return template.HTML(fmt.Sprintf("<%s>", w.Tag))
}
func (w *Wrapper) WrapperEnd() template.HTML {
	return template.HTML(fmt.Sprintf("</%s>", w.Tag))
}
