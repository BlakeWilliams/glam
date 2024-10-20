package glam

import (
	"fmt"
	htmltemplate "html/template"
	"io"
	"os"
	"reflect"
	"unicode"

	"github.com/blakewilliams/glam/internal/template"
)

type (
	FuncMap = htmltemplate.FuncMap

	// Recoverable is an interface that components can implement that will
	// rescue the component from panic's. It provides an io.Writer to write
	// fallback content when the template is `recover`ed.
	Recoverable = template.Recoverable

	// Engine is a template engine that can be used to render components
	Engine struct {
		// components is a map of component names that are available in the template
		// it's used to determine if a tag is a component and should be rendered as such _and_
		// to instantiate the component in the generated code
		components  map[string]reflect.Type
		templateMap map[string]*template.Template
		funcs       htmltemplate.FuncMap

		// recompileMap tracks components that were parsed in component templates
		// but not registered, so were compiled as raw HTML.
		recompileMap map[string][]*template.Template
	}
)

// New creates a new template engine that can be used to register and render components
// to be rendered.
func New(funcs FuncMap) *Engine {
	e := &Engine{
		components:   make(map[string]reflect.Type),
		templateMap:  make(map[string]*template.Template),
		recompileMap: make(map[string][]*template.Template),
	}

	e.funcs = htmltemplate.FuncMap{
		"__glamDict": Dict,
	}

	for k, v := range funcs {
		e.funcs[k] = v
	}

	return e
}

// Render renders the provided toRender value to the provided writer. `renderable` should
// be a struct or a pointer to a struct that has been registered with the engine.
func (e *Engine) Render(w io.Writer, renderable any) error {
	return e.RenderWithFuncs(w, renderable, nil)
}

func (e *Engine) RenderWithFuncs(w io.Writer, renderable any, funcMap FuncMap) error {
	// Thought, create a render function that accepts a funcmap to override
	// after `.cloning` a template. This will enable passing request specific data
	v := reflect.ValueOf(renderable)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if template, ok := e.templateMap[v.Type().Name()]; ok {
		err := template.Execute(w, renderable, funcMap)
		if err != nil {
			return fmt.Errorf("error rendering component: %w", err)
		}

		return nil
	}

	return fmt.Errorf("No component found for type %s", v.Type().Name())
}

// RegisterComponent registers a component with the engine. The provided value must be a struct
// or a pointer to a struct. The provided template string will be parsed and the component will be
// rendered using the provided template.
func (e *Engine) RegisterComponent(value any, templateString string) error {
	r := reflect.TypeOf(value)
	if r.Kind() != reflect.Struct && (r.Kind() != reflect.Ptr && r.Elem().Kind() != reflect.Struct) {
		return fmt.Errorf("provided value must be a struct or a pointer to a struct")
	}

	v := reflect.ValueOf(value)
	if r.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	name := v.Type().Name()
	// We need access to public structs, so disallow private structs
	if unicode.IsLower([]rune(name)[0]) {
		return fmt.Errorf("component %s is private, registered components must be public", name)
	}

	e.components[name] = reflect.TypeOf(value)
	err := e.parseTemplate(name, templateString)
	if err != nil {
		return fmt.Errorf("could not register template: %w", err)
	}

	return nil
}

// RegisterComponentFS registers the given component with the engine, reading
// the file at the given path and using it as the template for the component.
func (e *Engine) RegisterComponentFS(value any, filePath string) error {
	c, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	return e.RegisterComponent(value, string(c))
}

// KnownComponents returns a map of known component names
func (e *Engine) KnownComponents() map[string]reflect.Type {
	return e.components
}

// :nodoc:
func (e *Engine) FuncMap() FuncMap {
	return e.funcs
}

func (e *Engine) parseTemplate(name, templateValue string) error {
	// Recompile any templates that were parsed as raw HTML because this component
	// wasn't registered yet
	if templates, ok := e.recompileMap[name]; ok {
		for _, t := range templates {
			err := e.parseTemplate(t.Name, t.RawContent())
			if err != nil {
				return fmt.Errorf("could not recompile template: %w", err)
			}
		}

		delete(e.recompileMap, name)
	}

	t, err := template.New(name, e, templateValue)
	if err != nil {
		return err
	}

	// Register potentially referenced components with the engine so we can
	// recompile this template if the referenced component is registered later.
	for k := range t.ComponentsPotentiallyReferenced() {
		if _, ok := e.recompileMap[k]; !ok {
			e.recompileMap[k] = make([]*template.Template, 0)
		}

		e.recompileMap[k] = append(e.recompileMap[k], t)
	}

	e.templateMap[name] = t

	return nil
}

// Dict is a helper function that can be used to create a map[string]any
// in a template. It's primarily used to pass attributes to components.
func Dict(args ...any) map[string]any {
	if len(args)%2 != 0 {
		panic("invalid number of arguments passed to __glamDict")
	}

	dict := make(map[string]any, len(args)/2)

	for i := 0; i < len(args); i += 2 {
		dict[args[i].(string)] = args[i+1]
	}

	return dict
}
