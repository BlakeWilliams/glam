package template

import (
	"bytes"
	"fmt"
	"testing"

	htmltemplate "html/template"

	"github.com/stretchr/testify/require"
)

func TestTemplateParse(t *testing.T) {
	parser := &TemplateParser{
		components: map[string]bool{
			"Greeter": true,
		},
	}

	// err := parser.ParseTemplate(strings.NewReader(`<b>Hello <Yell name="{.Name}"></Yell></b>`))
	err := parser.ParseTemplate("main", `
	{{ range $index, $element := .Value }}
	 	<b>Hi {{.Foo}}</b>
	{{ end }}
	`)
	require.NoError(t, err)

	require.Fail(t, "omg")
}

func TestTemplateParse_WithContent(t *testing.T) {
	parser := &TemplateParser{
		components: map[string]bool{
			"wrapper": true,
		},
	}

	err := parser.ParseTemplate("main", `<b>Hello <WrapperComponent>Foo</WrapperComponent></b>`)
	require.NoError(t, err)

	require.Fail(t, "omg")
}

func TestTemplateParse_omg(t *testing.T) {
	template := &template{Name: "test"}
	template.Parse(`<b>Hello <WrapperComponent>Foo</WrapperComponent></b>`, map[string]bool{
		"WrapperComponent": true,
	})
	// require.NoError(t, err)

	fmt.Println(template.String())

	require.Fail(t, "omg")
}

func TestTemplateParse_Attributes(t *testing.T) {
	template := &template{Name: "test"}
	template.Parse(`<b>Hello <WrapperComponent rad name="Fox Mulder">Foo</WrapperComponent></b>`, map[string]bool{
		"WrapperComponent": true,
	})
	// require.NoError(t, err)

	fmt.Println(template.String())

	require.Fail(t, "omg")
}

func TestTemplateParse_Nested(t *testing.T) {
	tmpl := &template{Name: "test"}
	tmpl.Parse(`
		<b>
			Hello
			<WrapperComponent rad name="Fox Mulder" age="{{.Age}}">
				<NestedComponent>
				Foo
				</NestedComponent>
			</WrapperComponent></b>
	`, map[string]bool{
		"WrapperComponent": true,
		"NestedComponent":  true,
	})

	fmt.Println(tmpl.String())

	var parsed *htmltemplate.Template
	parsed, err := htmltemplate.New("test").Funcs(htmltemplate.FuncMap{
		"__goatRenderComponent": func(name string, identifier string, attributes string) string {
			var b bytes.Buffer
			err := parsed.ExecuteTemplate(&b, identifier, nil)
			if err != nil {
				panic(fmt.Errorf("error rendering component %s: %w", name, err))
			}

			return b.String()
		},
		"__goatDict": func(...any) string {
			return "wow"
		},
	}).Parse(tmpl.String())
	require.NoError(t, err)

	var b bytes.Buffer
	err = parsed.Execute(&b, nil)

	fmt.Println("output!", b.String())
	require.NoError(t, err)

	require.Fail(t, "omg")
}

func TestTemplateParse_AttributesWithGoAttributes(t *testing.T) {
	template := &template{Name: "test"}
	template.Parse(`<a href="{{ GenerateURL "sign up" }}">Sign up</a>`, map[string]bool{
		"WrapperComponent": true,
	})
	// require.NoError(t, err)

	fmt.Println(template.content)

	require.Fail(t, "omg")
}
