package template

import (
	"strings"
	"testing"
)

func TestEngineRegistersDefaults(t *testing.T) {
	e := NewEngine()
	for _, name := range []string{TemplateVerification, TemplateAccessCode, TemplateMarketing} {
		if !e.HasTemplate(name) {
			t.Fatalf("expected default template %q to be registered", name)
		}
	}
}

func TestRenderVerification(t *testing.T) {
	e := NewEngine()
	html, text, subject, err := e.Render(TemplateVerification, map[string]string{
		"BrandName":        "Acme",
		"VerificationLink": "https://acme.example.com/verify/abc",
		"ExpiryHours":      "24",
	})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if subject != "Verify your Acme account" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if !strings.Contains(html, "Verify email") {
		t.Errorf("HTML missing expected button text")
	}
	if !strings.Contains(text, "https://acme.example.com/verify/abc") {
		t.Errorf("text missing expected link")
	}
}

func TestRenderAccessCode(t *testing.T) {
	e := NewEngine()
	html, text, subject, err := e.Render(TemplateAccessCode, map[string]string{
		"BrandName": "Acme",
		"Code":      "123456",
		"LinkName":  "Q4 Report",
		"LinkURL":   "https://acme.example.com/l/xyz",
	})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if !strings.Contains(html, "123456") {
		t.Errorf("HTML missing expected code")
	}
	if !strings.Contains(text, "Q4 Report") {
		t.Errorf("text missing expected link name")
	}
	if !strings.Contains(subject, "Acme") {
		t.Errorf("subject missing brand name")
	}
}

func TestRegisterCustomTemplate(t *testing.T) {
	e := NewEngineEmpty()
	err := e.Register("custom_welcome", Template{
		Subject: "Welcome to {{.BrandName}}",
		HTML:    "<p>Hi {{.Name}}, welcome to {{.BrandName}}!</p>",
		Text:    "Hi {{.Name}}, welcome to {{.BrandName}}!",
	})
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	html, text, subject, err := e.Render("custom_welcome", map[string]string{
		"BrandName": "Acme",
		"Name":      "Alice",
	})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if subject != "Welcome to Acme" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if !strings.Contains(html, "Alice") {
		t.Errorf("HTML missing name")
	}
	if !strings.Contains(text, "Alice") {
		t.Errorf("text missing name")
	}
}

func TestRenderMissingTemplate(t *testing.T) {
	e := NewEngineEmpty()
	_, _, _, err := e.Render("does_not_exist", nil)
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

func TestPlaintextToHTMLFallback(t *testing.T) {
	e := NewEngineEmpty()
	_ = e.Register("text_only", Template{
		Subject: "Hello",
		Text:    "Line 1\nLine 2",
	})
	html, _, _, err := e.Render("text_only", nil)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if !strings.Contains(html, "<br>") {
		t.Errorf("expected HTML line break, got: %s", html)
	}
}

func TestRenderLocale(t *testing.T) {
	e := NewEngine()

	html, text, subject, err := e.RenderLocale(TemplateVerification, "zh-CN", map[string]string{
		"BrandName":        "Acme",
		"VerificationLink": "https://acme.example.com/verify/abc",
		"ExpiryHours":      "24",
	})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if subject != "验证您的 Acme 账户" {
		t.Errorf("unexpected Chinese subject: %s", subject)
	}
	if !strings.Contains(html, "验证邮箱") {
		t.Errorf("HTML missing expected Chinese button text")
	}
	if !strings.Contains(text, "验证您的邮箱地址") {
		t.Errorf("text missing expected Chinese text")
	}

	// Unknown locale falls back to the default (English) template.
	html, _, subject, err = e.RenderLocale(TemplateVerification, "fr", map[string]string{
		"BrandName":        "Acme",
		"VerificationLink": "https://acme.example.com/verify/abc",
		"ExpiryHours":      "24",
	})
	if err != nil {
		t.Fatalf("unexpected fallback render error: %v", err)
	}
	if subject != "Verify your Acme account" {
		t.Errorf("unexpected fallback subject: %s", subject)
	}
	if !strings.Contains(html, "Verify email") {
		t.Errorf("fallback HTML missing expected English button text")
	}
}
