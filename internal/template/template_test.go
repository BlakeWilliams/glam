package template

import (
	"bytes"
	htmltemplate "html/template"
	"io"
	"reflect"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

type FakeRenderer struct {
	knownComponents map[string]reflect.Type
	funcMap         htmltemplate.FuncMap
}

var _ Renderer = (*FakeRenderer)(nil)

func (r *FakeRenderer) KnownComponents() map[string]reflect.Type {
	return r.knownComponents
}

func (r *FakeRenderer) Render(w io.Writer, v any) error {
	t := reflect.ValueOf(v)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	_, _ = w.Write([]byte("<!-- placeholder for " + t.Type().Name() + " -->"))

	return nil
}

func (r *FakeRenderer) FuncMap() htmltemplate.FuncMap {
	return r.funcMap
}

func NewFakeRenderer() *FakeRenderer {
	return &FakeRenderer{
		knownComponents: make(map[string]reflect.Type, 0),
		funcMap: htmltemplate.FuncMap{
			"__glamDict": func(pairs ...any) map[string]any {
				return make(map[string]any)
			},
		},
	}
}

func TestStandardGoTemplate(t *testing.T) {
	renderer := &FakeRenderer{
		knownComponents: make(map[string]reflect.Type),
		funcMap: htmltemplate.FuncMap{
			"GenerateURL": func(name string) string {
				return "http://localhost:3000/sign-up"
			},
		},
	}
	tmpl, err := New("main.glam.html", renderer, `<a href="{{ GenerateURL "sign up" }}">Sign up</a>`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = tmpl.Execute(&b, nil, nil)
	require.NoError(t, err)

	require.Regexp(t, regexp.MustCompile(`<a href="http://localhost:3000/sign-up">Sign up</a>`), b.String())
}

func TestWipingRawContent(t *testing.T) {
	testCases := []struct {
		desc                  string
		template              string
		expectRawContentWiped bool
	}{
		{
			desc:                  "raw content remains when there are potentially referenced components",
			template:              `<Foo></Foo>`,
			expectRawContentWiped: false,
		},
		{
			desc:                  "raw content is cleared when there are no potentially referenced components",
			template:              `<b>hello</b>`,
			expectRawContentWiped: true,
		},
		{
			desc:                  "raw content is cleared potentially referenced components are likely HTML tags",
			template:              `<B>hello</B>`,
			expectRawContentWiped: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			renderer := &FakeRenderer{knownComponents: make(map[string]reflect.Type)}
			tmpl, err := New("testing", renderer, tC.template)
			require.NoError(t, err)

			if tC.expectRawContentWiped {
				require.Empty(t, tmpl.rawContent)
			} else {
				require.Equal(t, tC.template, tmpl.rawContent)
			}
		})
	}
}

type EmptyComponent struct{}

func TestSelfClosingTemplate(t *testing.T) {
	renderer := &FakeRenderer{knownComponents: make(map[string]reflect.Type)}
	renderer.knownComponents["Test"] = reflect.TypeOf(&EmptyComponent{})

	tmpl, err := New("testing", renderer, `hello <Test/>!`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = tmpl.Execute(&b, nil, nil)
	require.NoError(t, err)

	require.Contains(t, b.String(), `hello <!-- placeholder for EmptyComponent -->`)
}

func TestSelfClosingNestedTags(t *testing.T) {
	renderer := NewFakeRenderer()
	renderer.knownComponents["Test"] = reflect.TypeOf(&EmptyComponent{})

	tmpl, err := New("testing", renderer, `hello <Test><img foo="bar"/>Hello</Test>!`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = tmpl.Execute(&b, nil, nil)
	require.NoError(t, err)

	require.Contains(t, b.String(), `hello <!-- placeholder for EmptyComponent -->`)
}

type RescuableComponent struct {
	ShouldPanic       bool
	ShouldRenderHello bool
}

func (r *RescuableComponent) Recover(w io.Writer, err any) {
	_, _ = w.Write([]byte("oh no!"))
}

func TestRescue(t *testing.T) {
	renderer := &FakeRenderer{
		knownComponents: make(map[string]reflect.Type),
		funcMap: htmltemplate.FuncMap{
			"PanicOhNo": func() string {
				panic("oh no!")
			},
		},
	}
	tmpl, err := New("main.glam.html", renderer, `Hello world! {{PanicOhNo}}`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = tmpl.Execute(&b, &RescuableComponent{
		ShouldRenderHello: true,
		ShouldPanic:       true,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "oh no!", b.String())
}

func TestTextOnlyTemplate(t *testing.T) {
	renderer := &FakeRenderer{
		knownComponents: make(map[string]reflect.Type),
		funcMap: htmltemplate.FuncMap{
			"PanicOhNo": func() string {
				panic("oh no!")
			},
		},
	}
	tmpl, err := New("main.glam.html", renderer, `Hello world!`)
	require.NoError(t, err)

	var b bytes.Buffer
	err = tmpl.Execute(&b, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "Hello world!", b.String())
}

// There was an infinite loop while parsing this template. Lets fix it
func TestLoneLeftCurly(t *testing.T) {
	renderer := &FakeRenderer{}
	_, err := New("main.glam.html", renderer, `<h1 foo="{oops}">Hi</h1>`)
	require.NoError(t, err)
}
