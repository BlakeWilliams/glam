package template

import (
	"fmt"
	htmltemplate "html/template"
	"unicode"
)

type goatTemplate struct {
	Name         string
	htmltemplate *htmltemplate.Template
	rawContent   string

	// these are temporary until we have compilde into an htmltemplate
	pos int

	// potentialReferencedComponents is a map of component names that are
	// referenced in the template, but not registered with the engine. This
	// allows us to track references and recompile components when dependent
	// components are registered.
	potentialReferencedComponents map[string]bool
}

func newTemplate(name string, rawTemplate string) *goatTemplate {
	return &goatTemplate{
		Name:         name,
		htmltemplate: htmltemplate.New(name),
		rawContent:   rawTemplate,
	}
}

func (t *goatTemplate) parse(funcs htmltemplate.FuncMap, components map[string]bool) error {
	t.potentialReferencedComponents = make(map[string]bool)

	// If we have no potentially referenced components that might require
	// recompilation, we can save some space and remove the content
	defer func() {
		t.pos = 0
		if len(t.potentialReferencedComponents) == 0 {
			t.rawContent = ""
		}
	}()

	// turn template into AST nodes
	nodes := t.parseRoot([]rune(t.rawContent), components)

	// Turn nodes into an html/template compatible string
	content := compile(nodes)

	var err error
	t.htmltemplate.Funcs(funcs)
	t.htmltemplate, err = t.htmltemplate.Parse(content)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	return nil
}

func (t *goatTemplate) parseRoot(runes []rune, components map[string]bool) []*Node {
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
func (t *goatTemplate) parseTag(runes []rune, components map[string]bool) (*Node, error) {
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

			// Keep track of this potential component so we can recompile the
			// template if it's registered
			t.potentialReferencedComponents[string(tagName)] = true

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

func (t *goatTemplate) parseAttributes(runes []rune) (map[string]string, error) {
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

func (t *goatTemplate) parseQuotedAttribute(runes []rune) ([]rune, error) {
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

func (t *goatTemplate) skipGoTemplate(runes []rune) {
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

func (t *goatTemplate) parseUntilCloseTag(runes []rune, tagName []rune, components map[string]bool) ([]*Node, error) {
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
