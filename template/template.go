package template

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"io"
	"reflect"
	"strings"
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

	if funcs != nil {
		for k, v := range funcs {
			e.funcs[k] = v
		}
	}

	return e
}

func generateRenderFunc(e *Engine, t htmltemplate.Template, componentMap map[string]reflect.Type) func(string, string, map[string]any, any) htmltemplate.HTML {
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

// Render renders the provided toRender value to the provided writer. `toRender` should
// be a struct or a pointer to a struct that has been registered with the engine.
func (e *Engine) Render(w io.Writer, toRender any) error {
	v := reflect.ValueOf(toRender)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if template, ok := e.templateMap[v.Type().Name()]; ok {
		err := template.Execute(w, toRender)
		if err != nil {
			return fmt.Errorf("error rendering component: %w", err)
		}

		return nil
	}

	return fmt.Errorf("No component found for type %s", v.Type().Name())
}

type Template struct {
	Name         string
	htmltemplate *htmltemplate.Template

	// these are temporary until we have compilde into an htmltemplate
	pos     int
	content string
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

func (e *Engine) parseTemplate(name, templateValue string) (*Template, error) {
	t := &Template{
		Name: name,
	}

	// Normalize the template values for the parser+generator
	componentNames := make(map[string]bool, len(e.components))
	for k := range e.components {
		componentNames[k] = true
	}

	// Parse the template to populate t.content and ensure we wipe it afterwards
	t.Parse(templateValue, componentNames)
	defer func() {
		t.pos = 0
		t.content = ""
	}()

	t.htmltemplate = htmltemplate.New(name)

	// setup the functions, the renderer needs to be able to render components
	// within the context of the engine and itself
	funcs := htmltemplate.FuncMap{
		"__goatRenderComponent": generateRenderFunc(e, *t.htmltemplate, e.components),
	}
	for name, fn := range e.funcs {
		funcs[name] = fn
	}

	// Parse the template using the html/template parser
	var err error
	t.htmltemplate.Funcs(funcs)
	t.htmltemplate, err = t.htmltemplate.Parse(t.String())
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	return t, nil
}

func (t *Template) String() string {
	return string(t.content)

}

type NodeType int

const (
	NodeTypeComponent = iota
	NodeTypeRaw       = iota
)

// Node represents a single node in the template, which is either a component or raw HTML
type Node struct {
	Type NodeType
	// TagName is the name of the component, if this is a component type
	TagName string
	// Attributes is a map of the attributes of the component, if this is a component type
	Attributes map[string]string
	// Children is a list of child nodes, if this is a component type
	Children []*Node
	// Raw is the raw HTML content of this node, if this is a raw type
	Raw string
}

func (n *Node) String() string {
	var b strings.Builder

	typeName := "Component"
	if n.Type == NodeTypeRaw {
		typeName = "Raw"
	}

	b.WriteString("Node{\n")
	switch n.Type {
	case NodeTypeComponent:
		b.WriteString(fmt.Sprintf("  TagName: %s\n", n.TagName))
		b.WriteString(fmt.Sprintf("  Attributes: %s\n", n.Attributes))
		for _, c := range n.Children {
			parts := strings.Split(c.String(), "\n")
			for i, p := range parts {
				parts[i] = fmt.Sprintf("  %s", p)
			}
			b.WriteString(fmt.Sprintf("  Children: %s\n", strings.Join(parts, "\n")))
		}
	case NodeTypeRaw:
		b.WriteString(fmt.Sprintf("  Type: %s\n", typeName))
		b.WriteString(fmt.Sprintf("  Content: \"%s\"\n", n.Raw))
	}

	b.WriteString("}")

	return b.String()
}

func (t *Template) Parse(text string, components map[string]bool) {
	runes := []rune(text)
	nodes := make([]*Node, 0)

	start := t.pos
	for t.pos < len(runes) {
		if runes[t.pos] == '<' {
			if start != t.pos {
				nodes = append(nodes, &Node{
					Type: NodeTypeRaw,
					Raw:  string(runes[start:t.pos]),
				})
			}
			n, err := t.parseTag(runes, components)
			if err != nil {
				panic(err)
			}
			nodes = append(nodes, n)

			// Reset start so we can capture the next raw node
			start = t.pos
		} else {
			t.pos++
		}
	}

	t.content = compile(nodes)
}

// ParseTag parses an HTML tag and either emits it, or generates the necessary
// code to render a component
func (t *Template) parseTag(runes []rune, components map[string]bool) (*Node, error) {
	start := t.pos

	// We somehow got here without a <, this is a bug
	if runes[t.pos] != '<' {
		panic("unexpected < when parsing tag, this is a bug in the parser")
	}

	// skip the <
	t.pos++

	// If we're in a closing tag, we can just emit it
	if runes[t.pos] == '/' {
		for runes[t.pos] != '>' {
			t.pos++
		}

		// skip the >
		t.pos++

		// Return the raw content of the tag
		return &Node{
			Type: NodeTypeRaw,
			Raw:  string(runes[start:t.pos]),
		}, nil
	}

	// If we have a matching component, we need to generate the relevant code and omit the tag
	// and the end tag from the output
	if unicode.IsUpper(runes[t.pos]) {
		tagNameStart := t.pos

		// loop until we find the end of tag name
		for runes[t.pos] != ' ' && runes[t.pos] != '>' {
			t.pos++
		}

		tagName := runes[tagNameStart:t.pos]

		attrs, err := t.parseAttributes(runes)
		if err != nil {
			return nil, fmt.Errorf("error parsing attributes: %w", err)
		}

		// if we have no attributes, we can just skip to the end of the tag
		if runes[t.pos] == '>' {
			// There's a choice to be made here, we could either:
			//   - Parse the tag strictly until we find an end tag
			//   - Continue reading "raw" content until we find another tag to parse
			//
			// This currently chooses the latter which is less strict and more
			// error prone, but results in a faster implementation for now

			// skip the >
			t.pos++

			// If we have a matching component, we need to return a component node instead
			// of a raw node, which includes parsing content until we find the
			// relevant end tag so it can be lifted into a `define` block later.
			if _, ok := components[string(tagName)]; ok {
				children, err := t.parseUntilCloseTag(runes, tagName, components)
				if err != nil {
					return nil, fmt.Errorf("error parsing children: %w", err)
				}

				return &Node{
					Type:       NodeTypeComponent,
					TagName:    string(tagName),
					Attributes: attrs,
					Children:   children,
				}, nil

			}

			// skip the >
			t.pos++

			return &Node{
				Type: NodeTypeRaw,
				Raw:  string(runes[start:t.pos]),
			}, nil
		}
	}

	// if we're here we're in a raw tag, we need to:
	//   - Get past the tag name
	//   - Parse the attributes

	// loop until we find the end of tag name
	for runes[t.pos] != ' ' && runes[t.pos] != '>' {
		t.pos++
	}

	// If we're here, we're in a raw tag, so we need to parse the content until
	// we find another opening tag. We'll parse the attributes though, so we can
	// skip them without worrying too much about quotes
	_, err := t.parseAttributes(runes)

	if err != nil {
		return nil, fmt.Errorf("error parsing attributes: %w", err)
	}
	t.skipWhitespace(runes)

	// We would expect to find a > here, so let's double check and skip it
	if runes[t.pos] != '>' {
		panic("unexpected character when parsing tag")
	}
	// skip the >
	t.pos++

	return &Node{
		Type: NodeTypeRaw,
		Raw:  string(runes[start:t.pos]),
	}, nil
}

func (t *Template) parseAttributes(runes []rune) (map[string]string, error) {
	attributes := make(map[string]string)

	// If we have a > we can return the attributes as-is
	if runes[t.pos] == '>' {
		return attributes, nil
	}

	t.skipWhitespace(runes)

	for runes[t.pos] != '>' {
		nameStart := t.pos
		// Loop until we find the end of the attribute which can be:
		//   - a space (boolean attribute)
		//   - a > (end of tag, also boolean attribute)
		//   - a = (quoted attribute, but there can also be "raw" attributes with no quotes)
		for !unicode.IsSpace(runes[t.pos]) && runes[t.pos] != '=' || runes[t.pos] == '>' {
			t.pos++
		}

		name := runes[nameStart:t.pos]

		switch runes[t.pos] {
		// If we have a > we can return the attributes as-is
		case '>':
			attributes[string(name)] = "true"
			return attributes, nil
		// If we have a ' ' we can set the boolean attribute and move on
		case ' ':
			// TODO check if there's an equal sign after this space
			t.skipWhitespace(runes)

			attributes[string(name)] = "true"
			continue
		// If we have an = we need to find the end of the attribute value
		case '=':
			// Skip the =
			t.pos++

			value, err := t.parseQuotedAttribute(runes)
			if err != nil {
				return nil, fmt.Errorf("error parsing quoted attribute: %w", err)
			}

			attributes[string(name)] = string(value)
		}

		// Skip any whitespace
		t.skipWhitespace(runes)
	}

	return attributes, nil
}

func (t *Template) parseQuotedAttribute(runes []rune) ([]rune, error) {
	// Get the quote character and skip it
	// TODO: this could be a "quoteless" attribute, so we need to handle that at
	// some point
	quote := runes[t.pos]
	t.pos++

	valueStart := t.pos

	for {
		switch runes[t.pos] {
		// We're at the end of the tag, so we can just return
		case quote:
			value := runes[valueStart:t.pos]

			// skip the close quote
			t.pos++

			return value, nil
		// We might have a go template tag which means we need to handle quotes
		// inside of it
		case '{':
			if runes[t.pos+1] == '{' {
				t.skipGoTemplate(runes)
			}
		default:
			t.pos++
		}
	}
}

func (t *Template) skipGoTemplate(runes []rune) {
	// skip the {{
	t.pos += 2

	// This is a bit naive, but we're just going to skip until we find the end
	// of the tag ignoring any potential }} values inside of it that may be part
	// of string literals
	for runes[t.pos] != '}' && runes[t.pos+1] != '}' {
		t.pos++
	}

	// skip the }}
	t.pos += 2
}

func (t *Template) parseUntilCloseTag(runes []rune, tagName []rune, components map[string]bool) ([]*Node, error) {
	nodes := make([]*Node, 0)

	start := t.pos
	for {
		if t.pos >= len(runes) {
			panic("unclosed component tag")
		}

		switch runes[t.pos] {
		// we might be in a tag, which could be closing, could be another component, or could be an unescaped <
		case '<':
			if runes[t.pos+1] == '/' {
				// Capture end before we read the tag so we can emit the raw content
				// if we have a matching end tag
				end := t.pos

				// skip the </
				t.pos += 2

				endTagStart := t.pos
				for runes[t.pos] != '>' {
					t.pos++
				}

				// Capture the end tag name before the >
				endTagName := runes[endTagStart:t.pos]

				// skip the >
				t.pos++

				// If we have a matching end tag, we can return the nodes
				if string(endTagName) == string(tagName) {
					// If start == end we immediately ran into a closing tag, so
					// we can skip emitting raw content
					if start != end {
						nodes = append(nodes, &Node{
							Type: NodeTypeRaw,
							Raw:  string(runes[start:end]),
						})
					}

					// TODO we need to emit the already captured nodes too
					return nodes, nil
				}
			} else if unicode.IsLetter(runes[t.pos+1]) {
				// We're about to run another parser, so we need to capture the raw content
				nodes = append(nodes, &Node{
					Type: NodeTypeRaw,
					Raw:  string(runes[start : t.pos-1]),
				})

				// We have a tag, so we need to parse it
				n, err := t.parseTag(runes, components)
				if err != nil {
					return nil, fmt.Errorf("error parsing tag: %w", err)
				}
				nodes = append(nodes, n)

				start = t.pos
			} else {
				t.pos++
			}
		default:
			t.pos++
		}

	}
}

func (t *Template) skipWhitespace(runes []rune) {
	for unicode.IsSpace(runes[t.pos]) {
		t.pos++
	}
}
