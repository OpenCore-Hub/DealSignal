package template

// Built-in template names.
const (
	TemplateVerification = "verification"
	TemplateAccessCode   = "access_code"
	TemplateMarketing    = "marketing"
	TemplateInvitation   = "invitation"
)

// RegisterDefaults registers the built-in DealSignal templates.
func RegisterDefaults(e *Engine) {
	_ = e.Register(TemplateVerification, Template{
		Subject: "Verify your {{.BrandName}} account",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Verify your email</title>
  <style>
    body { margin: 0; padding: 0; background-color: #f4f6f8; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    .container { max-width: 600px; margin: 40px auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
    .header { background: #111827; padding: 32px 24px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 24px; font-weight: 600; }
    .body { padding: 32px 24px; color: #374151; font-size: 16px; line-height: 1.6; }
    .body p { margin: 0 0 16px; }
    .button { display: inline-block; margin: 16px 0; padding: 14px 28px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 8px; font-weight: 600; }
    .button:hover { background: #1d4ed8; }
    .link { word-break: break-all; color: #2563eb; }
    .footer { padding: 24px; text-align: center; font-size: 13px; color: #9ca3af; background: #f9fafb; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>{{.BrandName}}</h1>
    </div>
    <div class="body">
      <p>Hello,</p>
      <p>Please verify your email address by clicking the button below:</p>
      <p style="text-align:center;">
        <a class="button" href="{{.VerificationLink}}">Verify email</a>
      </p>
      <p>Or copy and paste this link into your browser:</p>
      <p><a class="link" href="{{.VerificationLink}}">{{.VerificationLink}}</a></p>
      <p>This link expires in {{.ExpiryHours}} hours.</p>
      <p>If you did not create an account, you can safely ignore this email.</p>
    </div>
    <div class="footer">
      &copy; {{.BrandName}}. All rights reserved.
    </div>
  </div>
</body>
</html>`,
		Text: `Hello,

Please verify your email address by clicking the link below:

{{.VerificationLink}}

This link expires in {{.ExpiryHours}} hours.

If you did not create an account, you can safely ignore this email.

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateVerification+".zh-CN", Template{
		Subject: "验证您的 {{.BrandName}} 账户",
		HTML: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>验证您的邮箱</title>
  <style>
    body { margin: 0; padding: 0; background-color: #f4f6f8; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    .container { max-width: 600px; margin: 40px auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
    .header { background: #111827; padding: 32px 24px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 24px; font-weight: 600; }
    .body { padding: 32px 24px; color: #374151; font-size: 16px; line-height: 1.6; }
    .body p { margin: 0 0 16px; }
    .button { display: inline-block; margin: 16px 0; padding: 14px 28px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 8px; font-weight: 600; }
    .link { word-break: break-all; color: #2563eb; }
    .footer { padding: 24px; text-align: center; font-size: 13px; color: #9ca3af; background: #f9fafb; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header"><h1>{{.BrandName}}</h1></div>
    <div class="body">
      <p>您好，</p>
      <p>请点击下方按钮验证您的邮箱地址：</p>
      <p style="text-align:center;"><a class="button" href="{{.VerificationLink}}">验证邮箱</a></p>
      <p>或复制以下链接到浏览器打开：</p>
      <p><a class="link" href="{{.VerificationLink}}">{{.VerificationLink}}</a></p>
      <p>此链接将在 {{.ExpiryHours}} 小时内失效。</p>
      <p>如果您没有创建账户，可以忽略此邮件。</p>
    </div>
    <div class="footer">&copy; {{.BrandName}}. 保留所有权利。</div>
  </div>
</body>
</html>`,
		Text: `您好，

请点击以下链接验证您的邮箱地址：

{{.VerificationLink}}

此链接将在 {{.ExpiryHours}} 小时内失效。

如果您没有创建账户，可以忽略此邮件。

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateAccessCode, Template{
		Subject: "Your {{.BrandName}} document access code",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Your access code</title>
  <style>
    body { margin: 0; padding: 0; background-color: #f4f6f8; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    .container { max-width: 600px; margin: 40px auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
    .header { background: #111827; padding: 32px 24px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 24px; font-weight: 600; }
    .body { padding: 32px 24px; color: #374151; font-size: 16px; line-height: 1.6; }
    .body p { margin: 0 0 16px; }
    .code { display: inline-block; padding: 16px 32px; background: #f3f4f6; border: 1px dashed #d1d5db; border-radius: 8px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size: 28px; letter-spacing: 4px; font-weight: 700; color: #111827; }
    .button { display: inline-block; margin: 16px 0; padding: 14px 28px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 8px; font-weight: 600; }
    .footer { padding: 24px; text-align: center; font-size: 13px; color: #9ca3af; background: #f9fafb; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>{{.BrandName}}</h1>
    </div>
    <div class="body">
      <p>Hello,</p>
      <p><strong>{{.LinkName}}</strong> has been shared with you.</p>
      <p>Your access code is:</p>
      <p style="text-align:center;"><span class="code">{{.Code}}</span></p>
      <p>Enter this code on the viewing page to access the document:</p>
      <p style="text-align:center;"><a class="button" href="{{.LinkURL}}">Open document</a></p>
      <p>This code is valid as long as the link is active.</p>
      <p>If you did not request access, you can safely ignore this email.</p>
    </div>
    <div class="footer">
      &copy; {{.BrandName}}. All rights reserved.
    </div>
  </div>
</body>
</html>`,
		Text: `Hello,

{{.LinkName}} has been shared with you.

Your access code is: {{.Code}}

Enter this code on the viewing page to access the document:

{{.LinkURL}}

This code is valid as long as the link is active.

If you did not request access, you can safely ignore this email.

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateAccessCode+".zh-CN", Template{
		Subject: "您的 {{.BrandName}} 文档访问码",
		HTML: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>您的访问码</title>
  <style>
    body { margin: 0; padding: 0; background-color: #f4f6f8; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    .container { max-width: 600px; margin: 40px auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
    .header { background: #111827; padding: 32px 24px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 24px; font-weight: 600; }
    .body { padding: 32px 24px; color: #374151; font-size: 16px; line-height: 1.6; }
    .body p { margin: 0 0 16px; }
    .code { display: inline-block; padding: 16px 32px; background: #f3f4f6; border: 1px dashed #d1d5db; border-radius: 8px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size: 28px; letter-spacing: 4px; font-weight: 700; color: #111827; }
    .button { display: inline-block; margin: 16px 0; padding: 14px 28px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 8px; font-weight: 600; }
    .footer { padding: 24px; text-align: center; font-size: 13px; color: #9ca3af; background: #f9fafb; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header"><h1>{{.BrandName}}</h1></div>
    <div class="body">
      <p>您好，</p>
      <p><strong>{{.LinkName}}</strong> 已与您共享。</p>
      <p>您的访问码是：</p>
      <p style="text-align:center;"><span class="code">{{.Code}}</span></p>
      <p>在查看页面输入此访问码以访问文档：</p>
      <p style="text-align:center;"><a class="button" href="{{.LinkURL}}">打开文档</a></p>
      <p>此访问码在链接有效期内均可使用。</p>
      <p>如果您没有请求访问，可以忽略此邮件。</p>
    </div>
    <div class="footer">&copy; {{.BrandName}}. 保留所有权利。</div>
  </div>
</body>
</html>`,
		Text: `您好，

{{.LinkName}} 已与您共享。

您的访问码是：{{.Code}}

在查看页面输入此访问码以访问文档：

{{.LinkURL}}

此访问码在链接有效期内均可使用。

如果您没有请求访问，可以忽略此邮件。

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateInvitation, Template{
		Subject: "You've been invited to join {{.WorkspaceName}} on {{.BrandName}}",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Workspace Invitation</title>
  <style>
    body { margin: 0; padding: 0; background-color: #f4f6f8; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    .container { max-width: 600px; margin: 40px auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
    .header { background: #111827; padding: 32px 24px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 24px; font-weight: 600; }
    .body { padding: 32px 24px; color: #374151; font-size: 16px; line-height: 1.6; }
    .body p { margin: 0 0 16px; }
    .button { display: inline-block; margin: 16px 0; padding: 14px 28px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 8px; font-weight: 600; }
    .footer { padding: 24px; text-align: center; font-size: 13px; color: #9ca3af; background: #f9fafb; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>{{.BrandName}}</h1>
    </div>
    <div class="body">
      <p>Hello,</p>
      <p>{{if .InviterEmail}}<strong>{{.InviterEmail}}</strong> has invited you{{else}}You have been invited{{end}} to join the <strong>{{.WorkspaceName}}</strong> workspace on {{.BrandName}} as a <strong>{{.Role}}</strong>.</p>
      <p style="text-align:center;"><a class="button" href="{{.InvitationLink}}">Accept invitation</a></p>
      <p>Or copy and paste this link into your browser:</p>
      <p style="word-break:break-all;"><a href="{{.InvitationLink}}">{{.InvitationLink}}</a></p>
      <p>This invitation expires in {{.ExpiryDays}} days.</p>
      <p>If you were not expecting this invitation, you can safely ignore this email.</p>
    </div>
    <div class="footer">
      &copy; {{.BrandName}}. All rights reserved.
    </div>
  </div>
</body>
</html>`,
		Text: `Hello,

{{if .InviterEmail}}{{.InviterEmail}} has invited you{{else}}You have been invited{{end}} to join the {{.WorkspaceName}} workspace on {{.BrandName}} as a {{.Role}}.

Accept invitation:
{{.InvitationLink}}

This invitation expires in {{.ExpiryDays}} days.

If you were not expecting this invitation, you can safely ignore this email.

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateMarketing, Template{
		Subject: "{{.Subject}}",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Subject}}</title>
  <style>
    body { margin: 0; padding: 0; background-color: #f4f6f8; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    .container { max-width: 600px; margin: 40px auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
    .header { background: #111827; padding: 32px 24px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 24px; font-weight: 600; }
    .body { padding: 32px 24px; color: #374151; font-size: 16px; line-height: 1.6; }
    .body p { margin: 0 0 16px; }
    .button { display: inline-block; margin: 16px 0; padding: 14px 28px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 8px; font-weight: 600; }
    .footer { padding: 24px; text-align: center; font-size: 13px; color: #9ca3af; background: #f9fafb; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>{{.BrandName}}</h1>
    </div>
    <div class="body">
      {{if .PreviewText}}<p style="color:#6b7280;font-size:14px;">{{.PreviewText}}</p>{{end}}
      {{if .Headline}}<h2 style="margin-top:0;color:#111827;">{{.Headline}}</h2>{{end}}
      {{if .Body}}<p>{{.Body | safeHTML}}</p>{{end}}
      {{if .CTAUrl}}<p style="text-align:center;"><a class="button" href="{{.CTAUrl}}">{{if .CTAText}}{{.CTAText}}{{else}}Learn more{{end}}</a></p>{{end}}
    </div>
    <div class="footer">
      &copy; {{.BrandName}}. All rights reserved.<br>
      You received this email because you are subscribed to updates from {{.BrandName}}.
    </div>
  </div>
</body>
</html>`,
		Text: `{{.BrandName}}

{{if .Headline}}{{.Headline}}{{end}}

{{if .Body}}{{.Body}}{{end}}

{{if .CTAUrl}}{{if .CTAText}}{{.CTAText}}{{else}}Learn more{{end}}: {{.CTAUrl}}{{end}}

- {{.BrandName}}`,
	})
}
