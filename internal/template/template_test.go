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

	_, _ = w.Write([]byte("<-- placeholder for " + t.String() + " -->"))

	return nil
}

func (r *FakeRenderer) FuncMap() htmltemplate.FuncMap {
	return r.funcMap
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
	err = tmpl.Execute(&b, nil)
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
