package mold

import (
	"bytes"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
)

type testFile struct {
	name string
	data string
}

func createTestFS(extraFiles ...testFile) fs.FS {
	mapFS := fstest.MapFS{
		"layout.html":   &fstest.MapFile{Data: []byte(`<html><body>{{render}}<br>{{partial "partial2.html" .Age}}</body></html>`)},
		"view.html":     &fstest.MapFile{Data: []byte(`Hello, {{.Name}}!<br>{{partial "partial.html" .Location}}`)},
		"partial.html":  &fstest.MapFile{Data: []byte(`Location: {{.}}`)},
		"partial2.html": &fstest.MapFile{Data: []byte(`Age: {{.}}`)},
		"skip.txt":      &fstest.MapFile{},
		".hidden_file":  &fstest.MapFile{},
		".hidden/.file": &fstest.MapFile{},
	}
	for _, f := range extraFiles {
		mapFS[f.name] = &fstest.MapFile{Data: []byte(f.data)}
	}
	return mapFS
}

func TestNew(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("Must() expected no panic, got panic: %v", err)
		}
	}()

	testFS := createTestFS()
	Must(New(testFS))
}

func TestNew_Sub(t *testing.T) {
	testFS := createTestFS(testFile{"web/view.html", `Hello`})
	Must(New(testFS, WithRoot("web")))
}

func TestNew_InvalidSub(t *testing.T) {
	testFS := createTestFS(testFile{"web/view.html", `Hello`})
	if _, err := New(testFS, WithRoot("invalid")); err == nil {
		t.Errorf("New() expected error, got nil")
	}
}

func TestNew_Panic(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("Must() expected panic, got nil")
		}
	}()

	testFS := createTestFS(testFile{"parse.html", "{{partial}}"})
	Must(New(testFS))
}

func TestNew_LayoutNotFound(t *testing.T) {
	testFS := createTestFS()

	if _, err := New(testFS, WithLayout("missing.html")); err == nil {
		t.Errorf("New() expected error, got nil")
	}
}

func TestNew_LayoutParseError(t *testing.T) {
	testFS := createTestFS(testFile{"layout.html", "{{partial}}"})

	if _, err := New(testFS, WithLayout("layout.html")); err == nil {
		t.Errorf("New() expected error, got nil")
	}
}

func TestNew_LayoutInvalidExt(t *testing.T) {
	testFS := createTestFS(testFile{"layout.txt", "{{partial}}"})

	if _, err := New(testFS, WithLayout("layout.txt")); err == nil {
		t.Errorf("New() expected error, got nil")
	}
}

func TestNew_ViewParseError(t *testing.T) {
	testFS := createTestFS(testFile{"parse.html", "{{partial}}"})

	if _, err := New(testFS); err == nil {
		t.Errorf("New() expected error, got nil")
	}
}

func TestNew_CyclicReferenceError(t *testing.T) {
	testFS := createTestFS(testFile{"parse.html", `{{partial "parse.html"}}`})

	if _, err := New(testFS); err == nil {
		t.Errorf("New() expected error, got nil %v", err)
	}
}

func TestNew_Ext(t *testing.T) {
	testFS := createTestFS(
		testFile{"layout.mine", "{{render}}"},
		testFile{"view.mine", "Hello {{.Name}}"},
	)

	option := With(
		WithExt(".mine"),
		WithLayout("layout.mine"),
	)
	engine := Must(New(testFS, option))

	data := map[string]any{
		"Name":     "John Doe",
		"Location": "Mars",
		"Age":      40,
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "view.mine", data); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "Hello John Doe"
	if buf.String() != expected {
		t.Errorf("Render() got = %q, want %q", buf.String(), expected)
	}
}

func TestNew_FuncMap(t *testing.T) {
	testFS := createTestFS(
		testFile{"layout.html", "{{render}}"},
		testFile{"view.html", "Hello {{reverse .Name}}"},
	)

	funcMap := map[string]any{
		"reverse": func(s string) (out string) {
			for _, r := range s {
				out = fmt.Sprintf("%c%s", r, out)
			}
			return
		},
	}
	option := With(
		WithLayout("layout.html"),
		WithFuncMap(funcMap),
	)
	engine := Must(New(testFS, option))

	data := map[string]any{
		"Name": "John Doe",
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "view.html", data); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "Hello eoD nhoJ"
	if buf.String() != expected {
		t.Errorf("Render() got = %q, want %q", buf.String(), expected)
	}
}

func TestRender(t *testing.T) {
	testFS := createTestFS()

	engine := Must(New(testFS, WithLayout("layout.html")))

	data := map[string]any{
		"Name":     "John Doe",
		"Location": "Mars",
		"Age":      40,
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "view.html", data); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "<html><body>Hello, John Doe!<br>Location: Mars<br>Age: 40</body></html>"
	if buf.String() != expected {
		t.Errorf("Render() got = %q, want %q", buf.String(), expected)
	}

}

func TestRender_ViewNotFound(t *testing.T) {
	testFS := createTestFS()

	engine, err := New(testFS)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "nonexistent.html", nil); err == nil {
		t.Errorf("Render() expected error, got nil")
	}
}

func TestRender_LayoutPartialNotFound(t *testing.T) {
	testFS := createTestFS(testFile{"layout.html", `{{partial "invalid.html"}}`})

	if _, err := New(testFS, WithLayout("layout.html")); err == nil {
		t.Errorf("New() expected error,  got nil")
	}
}

func TestRender_ViewPartialNotFound(t *testing.T) {
	testFS := createTestFS(testFile{"view.html", `{{partial "invalid.html"}}`})

	if _, err := New(testFS); err == nil {
		t.Errorf("New() expected error,  got nil")
	}
}

func TestRender_InvalidData(t *testing.T) {
	testFS := createTestFS(testFile{"index.html", `{{.Missing}}`})

	engine := Must(New(testFS))

	var buf bytes.Buffer
	if err := engine.Render(&buf, "index.html", "something"); err == nil {
		t.Errorf("Render() expected error, got nil")
	}
}

func TestRender_ViewInvalidRender(t *testing.T) {
	testFS := createTestFS(
		testFile{"view.html", `{{render}}`},
	)

	if _, err := New(testFS); err == nil {
		t.Errorf("New() expected error,  got nil")
	}
}

func TestRender_PartialInvalidRender(t *testing.T) {
	testFS := createTestFS(
		testFile{"view.html", `{{partial "partial.html"}}`},
		testFile{"view_partial.html", `{{render}}`},
	)

	if _, err := New(testFS); err == nil {
		t.Errorf("New() expected error,  got nil")
	}
}

func TestRender_PartialInvalidPartial(t *testing.T) {
	testFS := createTestFS(
		testFile{"view.html", `{{partial "partial.html"}}`},
		testFile{"partial.html", `{{partial "view.html"}}`},
	)

	if _, err := New(testFS); err == nil {
		t.Errorf("New() expected error, got nil")
	}
}

func TestRender_TemplateIf(t *testing.T) {
	testFS := createTestFS(testFile{"index.html", `{{if .Name}}{{.Name}}{{end}}`})

	engine := Must(New(testFS))

	var buf bytes.Buffer
	if err := engine.Render(&buf, "index.html", map[string]any{"Name": "John Doe"}); err != nil {
		t.Errorf("Render() expected nil, got %v", err)
	}
}

func TestRender_TemplateWith(t *testing.T) {
	testFS := createTestFS(testFile{"index.html", `{{with .Name}}{{.}}{{end}}`})

	engine := Must(New(testFS))

	var buf bytes.Buffer
	if err := engine.Render(&buf, "index.html", map[string]any{"Name": "John Doe"}); err != nil {
		t.Errorf("Render() expected nil, got %v", err)
	}
}

func TestRender_TemplateRange(t *testing.T) {
	testFS := createTestFS(testFile{"index.html", `{{range .}}{{.}}{{end}}`})

	engine := Must(New(testFS))

	var buf bytes.Buffer
	if err := engine.Render(&buf, "index.html", []string{"John Doe"}); err != nil {
		t.Errorf("Render() expected nil, got %v", err)
	}
}

func TestHideFS_Hidden(t *testing.T) {
	testFS := createTestFS()
	hideFS := HideFS(testFS)

	if _, err := hideFS.Open("view.html"); err == nil {
		t.Errorf("Open() expected error, got nil")
	}
}

func TestHideFS_Allowed(t *testing.T) {
	testFS := createTestFS()
	hideFS := HideFS(testFS, ".tpl")

	if _, err := hideFS.Open("view.html"); err != nil {
		t.Errorf("Open() expected nil, got %v", err)
	}
}
