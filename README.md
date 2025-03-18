# mold

Mold builds on Go templates to provide a simple and familiar API for rendering web pages.

## Getting Started

### 1. Create a template file

```html
{{define "head"}}
<link rel="stylesheet" href="https://cdn.simplecss.org/simple.min.css">
{{end}}

<h1>Hello from a <a href="//github.com/abiosoft/mold">Mold</a> template</h1>
```

### 2. Render

```go
//go:embed index.html
var dir embed.FS

var layout = mold.New(dir)

func handle(w http.ResponseWriter, r *http.Request){
    layout.Render(w, "index.html", nil)
}
```

### Examples

Check the [examples](https://github.com/abiosoft/mold/tree/main/examples) directory for more.

## Other Features

### Layouts

Custom layout can be specified to override the [default](https://github.com/abiosoft/mold/blob/main/layout.html).

```html
<!DOCTYPE html>
<html>
<head>
    {{ template "head" .data }}
</head>
<body>
    {{ template "body" .data }}
</body>
</html>
```

Create instance with options.

```go
options := mold.Options{
	Layout: "path/to/layout.html",
}
layout, err := mold.NewWithOptions(options)
```

### Partials

Partials can be rendered within templates.

```html
{{ partial "path/to/partial.html" }}
```

## Why?

Go templates, while simple and powerful, can be unfamiliar. Mold provides a more intuitive and familiar higher-level usage, without reinventing the wheel.

## License

MIT
