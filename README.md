# GOAT

GOAT is an attempt to make Go templates more component focused using the constraints of the existing Go language and tooling.

## Usage

GOAT takes an approach similar to [ViewComponent](https://viewcomponent.org/), using sidecar templates to define components. In addition to coupling Go templates with structs, GOAT also allows enables a React style syntax for utilizing components in your templates.

```go
type GreetPage struct {
  Name string
}

// a Helper method we can use in our template
func (g GreetPage) YellName() string {
  return strings.ToUpper(g.Name)
}

// Then, to render the template:
engine := template.New(nil)
engine.RegisterComponent(GreetPage{}, `Hello, {{.YellName}}`)

var b strings.Builder
engine.Render(&b, &GreetPage{Name: "World"})
```

The `GreetPage` struct instance will be passed to the `greet_page.html` template as `data`, allowing you to access public fields and call methods on the struct.

### Composing components

Let's say we want to reuse our YellName functionality in another component, but also **bold** the name. We can create a new component and reference the component directly in our `greet_page.html` template as if it was another element:

```go
//goat:component yell.html
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
