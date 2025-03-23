package mold

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// defaults
var (
	//go:embed layout.html
	defaultLayout string

	// default filename extenstions for template files
	defaultExts = []string{"html", "gohtml", "tpl", "tmpl"}
)

const (
	// sections
	bodySection = "body"
	headSection = "head"
)

type (
	templateSet map[string]*template.Template
	moldEngine  templateSet
)

func newEngine(fsys fs.FS, options ...Option) (Engine, error) {
	c := Config{
		fs: fsys,
	}
	if err := setup(&c, options...); err != nil {
		return nil, fmt.Errorf("error creating new engine: %w", err)
	}

	m := moldEngine{}

	// traverse to fetch all templates and populate the root template.
	root, ts, err := walk(c.fs, c.exts.val, c.funcMap.val)
	if err != nil {
		return nil, fmt.Errorf("error creating new engine: %w", err)
	}

	// process layout
	layout, err := parseLayout(root, c.layoutFile, c.funcMap.val)
	if err != nil {
		return nil, fmt.Errorf("error parsing layout: %w", err)
	}

	// process views
	for _, t := range ts {
		// ignore layout file
		if t.name == c.layoutFile.name {
			continue
		}
		view, err := parseView(root, layout, t.name, t.body)
		if err != nil {
			return nil, err
		}
		m[t.name] = view
	}

	return m, nil
}

// Render implements Layout.
func (m moldEngine) Render(w io.Writer, view string, data any) error {
	layout, ok := m[view]
	if !ok {
		return ErrNotFound
	}

	if err := layout.Execute(w, data); err != nil {
		return fmt.Errorf("error rendering '%s': %w", view, err)
	}

	return nil
}

func walk(fsys fs.FS, exts []string, funcMap template.FuncMap) (root templateSet, ts []templateFile, err error) {
	root = templateSet{}
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// skip hidden files and directories.
		if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if !validExt(exts, ext) {
			return nil
		}

		f, err := readFile(fsys, path)
		if err != nil {
			return err
		}

		if t, err := template.New(path).Funcs(funcMap).Parse(f); err != nil {
			return fmt.Errorf("error parsing template '%s': %w", path, err)
		} else {
			root[path] = t
			ts = append(ts, templateFile{name: path, body: f})
		}

		return nil
	})

	return
}

func setup(c *Config, options ...Option) error {
	// apply options
	for _, opt := range options {
		opt(c)
	}

	// root
	if c.root.set {
		sub, err := fs.Sub(c.fs, c.root.val)
		if err != nil {
			return fmt.Errorf("error setting subdirectory '%s': %w", c.root.val, err)
		}
		c.fs = sub
	}

	// layout
	if c.layout.set {
		f, err := readFile(c.fs, c.layout.val)
		if err != nil {
			return fmt.Errorf("error reading layout file '%s': %w", c.layout.val, err)
		}
		c.layoutFile.body = f
		c.layoutFile.name = c.layout.val
	} else {
		c.layoutFile.body = defaultLayout
		c.layoutFile.name = "default_layout"
	}

	// extensions
	if !c.exts.set {
		c.exts.update(defaultExts)
	}

	// funcMap
	funcMap := placeholderFuncs()
	if c.funcMap.set {
		for k, f := range c.funcMap.val {
			funcMap[k] = f
		}
	}
	c.funcMap.update(funcMap)

	return nil
}

func parseLayout(root templateSet, t templateFile, funcMap template.FuncMap) (*template.Template, error) {
	layout, err := template.New("layout").Funcs(funcMap).Parse(t.body)
	if err != nil {
		return nil, err
	}

	// process template tree for layout
	refs, err := processTree(layout, t.body)
	if err != nil {
		return nil, fmt.Errorf("error processing layout: %w", err)
	}
	for _, ref := range refs {
		t := root[ref.name]
		if t == nil {
			if ref.typ == partialFunc {
				return nil, fmt.Errorf("error parsing template '%s': %w", ref.name, ErrNotFound)
			}
			t, _ = template.New(ref.name).Parse("")
		}
		layout.AddParseTree(ref.name, t.Tree)
	}

	return layout, nil
}

func parseView(root templateSet, layout *template.Template, name, raw string) (*template.Template, error) {
	view, err := layout.Clone()
	if err != nil {
		return nil, fmt.Errorf("error creating layout for view '%s': %w", name, err)
	}

	body := root[name]
	if body == nil {
		return nil, ErrNotFound
	}

	// process template tree for body
	refs, err := processTree(body, raw)
	if err != nil {
		return nil, fmt.Errorf("error parsing view '%s': %w", name, err)
	}
	for _, ref := range refs {
		t := root[ref.name]
		if t == nil {
			return nil, fmt.Errorf("error parsing template '%s': %w", ref.name, ErrNotFound)
		}
		view.AddParseTree(ref.name, t.Tree)
	}

	// add defined templates to the layout
	for _, t := range body.Templates() {
		tName := t.Name()
		if tName == name {
			tName = bodySection
		}
		view.AddParseTree(tName, t.Tree)
	}

	return view, nil
}

func readFile(fsys fs.FS, name string) (string, error) {
	f, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return string(f), nil
}

func validExt(exts []string, ext string) bool {
	if ext == "" {
		return false
	}

	sanitize := func(ext string) string {
		return strings.ToLower(strings.TrimPrefix(ext, "."))
	}

	for _, e := range exts {
		if sanitize(e) == sanitize(ext) {
			return true
		}
	}

	return false
}

func placeholderFuncs() template.FuncMap {
	return map[string]any{
		renderFunc.String():  func(...string) string { return "" },
		partialFunc.String(): func(string, ...any) string { return "" },
	}
}

type templateFile struct {
	name string
	body string
}

type optionVal[T any] struct {
	val T
	set bool
}

func newVal[T any](val T) optionVal[T] {
	return optionVal[T]{val: val, set: true}
}

func (o *optionVal[T]) update(val T) {
	o.val = val
}
