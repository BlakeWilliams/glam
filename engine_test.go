package glam

import (
	"bytes"
	"html/template"
	"io/fs"
	"os"
	"regexp"
	"strings"
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

type MapComponent struct {
	Map map[string]string
}

func TestRenderMapTemplate(t *testing.T) {
	engine := New(nil)
	err := engine.RegisterComponent(
		&WrapperComponent{},
		wrapperTemplate,
	)
	require.NoError(t, err)
	err = engine.RegisterComponent(
		MapComponent{},
		`<b>
			Hello
			<WrapperComponent name="{{index .Map "Fox"}}">
			</WrapperComponent>
		</b>
	`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.Render(&b, MapComponent{Map: map[string]string{"Fox": "Fox Mulder"}})
	require.NoError(t, err)
	require.Contains(t, b.String(), "Name: Fox Mulder")
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
	templateFS := os.DirFS("internal/template")

	err := engine.RegisterComponentFS(&TestFSComponent{}, templateFS.(fs.ReadFileFS), "test.glam.html")
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

func TestComponentNoChildrenPassed(t *testing.T) {
	engine := New(FuncMap{
		"trim":    strings.TrimSpace,
		"toUpper": strings.ToUpper,
	})

	type ButtonComponent struct {
		Children template.HTML
	}
	type RootComponent struct{}
	err := engine.RegisterComponent(&ButtonComponent{}, `<button>{{.Children}}</button>`)
	require.NoError(t, err)
	err = engine.RegisterComponent(&RootComponent{}, `<ButtonComponent></ButtonComponent>`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.Render(&b, &RootComponent{})
	require.NoError(t, err)

	require.Equal(t, `<button></button>`, b.String())
}

func TestAttributePipeline(t *testing.T) {
	engine := New(FuncMap{
		"trim":    strings.TrimSpace,
		"toUpper": strings.ToUpper,
	})

	type ButtonComponent struct {
		Children template.HTML
	}
	type LoopComponent struct {
		Names []string
	}
	err := engine.RegisterComponent(&ButtonComponent{}, `<button>{{.Children}}</button>`)
	require.NoError(t, err)
	err = engine.RegisterComponent(&LoopComponent{}, `{{range $_, $name := .Names}}<ButtonComponent>{{$name | trim | toUpper}}</ButtonComponent>{{end}}`)

	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.RenderWithFuncs(&b, &LoopComponent{Names: []string{" Fox", "Dana", "Skinner"}}, FuncMap{})
	require.NoError(t, err)

	require.Equal(t, `<button>FOX</button><button>DANA</button><button>SKINNER</button>`, b.String())
}

func TestRenderLoop(t *testing.T) {
	engine := New(FuncMap{})

	type ButtonComponent struct {
		Children template.HTML
	}
	type LoopComponent struct {
		Names []string
	}
	err := engine.RegisterComponent(&ButtonComponent{}, `<button>{{.Children}}</button>`)
	require.NoError(t, err)
	err = engine.RegisterComponent(&LoopComponent{}, `{{range $_, $name := .Names}}<ButtonComponent>{{$name}} {{"$name"}} </ButtonComponent>{{end}}`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.RenderWithFuncs(&b, &LoopComponent{Names: []string{"Fox", "Dana", "Skinner"}}, FuncMap{})
	require.NoError(t, err)

	require.Equal(t, `<button>Fox $name </button><button>Dana $name </button><button>Skinner $name </button>`, b.String())
}

func TestNestedRenderLoop(t *testing.T) {
	engine := New(FuncMap{})

	type ButtonComponent struct {
		Children template.HTML
		DataName string `attr:"data-name"`
	}
	type LoopComponent struct {
		Names []string
	}
	err := engine.RegisterComponent(&ButtonComponent{}, `<button data-name="{{.DataName}}">{{.Children}}</button>`)
	require.NoError(t, err)
	err = engine.RegisterComponent(&LoopComponent{}, `
		{{range $_, $name := .Names}}
		<ButtonComponent data-name="{{$name}}">
			{{$name}}
			<ButtonComponent data-name="{{$name}}">
				{{$name}}
			</ButtonComponent>
		</ButtonComponent>
		{{end}}
	`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = engine.RenderWithFuncs(&b, &LoopComponent{Names: []string{"Fox", "Dana", "Skinner"}}, FuncMap{})
	require.NoError(t, err)

	require.Equal(t, `
		
		<button data-name="Fox">
			Fox
		<button data-name="Fox">
				Fox
			</button>
		</button>
		
		<button data-name="Dana">
			Dana
		<button data-name="Dana">
				Dana
			</button>
		</button>
		
		<button data-name="Skinner">
			Skinner
		<button data-name="Skinner">
				Skinner
			</button>
		</button>
		
	`, b.String())
}
