package template

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"sync"
)

// Template is a single email template with HTML, plain text, and subject variants.
// All fields are Go text/template strings and can use the variables passed to Render.
type Template struct {
	Subject string
	HTML    string
	Text    string
}

// Engine renders registered templates with variable substitution.
// It is safe for concurrent use.
type Engine struct {
	mu        sync.RWMutex
	templates map[string]Template
	parsed    map[string]*template.Template
	funcMap   template.FuncMap
}

// NewEngine creates an engine with the built-in DealSignal templates.
func NewEngine() *Engine {
	e := &Engine{
		templates: make(map[string]Template),
		parsed:    make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"upper":    strings.ToUpper,
			"lower":    strings.ToLower,
			"trim":     strings.TrimSpace,
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		},
	}
	RegisterDefaults(e)
	return e
}

// NewEngineEmpty creates an engine without any built-in templates. Useful for
// tests or for fully custom template catalogs.
func NewEngineEmpty() *Engine {
	return &Engine{
		templates: make(map[string]Template),
		parsed:    make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"upper":    strings.ToUpper,
			"lower":    strings.ToLower,
			"trim":     strings.TrimSpace,
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		},
	}
}

// Register adds or overrides a template. Names are case-insensitive.
// Either HTML or Text must be provided; the other will be derived if missing.
func (e *Engine) Register(name string, tpl Template) error {
	name = normalizeTemplateName(name)
	if name == "" {
		return fmt.Errorf("template name cannot be empty")
	}
	if tpl.Subject == "" {
		return fmt.Errorf("template %q subject cannot be empty", name)
	}
	if tpl.HTML == "" && tpl.Text == "" {
		return fmt.Errorf("template %q must define HTML or Text body", name)
	}

	htmlSrc := tpl.HTML
	if htmlSrc == "" {
		htmlSrc = plaintextToHTML(tpl.Text)
	}
	textSrc := tpl.Text
	if textSrc == "" {
		textSrc = htmlToPlaintext(tpl.HTML)
	}

	htmlParsed, err := template.New(name + "-html").Funcs(e.funcMap).Parse(htmlSrc)
	if err != nil {
		return fmt.Errorf("parse HTML template %q: %w", name, err)
	}
	textParsed, err := template.New(name + "-text").Funcs(e.funcMap).Parse(textSrc)
	if err != nil {
		return fmt.Errorf("parse text template %q: %w", name, err)
	}
	subjParsed, err := template.New(name + "-subject").Funcs(e.funcMap).Parse(tpl.Subject)
	if err != nil {
		return fmt.Errorf("parse subject template %q: %w", name, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.templates[name] = tpl
	e.parsed[name+"-html"] = htmlParsed
	e.parsed[name+"-text"] = textParsed
	e.parsed[name+"-subject"] = subjParsed
	return nil
}

// HasTemplate reports whether a template is registered.
func (e *Engine) HasTemplate(name string) bool {
	name = normalizeTemplateName(name)
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.templates[name]
	return ok
}

// Render executes a template with vars and returns HTML, text, and subject.
// It renders the default (usually English) template registered under name.
func (e *Engine) Render(name string, vars map[string]string) (html, text, subject string, err error) {
	return e.RenderLocale(name, "", vars)
}

// RenderLocale executes a template for a specific locale, falling back to the
// base template if the locale variant is missing. Empty locale defaults to en.
func (e *Engine) RenderLocale(name, locale string, vars map[string]string) (html, text, subject string, err error) {
	name = normalizeTemplateName(name)
	locale = normalizeTemplateName(locale)
	if locale == "" {
		locale = "en"
	}

	keys := []string{name + "." + locale, name}
	var key string
	e.mu.RLock()
	for _, k := range keys {
		if _, ok := e.parsed[k+"-html"]; ok {
			key = k
			break
		}
	}
	e.mu.RUnlock()
	if key == "" {
		return "", "", "", fmt.Errorf("template %q not found", name)
	}

	html, err = e.execute(key+"-html", vars)
	if err != nil {
		return "", "", "", fmt.Errorf("render HTML template %q: %w", key, err)
	}
	text, err = e.execute(key+"-text", vars)
	if err != nil {
		return "", "", "", fmt.Errorf("render text template %q: %w", key, err)
	}
	subject, err = e.execute(key+"-subject", vars)
	if err != nil {
		return "", "", "", fmt.Errorf("render subject template %q: %w", key, err)
	}
	return html, text, subject, nil
}

// SubjectOnly renders only the subject line for a template using the default
// (English) locale.
func (e *Engine) SubjectOnly(name string, vars map[string]string) (string, error) {
	return e.SubjectOnlyLocale(name, "", vars)
}

// SubjectOnlyLocale renders only the subject line for a template in the given
// locale, falling back to the base template if the locale variant is missing.
func (e *Engine) SubjectOnlyLocale(name, locale string, vars map[string]string) (string, error) {
	name = normalizeTemplateName(name)
	locale = normalizeTemplateName(locale)
	if locale == "" {
		locale = "en"
	}

	keys := []string{name + "." + locale + "-subject", name + "-subject"}
	e.mu.RLock()
	var key string
	for _, k := range keys {
		if _, ok := e.parsed[k]; ok {
			key = k
			break
		}
	}
	e.mu.RUnlock()
	if key == "" {
		return "", fmt.Errorf("subject template %q not found", name)
	}
	return e.execute(key, vars)
}

func (e *Engine) execute(key string, vars map[string]string) (string, error) {
	e.mu.RLock()
	t := e.parsed[key]
	e.mu.RUnlock()
	if t == nil {
		return "", fmt.Errorf("template %q not found", key)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func normalizeTemplateName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// plaintextToHTML creates a minimal HTML body from a plain-text template source.
// The resulting HTML is wrapped in a <p> tag and line breaks are preserved.
func plaintextToHTML(text string) string {
	return "<p>" + strings.ReplaceAll(template.HTMLEscapeString(text), "\n", "<br>\n") + "</p>"
}

// htmlToPlaintext is a best-effort plain-text fallback from an HTML template source.
// It strips common tags and normalizes whitespace. Custom templates should provide
// their own Text variant for full control.
func htmlToPlaintext(html string) string {
	out := html
	out = strings.ReplaceAll(out, "<br>", "\n")
	out = strings.ReplaceAll(out, "<br/>", "\n")
	out = strings.ReplaceAll(out, "</p>", "\n")
	// Very coarse tag removal; not meant for untrusted HTML.
	for {
		start := strings.Index(out, "<")
		if start == -1 {
			break
		}
		end := strings.Index(out[start:], ">")
		if end == -1 {
			break
		}
		out = out[:start] + out[start+end+1:]
	}
	return strings.TrimSpace(out)
}
