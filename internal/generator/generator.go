package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"strings"
	stdtemplate "text/template"
)

var errNoComponents = fmt.Errorf("no components found")

// component is a struct that can be rendered as a component
// we need the struct name and the file name to generate the correct
// `Render` method
type component struct {
	StructName       string
	TemplateFileName string
	packageName      string
	path             string
}

// TemplateContent returns the content of the template file as a string
// we need to know which components are valid so we can identify components
// that are valid and can be rendered using the <ComponentName> syntax
func (c component) TemplateContent(validComponents map[string]bool) string {
	f, err := os.Open(c.path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	rawContent, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	content := strings.Replace(string(rawContent), `"`, `\"`, -1)
	content = strings.Replace(content, `\`, `\\`, -1)
	content = strings.Replace(content, "\n", `\n`, -1)
	return content
}

// compile reads the go files in the given directory and generates the relevant
// `Render` methods for structs marked as components via `glam:component`.
func Compile(directory string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	componentsToGenerate := make([]component, 0, 10)

	for _, file := range files {
		// We don't recursively walk directories yet
		if file.IsDir() {
			continue
		}

		// We only care about go files
		if file.Name()[len(file.Name())-3:] != ".go" {
			continue
		}

		// Ignore test files
		if strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}

		// Don't read ourselves
		if file.Name() == "generated.go" {
			continue
		}

		filePath := path.Join(directory, file.Name())

		components, err := componentsFromFile(filePath)
		if err == errNoComponents {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		componentsToGenerate = append(componentsToGenerate, components...)

	}

	if len(componentsToGenerate) == 0 {
		return fmt.Errorf("no components found")
	}

	f, err := os.OpenFile(path.Join(directory, "generated.go"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for writing: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(generateFile(componentsToGenerate))
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func componentsFromFile(file string) ([]component, error) {
	fmt.Println("Inspecting file", file)

	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, f, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	components := make([]component, 0, 10)
	packageName := node.Name.Name

	ast.Inspect(node, func(n ast.Node) bool {

		gd, ok := n.(*ast.GenDecl)
		// If we're not in a GenDecl or a GenDecl for a type, we can move on
		if !ok || gd.Tok != token.TYPE {
			return true
		}

		// If there is only 1 spec, it might be a struct where the
		// GenDecl has consumed the comment for us
		if len(gd.Specs) == 1 {
			// Ensure we're looking at a `type` spec
			ts, ok := gd.Specs[0].(*ast.TypeSpec)
			if !ok {
				return true
			}

			// Ensure we're looking at a struct
			if _, ok := ts.Type.(*ast.StructType); !ok {
				return true
			}

			// First Name gets `Ident` and the second gets `string`
			structName := ts.Name.Name

			// If we have no doc, we can move on
			if gd.Doc == nil {
				return true
			}

			// find the glam:component comment if any, and add it to the comment map
			for _, comment := range gd.Doc.List {
				if strings.HasPrefix(comment.Text, "//glam:component") {
					name := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//glam:component"))
					if name == "" {
						fmt.Printf("WARNING: glam:component comment found for `%s`, but no template name provided", structName)
					}

					components = append(
						components,
						component{
							StructName:       structName,
							TemplateFileName: name,
							packageName:      packageName,
							path:             path.Join(path.Dir(file), name),
						},
					)
				}
			}

			return true
		}

		// If we have more than 1 spec, we might be looking at types in a `type
		// ()` block. The GenDecl _doesn't_ consume the comment in this case,
		// but the spec will
		if len(gd.Specs) > 1 {
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				// Ensure we're looking at a struct
				if _, ok := ts.Type.(*ast.StructType); !ok {
					continue
				}

				// First Name gets `Ident` and the second gets `string`
				structName := ts.Name.Name

				// If we have no doc, we can move on
				if ts.Doc == nil {
					continue
				}

				for _, comment := range ts.Doc.List {
					if strings.HasPrefix(comment.Text, "//glam:component") {
						name := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//glam:component"))
						if name == "" {
							fmt.Printf("WARNING: glam:component comment found for `%s`, but no template name provided", structName)
						}

						components = append(
							components,
							component{
								StructName:       structName,
								TemplateFileName: name,
								packageName:      packageName,
								path:             path.Join(path.Dir(file), name),
							},
						)
					}
				}
			}

			return true
		}

		return true
	})

	if len(components) == 0 {
		return nil, errNoComponents
	}

	return components, nil
}

func generateFile(components []component) string {
	tmpl := stdtemplate.Must(stdtemplate.New("file").Parse(`package {{.PackageName}}

	import (
		"fmt"
		"github.com/blakewilliams/glam/template"
		stdtemplate "html/template"
	)

	func NewEngine(funcs stdtemplate.FuncMap) (*template.Engine, error) {
		e := template.New(funcs)
		var err error
		{{ range .Components }}
			err = e.RegisterComponent(&{{.StructName}}{}, "{{.TemplateContent $.ComponentNames}}")
			if err != nil {
				return nil, fmt.Errorf("failed to register component {{.StructName}}: %w", err)
			}
		{{ end }}

		return e, nil
	}
	`))

	var b bytes.Buffer

	componentNames := make(map[string]bool, len(components))
	for _, c := range components {
		componentNames[c.StructName] = true
	}

	components[0].TemplateContent(componentNames)

	err := tmpl.Execute(&b, struct {
		PackageName    string
		Components     []component
		ComponentNames map[string]bool
	}{
		PackageName:    components[0].packageName,
		Components:     components,
		ComponentNames: componentNames,
	})

	if err != nil {
		panic(err)
	}

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		panic(err)
	}

	return string(formatted)
}
