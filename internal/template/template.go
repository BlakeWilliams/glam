package template

import (
	"fmt"
	"unicode"
)

type TemplateParser struct {
	components map[string]bool
}

type template struct {
	Name    string
	pos     int
	content []rune
}

func (p *TemplateParser) ParseTemplate(name, templateValue string) error {
	t := &template{
		Name: name,
	}

	t.Parse(templateValue, p.components)

	return nil
}

func (t *template) String() string {
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

// ParseTag parses an HTML tag and either emits it, or generates the necessary
// code to render a component
func (t *template) parseTag(runes []rune, components map[string]bool) (*Node, error) {
	start := t.pos

	// We somehow got here without a <
	if runes[t.pos] != '<' {
		panic("unexpected < when parsing tag")
		return &Node{
			Type: NodeTypeRaw,
			Raw:  string(runes[start:t.pos]),
		}, nil
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
	if unicode.IsUpper(runes[t.pos]) && components[string(runes[t.pos])] {
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
	t.pos++

	return &Node{
		Type: NodeTypeRaw,
		Raw:  string(runes[start:t.pos]),
	}, nil
}

func (t *template) parseUntilCloseTag(runes []rune, tagName []rune, components map[string]bool) ([]*Node, error) {
	nodes := make([]*Node, 0)

	for {
		start := t.pos

		if t.pos >= len(runes) {
			panic("unclosed component tag")
		}

		switch runes[t.pos] {
		// we might be in a tag, which could be closing, could be another component, or could be an unescaped <
		case '<':
			if runes[t.pos+1] == '/' {
				// Capture end before we read the tag so we can emit the raw content
				// if we have a matching end tag
				end := t.pos - 1

				// skip the </
				t.pos += 2

				endTagStart := t.pos
				for runes[t.pos] != '>' {
					t.pos++
				}

				// skip the >
				t.pos++

				endTagName := runes[endTagStart:t.pos]
				// If we have a matching end tag, we can return the nodes
				if string(endTagName) == string(tagName) {
					nodes = append(nodes, &Node{
						Type: NodeTypeRaw,
						Raw:  string(runes[start:end]),
					})

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
			} else {
				t.pos++
			}
		}

	}
}

func (t *template) parseAttributes(runes []rune) (map[string]string, error) {
	attributes := make(map[string]string)
	t.skipWhitespace(runes)

	for runes[t.pos] != '>' {
		valueStart := t.pos
		// Loop until we find the end of the attribute which can be:
		//   - a space (boolean attribute)
		//   - a > (end of tag, also boolean attribute)
		//   - a = (quoted attribute, but there can also be "raw" attributes with no quotes)
		for !unicode.IsSpace(runes[t.pos]) && runes[t.pos] != '=' || runes[t.pos] == '>' {
			t.pos++
		}

		value := runes[valueStart:t.pos]

		// Skip any whitespace
		t.skipWhitespace(runes)

		switch runes[t.pos] {
		// If we have a > we can return the attributes as-is
		case '>':
			attributes[string(value)] = "true"
			return attributes, nil
		// If we have a ' ' we can set the boolean attribute and move on
		case ' ':
			attributes[string(value)] = "true"
			continue
		// If we have an = we need to find the end of the attribute value
		case '=':
			// Skip the equals sign
			t.pos++

			// Get the quote character and skip it
			quote := runes[t.pos]
			t.pos++

			// This needs to account for {{ }} and ignore quotes inside of it
			for runes[t.pos] != quote {
				t.pos++
			}

			attributes[string(value)] = string(runes[valueStart+1 : t.pos])

			// Skip the end quote
			t.pos++
		}

		// Skip any whitespace
		t.skipWhitespace(runes)
	}

	return attributes, nil
}

func (t *template) skipWhitespace(runes []rune) {
	for unicode.IsSpace(runes[t.pos]) {
		t.pos++
	}

	return
}

func (t *template) Parse(text string, components map[string]bool) {
	runes := []rune(text)
	nodes := make([]*Node, 0)

	start := t.pos
	for t.pos < len(runes) {
		if runes[t.pos] == '<' {
			nodes = append(nodes, &Node{
				Type: NodeTypeRaw,
				Raw:  string(runes[start : t.pos-1]),
			})
			t.parseComponent(runes, components)

			// Reset start so we can capture the next raw node
			start = t.pos
		} else {
			t.pos++
		}
	}

	fmt.Println(nodes)
}

// func (t *template) Parse(text string, components map[string]bool) {
// 	runes := []rune(text)

// 	for t.pos < len(runes) {
// 		if runes[t.pos] == '<' {
// 			t.parseComponent(runes, components)
// 		} else {
// 			t.content = append(t.content, runes[t.pos])
// 			t.pos++
// 		}
// 	}
// }

func (t *template) parseComponent(runes []rune, components map[string]bool) {
	start := t.pos

	// skip the <
	t.pos++

	if runes[t.pos] == '/' {
		t.content = append(t.content, runes[start:t.pos]...)
		return
	}
	tagName := make([]rune, 0, 6)

	for runes[t.pos] != ' ' && runes[t.pos] != '>' {
		tagName = append(tagName, runes[t.pos])
		t.pos++

		if t.pos >= len(runes) {
			panic("unexpected end of file parsing component tag")
		}
	}

	// This could be a component if it's uppercase
	if !unicode.IsUpper(tagName[0]) && !components[string(tagName)] {
		t.content = append(t.content, runes[start:t.pos]...)
		return
	}

	attributes := map[string]string{}

	for runes[t.pos] != '>' {
		// skip spaces
		for runes[t.pos] == ' ' {
			t.pos++
		}

		// If we're at the end of the file, we're missing a closing tag
		if t.pos >= len(runes) {
			panic("unexpected end of file parsing component attributes")
		}

		if runes[t.pos] == '>' {
			t.pos++
			break
		}

		// We're in an attribute, so we need to parse the key and value
		name := make([]rune, 0, 6)
		for runes[t.pos] != '=' && runes[t.pos] != ' ' && runes[t.pos] != '>' {
			name = append(name, runes[t.pos])
			t.pos++
		}

		switch runes[t.pos] {
		case ' ':
			// This is a boolean attribute
			attributes[string(name)] = "true"
		case '=':
			// Skip the equals sign
			t.pos++

			for runes[t.pos] == ' ' {
				t.pos++
			}

			if t.pos >= len(runes) {
				panic("unexpected end of file parsing component attribute value")
			}

			// This is a quoted attribute
			quote := runes[t.pos]
			if quote != '"' && quote != '\'' {
				panic("unexpected character parsing component attribute value, expected quote")
			}

			t.pos++

			value := make([]rune, 0, 6)
			for runes[t.pos] != quote {
				value = append(value, runes[t.pos])
				t.pos++

				if t.pos >= len(runes) {
					panic("unexpected end of file parsing component attribute value")
				}
			}

			attributes[string(name)] = string(value)

			t.pos++

			if t.pos >= len(runes) {
				panic("unexpected end of file parsing component attributes")
			}

			for runes[t.pos] == ' ' {
				t.pos++
			}
		}

		if runes[t.pos] == '>' {
			t.pos++
			break
		}

		// TODO: Actually emit some code!
		fmt.Println("tagname", string(tagName))
		fmt.Println("attrs", attributes)
	}

	if runes[t.pos] == '>' {
		t.pos++
	}

	// Now we need to parse the content of the component
	//t.parseUntilCloseTag(runes, string(tagName), components)
}

// func (t *template) parseUntilCloseTag(text []rune, expected string, components map[string]bool) {
// 	start := t.pos

// 	for {
// 		if t.pos >= len(text) {
// 			panic("unclosed component tag")
// 		}

// 		switch text[t.pos] {
// 		// we might be in a tag
// 		case '<':
// 			t.content = append(t.content, text[start:t.pos]...)
// 			start = t.pos

// 			t.parseComponent(text, components)
// 			start = t.pos
// 			fmt.Println("YO")
// 			fmt.Println(string(t.content))

// 			// switch text[t.pos] {
// 			// case '/':
// 			// 	t.pos++

// 			// 	name := make([]rune, 0, 6)

// 			// 	for text[t.pos] != '>' {
// 			// 		name = append(name, text[t.pos])
// 			// 		t.pos++
// 			// 	}

// 			// 	// skip the >
// 			// 	t.pos++

// 			// 	if strings.TrimSpace(string(name)) == expected {
// 			// 		return
// 			// 	}

// 			// case ' ':
// 			// 	t.pos++
// 			// default:
// 			// 	if unicode.IsUpper(text[t.pos]) {
// 			// 		t.parseComponent(text, components)
// 			// 		start = t.pos
// 			// 		continue outer
// 			// 	}
// 			// 	t.pos++
// 			// }
// 		default:
// 			t.pos++
// 		}

// 		// if text[t.pos] == '<' {
// 		// 	t.parseComponent(text, components)
// 		// } else {
// 		// 	t.content = append(t.content, text[t.pos])
// 		// 	t.pos++
// 		// }
// 	}

// 	t.content = append(t.content, text[start:t.pos]...)
// }
