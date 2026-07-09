package mailer

import "maps"

// EmailType categorizes transactional emails for tracking and templates.
type EmailType string

const (
	EmailTypeVerification EmailType = "verification"
	EmailTypeAccessCode   EmailType = "access_code"
	EmailTypeMarketing    EmailType = "marketing"
	EmailTypeCustom       EmailType = "custom"
	EmailTypeInvitation   EmailType = "invitation"
	EmailTypeLinkInvite   EmailType = "link_invite"
	EmailTypeLinkAccess   EmailType = "link_access"
)

// Attachment is an email attachment carried inside an EmailJob.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Content     []byte `json:"content"`
}

// EmailJob is a unit of work placed on the queue and processed by a Worker.
// It carries either legacy Body/Subject or TemplateName/TemplateVariables for
// HTML/Text templates. When TemplateName is set, templates take precedence.
type EmailJob struct {
	ID                string            `json:"id"`
	EmailType         EmailType         `json:"email_type"`
	Recipient         string            `json:"recipient"`
	Subject           string            `json:"subject,omitempty"`
	Body              string            `json:"body,omitempty"`
	RenderedHTML      string            `json:"rendered_html,omitempty"`
	LinkName          string            `json:"link_name,omitempty"`
	LinkURL           string            `json:"link_url,omitempty"`
	Code              string            `json:"code,omitempty"`
	VerificationLink  string            `json:"verification_link,omitempty"`
	TemplateName      string            `json:"template_name,omitempty"`
	TemplateVariables map[string]string `json:"template_variables,omitempty"`
	Attachments       []Attachment      `json:"attachments,omitempty"`
	WorkspaceID       string            `json:"workspace_id,omitempty"`
	Locale            string            `json:"locale,omitempty"`
	Attempt           int               `json:"attempt"`
	MaxAttempts       int               `json:"max_attempts"`
	TrackOpens        bool              `json:"track_opens,omitempty"`
	TrackClicks       bool              `json:"track_clicks,omitempty"`
}

// TemplateVars returns a copy of the job's template variables, ensuring that
// legacy fields are also available as template variables for built-in templates.
func (j EmailJob) TemplateVars() map[string]string {
	vars := make(map[string]string, len(j.TemplateVariables)+6)
	maps.Copy(vars, j.TemplateVariables)
	if j.VerificationLink != "" {
		vars["VerificationLink"] = j.VerificationLink
	}
	if j.Code != "" {
		vars["Code"] = j.Code
	}
	if j.LinkName != "" {
		vars["LinkName"] = j.LinkName
	}
	if j.LinkURL != "" {
		vars["LinkURL"] = j.LinkURL
	}
	if j.Subject != "" {
		vars["Subject"] = j.Subject
	}
	if j.Body != "" {
		vars["Body"] = j.Body
	}
	return vars
}

// templateName returns the effective template name for this job. It falls back
// to the email type for built-in templates when no explicit template is set.
func (j EmailJob) templateName() string {
	if j.TemplateName != "" {
		return j.TemplateName
	}
	switch j.EmailType {
	case EmailTypeVerification:
		return "verification"
	case EmailTypeAccessCode:
		return "access_code"
	case EmailTypeMarketing:
		return "marketing"
	case EmailTypeInvitation:
		return "invitation"
	case EmailTypeLinkInvite:
		return "link_invite"
	case EmailTypeLinkAccess:
		return "link_access"
	default:
		return "custom"
	}
}
