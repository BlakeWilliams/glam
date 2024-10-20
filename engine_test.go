package glam

import (
	"bytes"
	"html/template"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// WrapperComponent is a test component that renders a div with a name and age
type WrapperComponent struct {
	Name     string `attr:"name"`
	Age      int
	Children template.HTML
}

var wrapperTemplate = `<div>
	Name: {{.Name}}
	Age: {{.Age}}
	{{.Children}}
</div>
`

type NestedComponent struct {
	Children template.HTML
}

var nestedTemplate = `<article>
	{{.Children}}
</article>
`

// TODO: raise when a component is registered but is lowercased

type HelloNestedComponent struct {
	// TODO: Make this an int64 and handle casting
	Age int
}

func TestRenderNestedTemplate(t *testing.T) {
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
	err = engine.RegisterComponent(
		HelloNestedComponent{},
		`<b>
			Hello
			<WrapperComponent rad name="Fox Mulder" Age="{{.Age}}">
				<NestedComponent>
				Foo
				</NestedComponent>
			</WrapperComponent></b>
	`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.Render(&b, HelloNestedComponent{Age: 32})
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`<b>\s+Hello`), b.String())
	require.Contains(t, b.String(), "Name: Fox Mulder")
	require.Contains(t, b.String(), "Age: 32")
	require.Regexp(t, regexp.MustCompile(`<article>\s+Foo`), b.String())
	require.Regexp(t, regexp.MustCompile(`</b>`), b.String())
}

type GreetingPage struct {
	Name string
}

var greetingTemplate = `<b>
	Hello
	<WrapperComponent rad name="{{.Name}}" Age="{{32}}">
		<NestedComponent>
		Foo
		</NestedComponent>
	</WrapperComponent>
</b>`

func TestTemplateParse_Nested_ReverseRegister(t *testing.T) {
	engine := New(nil)

	err := engine.RegisterComponent(&GreetingPage{}, greetingTemplate)
	require.NoError(t, err)
	err = engine.RegisterComponent(&WrapperComponent{}, wrapperTemplate)
	require.NoError(t, err)
	err = engine.RegisterComponent(&NestedComponent{}, nestedTemplate)
	require.NoError(t, err)

	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.Render(&b, &GreetingPage{Name: "Fox Mulder"})
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`<b>\s+Hello`), b.String())
	require.Contains(t, b.String(), "Name: Fox Mulder")
	require.Contains(t, b.String(), "Age: 32")
	require.Regexp(t, regexp.MustCompile(`<article>\s+Foo`), b.String())
	require.Regexp(t, regexp.MustCompile(`</b>`), b.String())
}

type TestFSComponent struct {
	Value string
}

func TestEngineRegisterComponentFS(t *testing.T) {
	engine := New(nil)

	err := engine.RegisterComponentFS(&TestFSComponent{}, "internal/template/test.glam.html")
	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.Render(&b, &TestFSComponent{Value: "world!"})
	require.NoError(t, err)

	require.Contains(t, b.String(), "Testing, world!")
}

type FormComponent struct{}

func TestRenderWithFuncs(t *testing.T) {
	engine := New(FuncMap{
		"CSRF": func() string {
			panic("must be overridden")
		},
	})

	err := engine.RegisterComponent(&TestFSComponent{}, `<input type="hidden" value="{{ CSRF }}">`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.RenderWithFuncs(&b, &TestFSComponent{Value: "world!"}, FuncMap{
		"CSRF": func() string {
			// pretend this is csrf
			return "abc123"
		},
	})
	require.NoError(t, err)

	require.Equal(t, `<input type="hidden" value="abc123">`, b.String())
}

type privateComponent struct{}
type PublicComponent struct{}
type Title struct{}

func TestRegistrationFailures(t *testing.T) {
	testCases := []struct {
		desc        string
		component   any
		errorString string
	}{
		{
			desc:        "lowercase component names return an error",
			component:   privateComponent{},
			errorString: "registered components must be public",
		},
		{
			desc:        "components that collide with HTML tags return an error",
			component:   Title{},
			errorString: "component Title conflicts with an existing HTML tag",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			engine := New(nil)
			err := engine.RegisterComponent(tC.component, "<h1>Hi</h1>")

			if tC.errorString == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tC.errorString)
			}
		})
	}
}
