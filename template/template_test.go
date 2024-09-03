package template

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// WrapperComponent is a test component that renders a div with a name and age
type WrapperComponent struct {
	Name     string
	Age      int
	Children template.HTML
}

var wrapperTemplate = `<div>
	Name: {{.Name}}
	Age: {{.Age}}
	{{.Children}}
</div>
`

// func (wc *WrapperComponent) Render(w io.Writer) {
// 	w.Write([]byte(fmt.Sprintf("<div>Name: %s\nAge: %d\n%s</div>", wc.Name, wc.Age, wc.Children)))
// }

type NestedComponent struct {
	Children template.HTML
}

// func (nc *NestedComponent) Render(w io.Writer) {
// 	w.Write([]byte(fmt.Sprintf("<article>%s</article>", nc.Children)))
// }

var nestedTemplate = `<article>
	{{.Children}}
</article>
`

// func TestTemplateParse(t *testing.T) {
// 	parser := &TemplateParser{
// 		components: map[string]bool{
// 			"Greeter": true,
// 		},
// 	}

// 	// err := parser.ParseTemplate(strings.NewReader(`<b>Hello <Yell name="{.Name}"></Yell></b>`))
// 	err := parser.ParseTemplate("main", `
// 	{{ range $index, $element := .Value }}
// 	 	<b>Hi {{.Foo}}</b>
// 	{{ end }}
// 	`)
// 	require.NoError(t, err)

// 	require.Fail(t, "omg")
// }

// func TestTemplateParse_WithContent(t *testing.T) {
// 	parser := &Engine{
// 		components: map[string]reflect.Value{
// 			"WrapperComponent": reflect.ValueOf(WrapperComponent{}),
// 		},
// 	}

// 	_, err := parser.ParseTemplate("main.go", `<b>Hello <WrapperComponent>Foo</WrapperComponent></b>`)
// 	require.NoError(t, err)

// 	require.Fail(t, "omg")
// }

// func TestTemplateParse_omg(t *testing.T) {
// 	template := &Template{Name: "test"}
// 	template.Parse(`<b>Hello <WrapperComponent>Foo</WrapperComponent></b>`, map[string]reflect.Value{
// 		"WrapperComponent": reflect.ValueOf(WrapperComponent{}),
// 	})
// 	// require.NoError(t, err)

// 	fmt.Println(template.String())

// 	require.Fail(t, "omg")
// }

// func TestTemplateParse_Attributes(t *testing.T) {
// 	template := &Template{Name: "test"}
// 	template.Parse(`<b>Hello <WrapperComponent rad name="Fox Mulder">Foo</WrapperComponent></b>`, map[string]reflect.Value{
// 		"WrapperComponent": reflect.ValueOf(WrapperComponent{}),
// 	})
// 	// require.NoError(t, err)

// 	fmt.Println(template.String())

// 	require.Fail(t, "omg")
// }

func TestTemplateParse_Nested(t *testing.T) {
	engine := New(nil)
	err := engine.RegisterComponent(
		&WrapperComponent{},
		wrapperTemplate,
	)
	require.NoError(t, err)
	err = engine.RegisterComponent(
		&NestedComponent{},
		nestedTemplate,
	)
	require.NoError(t, err)
	fmt.Println(engine)

	tmpl, err := engine.parseTemplate("main.goat.html", `
		<b>
			Hello
			<WrapperComponent rad Name="Fox Mulder" Age="{{.Age}}">
				<NestedComponent>
				Foo
				</NestedComponent>
			</WrapperComponent></b>
	`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = tmpl.htmltemplate.Execute(&b, map[string]any{"Age": 32})
	require.NoError(t, err)
	fmt.Println("output!", b.String())
	require.Regexp(t, regexp.MustCompile(`<b>\s+Hello`), b.String())
	require.Contains(t, b.String(), "Name: Fox Mulder")
	require.Contains(t, b.String(), "Age: 32")
	require.Regexp(t, regexp.MustCompile(`<article>\s+Foo`), b.String())
	require.Regexp(t, regexp.MustCompile(`</b>`), b.String())
}

func TestTemplateParse_AttributesWithGoAttributes(t *testing.T) {
	engine := New(nil)
	engine.funcs["GenerateURL"] = func(name string) string {
		return "http://localhost:3000/sign-up"
	}

	tmpl, err := engine.parseTemplate("main.goat.html", `<a href="{{ GenerateURL "sign up" }}">Sign up</a>`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = tmpl.htmltemplate.Execute(&b, nil)
	require.NoError(t, err)

	require.Regexp(t, regexp.MustCompile(`<a href="http://localhost:3000/sign-up">Sign up</a>`), b.String())
}
