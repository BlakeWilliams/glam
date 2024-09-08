package template

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"io"
	"reflect"
	"unicode"
)

type (
	Template struct {
		Name         string
		htmltemplate *htmltemplate.Template
		rawContent   string
		renderer     Renderer

		// these are temporary until we have compilde into an htmltemplate
		pos int

		// potentiallyReferencedComponents is a map of component names that are
		// referenced in the template, but not registered with the engine. This
		// allows us to track references and recompile components when dependent
		// components are registered.
		potentiallyReferencedComponents map[string]bool
	}

	Renderer interface {
		Render(io.Writer, any) error
		KnownComponents() map[string]reflect.Type
		FuncMap() htmltemplate.FuncMap
	}
)

func New(name string, r Renderer, rawTemplate string) (*Template, error) {
	t := &Template{
		Name:         name,
		htmltemplate: htmltemplate.New(name).Funcs(r.FuncMap()),
		rawContent:   rawTemplate,
		renderer:     r,
	}

	// Ensure this component doesn't conflict with an existing HTML tag since
	// this can break the recompilation strategy (because we don't consider
	// matching HTML tags a potentially rendered component, so don't recompile
	// dependencies upon registration)
	if knownHTMLTags.IsKnown(name) {
		return nil, fmt.Errorf("component %s conflicts with an existing HTML tag, consider suffixing it with Component", name)
	}

	err := t.parse()
	if err != nil {
		return nil, fmt.Errorf("could not parse template %s: %w", name, err)
	}

	return t, err
}

// Execute delegates to the underlying html/template
func (t *Template) Execute(w io.Writer, data any) error {
	return t.htmltemplate.Execute(w, data)
}

func (t *Template) ComponentsPotentiallyReferenced() map[string]bool {
	return t.potentiallyReferencedComponents
}

func (t *Template) RawContent() string {
	if t.rawContent == "" {
		panic("raw content not available after compilation")
	}

	return t.rawContent
}

// Parse parses the template into an AST and then into an html/template
// template. It also tracks any components that are referenced in the template
// so they can be recompiled if/when they are registered with the engine.
func (t *Template) parse() error {
	t.htmltemplate.Funcs(htmltemplate.FuncMap{
		"__glamRenderComponent": t.generateRenderFunc(),
	})

	t.potentiallyReferencedComponents = make(map[string]bool)

	// If we have no potentially referenced components that might require
	// recompilation, we can save some space and remove the content
	defer func() {
		t.pos = 0
		if len(t.potentiallyReferencedComponents) == 0 {
			t.rawContent = ""
		}
	}()

	// turn template into AST nodes
	nodes := t.parseRoot([]rune(t.rawContent), t.renderer.KnownComponents())

	// Turn nodes into an html/template compatible string
	content := compile(nodes)

	var err error
	t.htmltemplate, err = t.htmltemplate.Parse(content)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	return nil
}

func (t *Template) parseRoot(runes []rune, components map[string]reflect.Type) []*Node {
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

	return nodes
}

// ParseTag parses an HTML tag and either emits it, or generates the necessary
// code to render a component
func (t *Template) parseTag(runes []rune, components map[string]reflect.Type) (*Node, error) {
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
		for runes[t.pos] != ' ' && runes[t.pos] != '>' && runes[t.pos] != '/' {
			t.pos++
		}

		tagName := runes[tagNameStart:t.pos]

		attrs, err := t.parseAttributes(runes)
		if err != nil {
			return nil, fmt.Errorf("error parsing attributes: %w", err)
		}

		t.skipWhitespace(runes)

		switch runes[t.pos] {
		// we're in a self closing tag
		case '/':
			// skip the /
			t.pos++

			// Skip any accidental/unwanted whitespace
			t.skipWhitespace(runes)

			// Ensure we're actually closing the component
			if runes[t.pos] != '>' {
				return nil, fmt.Errorf("found invalid HTML")
			}

			// Skip the >
			t.pos++

			if _, ok := components[string(tagName)]; ok {
				return &Node{
					Type:       NodeTypeComponent,
					TagName:    string(tagName),
					Attributes: attrs,
					Children:   make([]*Node, 0),
				}, nil
			}
		// We're in a full tag
		case '>':
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

			// If this isn't just a capitalized HTML tag, keep track of this
			// potential component so we can recompile the template if it's
			// registered
			if !knownHTMLTags.IsKnown(string(tagName)) {
				t.potentiallyReferencedComponents[string(tagName)] = true
			}

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
	for runes[t.pos] != ' ' && runes[t.pos] != '>' && runes[t.pos] != '/' {
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

	// Check if we're self-closing and skip over it
	if runes[t.pos] == '/' {
		t.pos++
	}

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

	for runes[t.pos] != '>' && runes[t.pos] != '/' {
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
		// If we have a / we can consume it and subsequent whitespace and return attributes as-is
		case '/':
			t.pos++
			t.skipWhitespace(runes)
			attributes[string(name)] = "true"
			return attributes, nil
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

func (t *Template) parseUntilCloseTag(runes []rune, tagName []rune, components map[string]reflect.Type) ([]*Node, error) {
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
				// if we've captured any content
				if t.pos != start {
					nodes = append(nodes, &Node{
						Type: NodeTypeRaw,
						Raw:  string(runes[start : t.pos-1]),
					})
				}

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

func (t *Template) generateRenderFunc() func(string, string, map[string]any, any) htmltemplate.HTML {
	return func(name string, identifier string, attributes map[string]any, existingData any) htmltemplate.HTML {
		componentType, ok := t.renderer.KnownComponents()[name]
		if !ok {
			panic(fmt.Errorf("component %s not found", name))
		}

		// Get the type of the component, and if it's a pointer, get the underlying type
		// so we can create a new instance of it
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
				err := t.htmltemplate.ExecuteTemplate(&b, identifier, existingData)
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
		err := t.renderer.Render(&b, toCallRenderOn.Interface())
		if err != nil {
			panic(err)
		}
		return htmltemplate.HTML(b.String())
	}

}
