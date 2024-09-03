package template

import (
	"fmt"
	"strings"
)

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
