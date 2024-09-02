package template

import (
	"fmt"
	"testing"

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
	template := &template{Name: "test"}
	template.Parse(`
		<b>
			Hello
			<WrapperComponent rad name="Fox Mulder">
				<NestedComponent>
				Foo
				</NestedComponent>
			</WrapperComponent></b>
	`, map[string]bool{
		"WrapperComponent": true,
		"NestedComponent":  true,
	})

	fmt.Println(template.String())

	require.Fail(t, "omg")
}
