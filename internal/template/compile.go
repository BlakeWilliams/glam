package template

import (
	"crypto/rand"
	"fmt"
	"strings"
)

type define struct {
	Node       *Node
	identifier string
}

func newDefine(node *Node) *define {
	return &define{
		Node:       node,
		identifier: fmt.Sprintf("glam__%s__%s", node.TagName, randomString()),
	}
}
func compile(nodes []*Node) string {
	primaryContent, defines := rawCompile(nodes)

	defineText := strings.Join(defines, "")

	return defineText + primaryContent
}

// rawCompile accepts nodes and returns primaryContent, which is rendered in the
// immediate context, and defineContent, which is content that must be wrapped
// in a `{{define}}` statement, so it can be rendered and passed to a component
// as `Children`.
func rawCompile(nodes []*Node) (primaryContent string, defineContent []string) {
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
					v = strings.Trim(v, "{} ")
					attributes.WriteString(fmt.Sprintf(` "%s" (%s)`, k, v))
					continue
				}
				attributes.WriteString(fmt.Sprintf(` "%s" "%s"`, k, v))
			}

			attributes.WriteString(`)`)
			rawContent.WriteString(fmt.Sprintf(`{{__glamRenderComponent "%s" "%s" %s .}}`, node.TagName, definition.identifier, attributes.String()))
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
		currentDefineContent, subDefines := rawCompile(definition.Node.Children)

		currentContent.WriteString(fmt.Sprintf(`{{define "%s"}}%s{{end}}`, definition.identifier, currentDefineContent))
		defineCalls = append(defineCalls, subDefines...)
		defineCalls = append(defineCalls, currentContent.String())
	}

	return rawContent.String(), defineCalls
}

func randomString() string {
	b := make([]byte, 9)

	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return fmt.Sprintf("%x", b)
}
