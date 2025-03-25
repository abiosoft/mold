package mold

import (
	"errors"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
)

// Engine represents a web page renderer, incorporating a specific layout.
// It provides a flexible way to generate HTML by combining views with a
// predefined layout structure.
//
// The layout defines the overall page structure,
// while views provide the dynamic content.
type Engine interface {
	// Render executes the layout template, merging it with the specified view template,
	// then writes the resulting HTML to the provided io.Writer.
	//
	// Parameters:
	//   w: The writer to which the rendered HTML will be written.
	//   view: The path of the view template whose content will be injected into the layout.
	//   data: The data to be made available to both the layout and view templates during rendering.
	//
	// Returns:
	//   An error, if any, that occurred during template execution or while writing to the writer.
	Render(w io.Writer, view string, data any) error
}

// Config is the configuration for a new [Engine].
type Config struct {
	fs        fs.FS
	layoutRaw string

	// options
	root    optionVal[string]
	layout  optionVal[string]
	exts    optionVal[[]string]
	funcMap optionVal[template.FuncMap]
}

// Option is a configuration option for a new [Engine].
// It is passed as argument(s) to [New].
type Option func(*Config)

// ErrNotFound is returned when a template is not found.
var ErrNotFound = errors.New("template not found")

// New creates a new [Engine] with fs as the underlying filesystem.
//
// The directory will be traversed and all files matching the configured filename extensions would be parsed.
// The filename extensions can be configured with [WithExt].
//
// At most one layout file would be parsed, if set with [WithLayout]. Others would be skipped.
//
// Layout files are files suffixed (case insensitively) with "layout" before the filename extension.
// e.g. "layout.html", "Layout.html", "AppLayout.html", "app_layout.html" would all be regarded as layout files
// and skipped.
//
// Example:
//
//	//go:embed web
//	var dir embed.FS
//	var engine, err = mold.New(dir)
//
// To use a directory on the filesystem.
//
//	var dir = os.DirFS("web")
//	var engine, err = mold.New(dir)
//
// To specify option(s) e.g. a custom layout file.
//
//	option := mold.WithLayout("layout.html")
//	engine, err := mold.New(fs, option)
func New(fs fs.FS, options ...Option) (Engine, error) {
	return newEngine(fs, options...)
}

// Must is a helper that wraps a call to a function returning ([Engine], error)
// and panics if the error is non-nil.
//
// Example:
//
//	engine := mold.Must(mold.New(fs))
func Must(l Engine, err error) Engine {
	if err != nil {
		panic(err)
	}
	return l
}

// With is a helper for specifying multiple options.
//
// Example:
//
//	options := mold.With(
//	    mold.WithLayout("layout.html"),
//	    mold.WithRoot("web"),
//	    mold.WithExt("html"),
//	)
//	engine := mold.New(dir, options)
func With(opts ...Option) Option {
	return func(c *Config) {
		for _, opt := range opts {
			opt(c)
		}
	}
}

// WithRoot configures the base directory from which template files are loaded.
func WithRoot(subdir string) Option {
	return func(c *Config) { c.root = newVal(subdir) }
}

// WithLayout configures the path to the layout file.
func WithLayout(layout string) Option {
	return func(c *Config) { c.layout = newVal(layout) }
}

// WithExt configures the filename extensions for the templates.
// Only files with the specified extensions would be parsed.
//
//	Default: [".html", ".gohtml", ".tpl", ".tmpl"]
func WithExt(exts ...string) Option {
	return func(c *Config) { c.exts = newVal(exts) }
}

// WithFuncMap configures the custom Go template functions.
func WithFuncMap(funcMap template.FuncMap) Option {
	return func(c *Config) { c.funcMap = newVal(funcMap) }
}

// HideFS wraps an [fs.FS] and restricts access to files with the specified extensions,
// essentially hiding them.
// This is useful to prevent exposing templates (or sensitive files) when serving
// static assets and templates from the same directory.
//
// Example:
//
//	var dir = os.DirFS("web")
//	http.Handle("/static", http.FileServerFS(mold.HideFS(dir)))
//
// If no extensions are specified, the default template extensions are used.
//
//	Default: [".html", ".gohtml", ".tpl", ".tmpl"]
func HideFS(fsys fs.FS, exts ...string) fs.FS {
	if len(exts) == 0 {
		exts = defaultExts
	}
	return &hideFS{
		FS:   fsys,
		exts: exts,
	}
}

var _ fs.FS = (*hideFS)(nil)

type hideFS struct {
	exts []string
	fs.FS
}

// Open implements fs.FS.
func (s *hideFS) Open(name string) (fs.File, error) {
	ext := filepath.Ext(name)
	if hasExt(s.exts, ext) {
		return nil, fs.ErrNotExist
	}

	return s.FS.Open(name)
}
