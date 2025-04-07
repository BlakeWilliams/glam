package template

import (
	"crypto/rand"
	"fmt"
	"strings"
	"unicode"
)

var rootPrefix = "glam__root__"
var localsPrefix = "glam__locals__"
var dotPrefix = "glam__dot__"

type define struct {
	Node       *Node
	identifier string
	locals     []string
}

func newDefine(node *Node) *define {
	return &define{
		Node:       node,
		identifier: fmt.Sprintf("glam__%s__%s", node.TagName, randomString()),
	}
}
func compile(nodes []*Node) string {
	primaryContent, defines := rawCompile(nodes, false)
	defineText := strings.Join(defines, "")

	return primaryContent + defineText
}

// rawCompile accepts nodes and returns primaryContent, which is rendered in the
// immediate context, and defineContent, which is content that must be wrapped
// in a `{{define}}` statement, so it can be rendered and passed to a component
// as `Children`.
func rawCompile(nodes []*Node, subdefine bool) (primaryContent string, defineContent []string) {
	// defineReferences is a map of components that need a {{define}} statement so
	// they can be passed child nodes as HTML text
	defineReferences := make(map[string]*define)
	var rawContent strings.Builder

	for _, node := range nodes {
		switch {
		case node.Type == NodeTypeRaw:
			rawContent.WriteString(node.Raw)
		case node.Type == NodeTypeComponent && len(node.Children) > 0:
			definition := newDefine(node)
			defineReferences[definition.identifier] = definition

			var attributes strings.Builder

			attributes.WriteString(`(__glamDict`)

			for k, v := range node.Attributes {
				if strings.HasPrefix(v, "{{") {
					if subdefine {
						s, _ := rewriteTemplateRunes([]rune(v))
						v = strings.Trim(string(s), "{}")
					} else {
						v = strings.Trim(v, "{} ")
					}
					attributes.WriteString(fmt.Sprintf(` "%s" (%s)`, k, v))
					continue
				}
				attributes.WriteString(fmt.Sprintf(` "%s" "%s"`, k, v))
			}

			attributes.WriteString(`)`)

			var defineArgs strings.Builder
			if !subdefine {
				nodes, locals := rewriteChildren([]*Node{node}, 0)
				// rewrite variable, but don't change the node
				node = nodes[0]
				definition.locals = locals

				{
					defineArgs.WriteString(`(__glamDict`)
					// TODO this doesn't have to be reformatted every time
					defineArgs.WriteString(fmt.Sprintf(` "%s" . "%s" $ `, dotPrefix, rootPrefix))
					defineArgs.WriteString(fmt.Sprintf(`"%s" (__glamDict `, localsPrefix))
					for _, local := range locals {
						defineArgs.WriteString(fmt.Sprintf(`"%s" $%s `, local, local))
					}
					defineArgs.WriteString(`))`)

				}
			} else {
				defineArgs.WriteString(`.`)
			}

			rawContent.WriteString(fmt.Sprintf(`{{__glamRenderComponent "%s" "%s" %s %s}}`, node.TagName, definition.identifier, attributes.String(), defineArgs.String()))
		case node.Type == NodeTypeComponent && len(node.Children) == 0:
			rawContent.WriteString(fmt.Sprintf(`{{__glamRenderComponent "%s" "" nil .}}`, node.TagName))
		}
	}

	// Now that we have all the define references, we need to render them
	// in the correct order and in their own fragments since nesting defines
	// is not allowed
	defineCalls := make([]string, 0, len(defineReferences))
	for _, definition := range defineReferences {
		var currentContent strings.Builder
		currentDefineContent, subDefines := rawCompile(definition.Node.Children, true)

		currentContent.WriteString(fmt.Sprintf(`{{define "%s"}}%s{{end}}`, definition.identifier, currentDefineContent))
		defineCalls = append(defineCalls, subDefines...)
		defineCalls = append(defineCalls, currentContent.String())
	}

	return rawContent.String(), defineCalls
}

func rewriteChildren(nodes []*Node, depth int) ([]*Node, []string) {
	locals := make([]string, 0)

	for i, node := range nodes {
		if node.Type == NodeTypeRaw || (node.Type == NodeTypeComponent) {
			raw, newLocals := rewriteTemplateRunes([]rune(node.Raw))
			nodes[i].Raw = string(raw)
			locals = append(locals, newLocals...)
		}

		children, newLocals := rewriteChildren(node.Children, depth+1)
		nodes[i].Children = children
		locals = append(locals, newLocals...)
	}
	return nodes, locals
}

func rewriteTemplateRunes(input []rune) ([]rune, []string) {
	var out []rune
	inAction := false
	inString := false
	inTemplateString := false
	locals := make([]string, 0)

	i := 0
	for i < len(input) {
		if inString {
			if input[i] == '"' && input[i-1] != '\\' {
				inString = false
				out = append(out, input[i])
				i++
				continue
			}

			out = append(out, input[i])
			i++
			continue
		}

		if inTemplateString {
			if input[i] == '`' {
				inString = false
				out = append(out, input[i])
				i++
				continue
			}

			out = append(out, input[i])
			i++
			continue
		}

		if i+1 < len(input) && input[i] == '{' && input[i+1] == '{' {
			inAction = true
			out = append(out, input[i], input[i+1])
			i += 2
			continue
		}
		if inAction && i+1 < len(input) && input[i] == '}' && input[i+1] == '}' {
			inAction = false
			out = append(out, input[i], input[i+1])
			i += 2
			continue
		}

		if inAction {
			if input[i] == '$' {
				i++

				j := i
				for j < len(input) && isIdentRune(input[j], j-i) {
					j++
				}

				if j == i {
					out = append(out, []rune(fmt.Sprintf(".%s", rootPrefix))...)
					if input[i] == '.' {
						out = append(out, []rune(".")...)
						j++
					}
				} else {
					name := string(input[i:j])
					locals = append(locals, name)
					out = append(out, []rune("."+localsPrefix+"."+name)...)
				}
				i = j
			} else if input[i] == '.' {
				i++
				j := i
				for j < len(input) && isIdentRune(input[j], j-i) {
					j++
				}
				if j == i {
					out = append(out, []rune("."+dotPrefix)...)
				} else {
					field := string(input[i:j])
					out = append(out, []rune("."+dotPrefix+"."+field)...)
				}
				i = j
			} else if input[i] == '"' {
				inString = true
				out = append(out, input[i])
				i++
			} else if input[i] == '`' {
				inTemplateString = true
				out = append(out, input[i])
				i++
			} else {
				out = append(out, input[i])
				i++
			}
		} else {
			out = append(out, input[i])
			i++
		}
	}
	return out, locals
}

func isIdentRune(r rune, pos int) bool {
	if pos == 0 {
		return unicode.IsLetter(r) || r == '_'
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func randomString() string {
	b := make([]byte, 9)

	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return fmt.Sprintf("%x", b)
}
