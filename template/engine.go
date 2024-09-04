package template

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"io"
	"reflect"
	"unicode"
)

// Engine is a template engine that can be used to render components
type Engine struct {
	// components is a map of component names that are available in the template
	// it's used to determine if a tag is a component and should be rendered as such _and_
	// to instantiate the component in the generated code
	components  map[string]reflect.Type
	templateMap map[string]*htmltemplate.Template
	funcs       htmltemplate.FuncMap
}

func New(funcs htmltemplate.FuncMap) *Engine {
	e := &Engine{
		components:  make(map[string]reflect.Type),
		templateMap: make(map[string]*htmltemplate.Template),
	}

	e.funcs = htmltemplate.FuncMap{
		"__goatDict": func(args ...any) map[string]any {
			if len(args)%2 != 0 {
				panic("invalid number of arguments to __goatDict")
			}

			dict := make(map[string]any, len(args)/2)

			for i := 0; i < len(args); i += 2 {
				dict[args[i].(string)] = args[i+1]
			}

			return dict
		},
	}

	for k, v := range funcs {
		e.funcs[k] = v
	}

	return e
}

// Render renders the provided toRender value to the provided writer. `renderable` should
// be a struct or a pointer to a struct that has been registered with the engine.
func (e *Engine) Render(w io.Writer, renderable any) error {
	v := reflect.ValueOf(renderable)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if template, ok := e.templateMap[v.Type().Name()]; ok {
		err := template.Execute(w, renderable)
		if err != nil {
			return fmt.Errorf("error rendering component: %w", err)
		}

		return nil
	}

	return fmt.Errorf("No component found for type %s", v.Type().Name())
}

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

	e.components[name] = reflect.TypeOf(value)
	template, err := e.parseTemplate(name, templateString)
	if err != nil {
		return fmt.Errorf("could not register template: %w", err)
	}

	e.templateMap[name] = template.htmltemplate

	return nil
}

func (e *Engine) parseTemplate(name, templateValue string) (*goatTemplate, error) {
	t := &goatTemplate{
		Name: name,
	}

	// Normalize the template values for the parser+generator
	componentNames := make(map[string]bool, len(e.components))
	for k := range e.components {
		componentNames[k] = true
	}

	// Parse the template to populate t.content and ensure we wipe it afterwards
	t.parse(templateValue, componentNames)
	defer func() {
		t.pos = 0
		t.content = ""
	}()

	t.htmltemplate = htmltemplate.New(name)

	// setup the functions, the renderer needs to be able to render components
	// within the context of the engine and itself
	funcs := htmltemplate.FuncMap{
		"__goatRenderComponent": e.generateRenderFunc(t.htmltemplate, e.components),
	}
	for name, fn := range e.funcs {
		funcs[name] = fn
	}

	// Parse the template using the html/template parser
	var err error
	t.htmltemplate.Funcs(funcs)
	t.htmltemplate, err = t.htmltemplate.Parse(t.content)
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	return t, nil
}

func (e *Engine) generateRenderFunc(t *htmltemplate.Template, componentMap map[string]reflect.Type) func(string, string, map[string]any, any) htmltemplate.HTML {
	return func(name string, identifier string, attributes map[string]any, existingData any) htmltemplate.HTML {
		if _, ok := componentMap[name]; !ok {
			panic(fmt.Errorf("component %s not found", name))
		}

		// Get the type of the component, and if it's a pointer, get the underlying type
		// so we can create a new instance of it
		componentType := componentMap[name]
		isPointer := componentType.Kind() == reflect.Ptr
		if isPointer {
			componentType = componentType.Elem()
		}

		// Create a new instance of the component
		toRender := reflect.New(componentType)
		toCallRenderOn := toRender
		if isPointer {
			toRender = toRender.Elem()
		}

		// Loop through the attributes and set them on the component
		for i := 0; i < componentType.NumField(); i++ {
			fieldType := componentType.Field(i)
			field := toRender.Field(i)
			if !field.CanSet() {
				continue
			}

			if fieldType.Name == "Children" {
				var b bytes.Buffer
				err := t.ExecuteTemplate(&b, identifier, existingData)
				if err != nil {
					panic(err)
				}
				field.Set(reflect.ValueOf(htmltemplate.HTML(b.String())))
				continue
			}

			if value, ok := attributes[fieldType.Name]; ok {
				field.Set(reflect.ValueOf(value))
				continue
			}
		}

		var b bytes.Buffer
		err := e.Render(&b, toCallRenderOn.Interface())
		if err != nil {
			panic(err)
		}
		return htmltemplate.HTML(b.String())
	}

}

func (t *goatTemplate) skipWhitespace(runes []rune) {
	for unicode.IsSpace(runes[t.pos]) {
		t.pos++
	}
}
