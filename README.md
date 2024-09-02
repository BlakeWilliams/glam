# GOAT

GOAT is an attempt to make Go templates more component focused using the constraints of the existing Go language and tooling.

## Usage

GOAT takes an approach similar to [ViewComponent](https://viewcomponent.org/), using sidecar templates to define components. It differs though, in that it enhances `html/template` templates by automatically replacing component tags with their corresponding templates. Here's a small example:

```go
// GOAT uses comments to define components which will then be generated into an
// implementation file.

//goat:greet_page.html
type GreetPage struct {
  Name string
}

// a Helper method we can use in our template
func (g GreetPage) YellName() string {
  return strings.ToUpper(g.Name)
}
```

The `GreetPage` struct instance will be passed to the `greet_page.html` template as `data`, allowing you to access public fields and call methods on the struct.

```html
<!-- greet_page.html -->
<h1>Hello, {{.Name}}!</h1>
```

Let's say we want to reuse our YellName functionality in another component, but also **bold** the name. We can create a new component and reference the component directly in our `greet_page.html` template as if it was another element:

```go
//goat:yell.html
type Yell struct {
  Name string
}

func (y Yell) YellName() string {
  return strings.ToUpper(y.Name)
}
```

```html
<!-- yell.html -->
<b>{{.YellName}}</b>
```

```html
<!-- greet_page.html -->
<h1>Hello, <Yell name="{{.Name}}" /></h1>
```

The HTML is parsed and the `Yell` HTML tag is replaced with a call to render our Yell component.
