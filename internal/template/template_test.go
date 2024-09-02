package template

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type WrapperComponent struct {
	Name     string
	Age      int
	Children template.HTML
}

func (wc *WrapperComponent) Render(w io.Writer) {
	w.Write([]byte(fmt.Sprintf("<div>Name:%s\nAge: %d\n%s</div>", wc.Name, wc.Age, wc.Children)))
}

type NestedComponent struct {
	Children template.HTML
}

func (nc *NestedComponent) Render(w io.Writer) {
	w.Write([]byte(fmt.Sprintf("<article>%s</article>", nc.Children)))
}

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
	engine := NewEngine()
	err := engine.RegisterComponent("WrapperComponent", &WrapperComponent{})
	require.NoError(t, err)
	err = engine.RegisterComponent("NestedComponent", &NestedComponent{})
	require.NoError(t, err)

	tmpl, err := engine.ParseTemplate("main.goat.html", `
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
	require.Fail(t, "omg")
}

func TestTemplateParse_AttributesWithGoAttributes(t *testing.T) {
	template := &Template{Name: "test"}
	template.Parse(`<a href="{{ GenerateURL "sign up" }}">Sign up</a>`, map[string]reflect.Type{})

	fmt.Println(template.content)

	require.Fail(t, "omg")
}
