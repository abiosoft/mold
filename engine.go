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
	defaultExts = []string{".html", ".gohtml", ".tpl", ".tmpl"}
)

type (
	templateSet map[string]*templateFile
	moldEngine  map[string]*template.Template
)

func newEngine(fsys fs.FS, options ...Option) (Engine, error) {
	c := Config{
		fs: fsys,
	}
	if err := setup(&c, options...); err != nil {
		return nil, fmt.Errorf("error creating new engine: %w", err)
	}

	m := moldEngine{}

	// traverse to fetch all templates
	set, err := walk(c.fs, c.exts.val, c.funcMap.val)
	if err != nil {
		return nil, fmt.Errorf("error creating new engine: %w", err)
	}

	// process layout
	layout, err := parseLayout(set, c.layoutRaw, c.funcMap.val)
	if err != nil {
		return nil, fmt.Errorf("error parsing layout: %w", err)
	}

	// process views
	for name := range set {
		view, err := parseView(set, layout, name)
		if err != nil {
			return nil, err
		}
		m[name] = view
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

func walk(fsys fs.FS, exts []string, funcMap template.FuncMap) (set templateSet, err error) {
	set = templateSet{}
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
		if !hasExt(exts, ext) {
			return nil
		}

		// skip layout files
		if err := validateLayoutFile(exts, path); err == nil {
			return nil
		}

		f, err := readFile(fsys, path)
		if err != nil {
			return err
		}

		if t, err := template.New(path).Funcs(funcMap).Parse(f); err != nil {
			return fmt.Errorf("error parsing template '%s': %w", path, err)
		} else {
			set[path] = &templateFile{Template: t, body: f}
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

	// extensions
	if !c.exts.set {
		c.exts.update(defaultExts)
	}

	// layout
	if c.layout.set {
		if err := validateLayoutFile(c.exts.val, c.layout.val); err != nil {
			return fmt.Errorf("invalid layout file: %w", err)
		}
		f, err := readFile(c.fs, c.layout.val)
		if err != nil {
			return fmt.Errorf("error reading layout file '%s': %w", c.layout.val, err)
		}
		c.layoutRaw = f
	} else {
		c.layout.update("default_layout")
		c.layoutRaw = defaultLayout
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

func parseLayout(root templateSet, layoutRaw string, funcMap template.FuncMap) (*templateFile, error) {
	t, err := template.New("layout").Funcs(funcMap).Parse(layoutRaw)
	if err != nil {
		return nil, err
	}

	layout := &templateFile{
		Template: t,
		typ:      layoutType,
		body:     layoutRaw,
	}

	// process template tree for layout
	refs, err := processTree(layout)
	if err != nil {
		return nil, fmt.Errorf("error processing layout: %w", err)
	}
	for _, ref := range refs {
		t := root[ref.name]
		if t == nil {
			if ref.typ == partialFunc {
				return nil, fmt.Errorf("error parsing template '%s': %w", ref.name, ErrNotFound)
			}
			tpl, _ := template.New(ref.name).Parse("") // safe to ignore the err
			t = &templateFile{Template: tpl}
		}

		t.typ = partialType
		if err := parsePartial(t); err != nil {
			return nil, fmt.Errorf("error parsing partial: '%s': %w", ref.name, err)
		}

		layout.AddParseTree(ref.name, t.Tree)
	}

	return layout, nil
}

func parseView(set templateSet, layout *templateFile, name string) (*template.Template, error) {
	view := template.Must(layout.Clone()) // safe

	body := set[name]
	body.typ = viewType

	// process template tree for body
	refs, err := processTree(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing view '%s': %w", name, err)
	}
	for _, ref := range refs {
		t := set[ref.name]
		if t == nil {
			return nil, fmt.Errorf("error parsing template '%s': %w", ref.name, ErrNotFound)
		}

		t.typ = partialType
		if err := parsePartial(t); err != nil {
			return nil, fmt.Errorf("error parsing partial: '%s': %w", ref.name, err)
		}

		view.AddParseTree(ref.name, t.Tree)
	}

	// add defined templates to the layout
	for _, t := range body.Templates() {
		tName := t.Name()
		if tName == name {
			tName = "body"
		}
		view.AddParseTree(tName, t.Tree)
	}

	return view, nil
}

func parsePartial(partial *templateFile) error {
	_, err := processTree(partial)
	return err
}

func readFile(fsys fs.FS, name string) (string, error) {
	f, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return string(f), nil
}

func hasExt(exts []string, ext string) bool {
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

func validateLayoutFile(exts []string, name string) error {
	ext := filepath.Ext(name)
	if !hasExt(exts, ext) {
		return fmt.Errorf("unsupported filename extension '%s'", ext)
	}

	nameOnly := strings.TrimSuffix(name, ext)
	if !strings.HasSuffix(strings.ToLower(nameOnly), "layout") {
		return fmt.Errorf("invalid file name, must be suffixed with 'layout'. e.g. layout%s", ext)
	}

	return nil
}

func placeholderFuncs() template.FuncMap {
	return map[string]any{
		renderFunc.String():  func(...string) string { return "" },
		partialFunc.String(): func(string, ...any) string { return "" },
	}
}

type templateType string

// template types
const (
	layoutType  templateType = "layout"
	viewType    templateType = "view"
	partialType templateType = "partial"
)

type templateFile struct {
	*template.Template
	typ  templateType
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
