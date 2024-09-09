# Glam

Glam is an attempt to make Go templates more component focused using the constraints of the existing Go language and tooling.

With glam, you can write templates like:

```html
<!-- Initializes and renders a `type FormComponent struct` -->
<FormComponent action="{{ .URL }}">
  <!-- Initializes and renders the `type TextField struct` -->
  <TextField placeholder="username" prefix="@" value="{{.Username}}" />
  <TextField placeholder="email" value="{{.Email}}" />
  <TextField placeholder="password" type="password" />

  <button type="submit" class="btn">Sign Up</button>
</FormComponent>
```

With structs backing the logic and templates:

```go
type FormComponent struct {
	Action   string        `attr:"action"`
	Children template.HTML
	Class    string        `attr:"class"`
}
```

```html
<!-- FormComponent template -->
<form action="{{.Action}}" class="default classes {{.Class}}">
  {{.Children}}
</form>
```

## Usage

Glam takes an approach similar to [ViewComponent](https://viewcomponent.org/), using sidecar templates to define components. In addition to coupling Go templates with structs, glam also allows enables a React style syntax for utilizing components in your templates.

```go
import "github.com/blakewilliams/glam"

type GreetPage struct {
	Name string
}

// a Helper method we can use in our template
func (g GreetPage) YellName() string {
	return strings.ToUpper(g.Name)
}

// Then, to render the template:
engine := glam.New(nil)
engine.RegisterComponent(GreetPage{}, `Hello, {{.YellName}}`)

var b strings.Builder
engine.Render(&b, &GreetPage{Name: "World"})
```

The `GreetPage` struct instance will be passed to the `greet_page.html` template as `data`, allowing you to access public fields and call methods on the struct.

### Composing components

Let's say we want to reuse our YellName functionality in another component, but also **bold** the name. We can create a new component and reference the component directly in our `greet_page.html` template as if it was another element:

```go
type Yell struct {
	Name string
}

func (y Yell) YellName() string {
	return strings.ToUpper(y.Name)
}

// Lets update our GreetPage component to use our new Yell component
engine.RegisterComponent(GreetPage{}, `Hello, <Yell Name={{.Name}}></Yell>`)
// Let's also register our new Yell component
engine.RegisterComponent(Yell{}, `<b>{{.YellName}}</b>`)
```

The HTML is parsed and the `Yell` HTML tag is replaced with a call to render our Yell component.

### Child content

Since components can be used like HTML tags, that means they can have child content too. The current approach is relatively basic since it always expects a `template.HTML` value, but you can accept and render child content using the conventional `Children` struct field:

```go
type WrapperComponent struct {
	Children template.HTML
}
```

```html
<WrapperComponent>Hello</WrapperComponent>
```

When the template above is executed, `WrapperComponent` will have `Children` populated with the HTML safe string `Hello`.

### Graceful degradation

Components can implement the `Recoverable` interface to rescue against `panic`s and render fallback content. For example:

```go
type SafeSidebar struct {
	Children template.HTML
}

func (mc *SafeSidebar) Recover(w io.Writer, err any) {
	w.Write(`<b>Failed to load sidebar</b>`)
}
```

If any `panic` occurs when rendering `SafeSidebar` or child content (via `<SafeSidebar>foo bar</SafeSidebar>`) it will render the fallback content written.
