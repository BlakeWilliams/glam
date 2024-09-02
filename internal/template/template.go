package template

import (
	"fmt"
	"strings"
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

func (t *template) Parse(text string, components map[string]bool) {
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

	for _, n := range nodes {
		fmt.Println(n)

	}
}

// ParseTag parses an HTML tag and either emits it, or generates the necessary
// code to render a component
func (t *template) parseTag(runes []rune, components map[string]bool) (*Node, error) {
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

			if components[string(tagName)] {
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

			} else {
				// skip the >
				t.pos++

				return &Node{
					Type: NodeTypeRaw,
					Raw:  string(runes[start:t.pos]),
				}, nil
			}
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

func (t *template) parseAttributes(runes []rune) (map[string]string, error) {
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

		value := runes[nameStart:t.pos]

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
			// Skip the =
			t.pos++

			// Get the quote character and skip it
			quote := runes[t.pos]
			t.pos++

			valueStart := t.pos

			// TODO: This needs to account for {{ }} and ignore quotes inside of it
			for runes[t.pos] != quote {
				t.pos++
			}

			attributes[string(value)] = string(runes[valueStart:t.pos])

			// Skip the end quote
			t.pos++
		}

		// Skip any whitespace
		t.skipWhitespace(runes)
	}

	return attributes, nil
}

func (t *template) parseUntilCloseTag(runes []rune, tagName []rune, components map[string]bool) ([]*Node, error) {
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

				fmt.Println("end tag", string(endTagName))
				// If we have a matching end tag, we can return the nodes
				if string(endTagName) == string(tagName) {
					// If start == end we immediately ran into a closing tag, so
					// we can skip emitting raw content
					if start != end {
						fmt.Println("raw", string(runes[start:end]))
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

func (t *template) skipWhitespace(runes []rune) {
	for unicode.IsSpace(runes[t.pos]) {
		t.pos++
	}
}

// func (t *template) parseComponent(runes []rune, components map[string]bool) *Node {
// 	start := t.pos

// 	// skip the <
// 	t.pos++

// 	if runes[t.pos] == '/' {
// 		return &Node{
// 			Type: NodeTypeRaw,
// 			Raw:  string(runes[start : t.pos-1]),
// 		}
// 	}
// 	tagName := make([]rune, 0, 6)

// 	for runes[t.pos] != ' ' && runes[t.pos] != '>' {
// 		tagName = append(tagName, runes[t.pos])
// 		t.pos++

// 		if t.pos >= len(runes) {
// 			panic("unexpected end of file parsing component tag")
// 		}
// 	}

// 	// This is a raw tag, so we can
// 	if !unicode.IsUpper(tagName[0]) && !components[string(tagName)] {
// 		return
// 	}

// 	attributes := map[string]string{}

// 	for runes[t.pos] != '>' {
// 		// skip spaces
// 		for runes[t.pos] == ' ' {
// 			t.pos++
// 		}

// 		// If we're at the end of the file, we're missing a closing tag
// 		if t.pos >= len(runes) {
// 			panic("unexpected end of file parsing component attributes")
// 		}

// 		if runes[t.pos] == '>' {
// 			t.pos++
// 			break
// 		}

// 		// We're in an attribute, so we need to parse the key and value
// 		name := make([]rune, 0, 6)
// 		for runes[t.pos] != '=' && runes[t.pos] != ' ' && runes[t.pos] != '>' {
// 			name = append(name, runes[t.pos])
// 			t.pos++
// 		}

// 		switch runes[t.pos] {
// 		case ' ':
// 			// This is a boolean attribute
// 			attributes[string(name)] = "true"
// 		case '=':
// 			// Skip the equals sign
// 			t.pos++

// 			for runes[t.pos] == ' ' {
// 				t.pos++
// 			}

// 			if t.pos >= len(runes) {
// 				panic("unexpected end of file parsing component attribute value")
// 			}

// 			// This is a quoted attribute
// 			quote := runes[t.pos]
// 			if quote != '"' && quote != '\'' {
// 				panic("unexpected character parsing component attribute value, expected quote")
// 			}

// 			t.pos++

// 			value := make([]rune, 0, 6)
// 			for runes[t.pos] != quote {
// 				value = append(value, runes[t.pos])
// 				t.pos++

// 				if t.pos >= len(runes) {
// 					panic("unexpected end of file parsing component attribute value")
// 				}
// 			}

// 			attributes[string(name)] = string(value)

// 			t.pos++

// 			if t.pos >= len(runes) {
// 				panic("unexpected end of file parsing component attributes")
// 			}

// 			for runes[t.pos] == ' ' {
// 				t.pos++
// 			}
// 		}

// 		if runes[t.pos] == '>' {
// 			t.pos++
// 			break
// 		}

// 		// TODO: Actually emit some code!
// 		fmt.Println("tagname", string(tagName))
// 		fmt.Println("attrs", attributes)
// 	}

// 	if runes[t.pos] == '>' {
// 		t.pos++
// 	}

// 	// Now we need to parse the content of the component
// 	//t.parseUntilCloseTag(runes, string(tagName), components)
// }
