package template

// Built-in template names.
const (
	TemplateVerification  = "verification"
	TemplateAccessCode    = "access_code"
	TemplateMarketing     = "marketing"
	TemplateInvitation    = "invitation"
	TemplateRoomInvite    = "room_invite"
	TemplateLinkInvite    = "link_invite"
	TemplateLinkAccess    = "link_access"
	TemplatePasswordReset = "password_reset"
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

	_ = e.Register(TemplateRoomInvite, Template{
		Subject: "You've been added to {{.RoomName}} on {{.BrandName}}",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Data room invitation</title>
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
      <p>{{if .InviterEmail}}<strong>{{.InviterEmail}}</strong> added you{{else}}You have been added{{end}} to the <strong>{{.RoomName}}</strong> data room{{if .WorkspaceName}} in <strong>{{.WorkspaceName}}</strong>{{end}}.</p>
      <p>Open the data room to continue. If an NDA is required, you will be asked to sign it there.</p>
      <p style="text-align:center;"><a class="button" href="{{.InvitationLink}}">Open data room</a></p>
      <p>Or copy and paste this link into your browser:</p>
      <p style="word-break:break-all;"><a href="{{.InvitationLink}}">{{.InvitationLink}}</a></p>
      <p>If you were not expecting this, you can safely ignore this email.</p>
    </div>
    <div class="footer">
      &copy; {{.BrandName}}. All rights reserved.
    </div>
  </div>
</body>
</html>`,
		Text: `Hello,

{{if .InviterEmail}}{{.InviterEmail}} added you{{else}}You have been added{{end}} to the {{.RoomName}} data room{{if .WorkspaceName}} in {{.WorkspaceName}}{{end}}.

Open the data room to continue. If an NDA is required, you will be asked to sign it there.

{{.InvitationLink}}

If you were not expecting this, you can safely ignore this email.

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateRoomInvite+".zh-CN", Template{
		Subject: "{{.InviterEmail}} 已将你加入 {{.RoomName}}",
		HTML: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>数据室邀请</title>
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
      <p>你好，</p>
      <p>{{if .InviterEmail}}<strong>{{.InviterEmail}}</strong> 已将你加入{{else}}你已被加入{{end}}数据室 <strong>{{.RoomName}}</strong>{{if .WorkspaceName}}（工作区 <strong>{{.WorkspaceName}}</strong>）{{end}}。</p>
      <p>打开数据室即可继续。如需签署 NDA，会在进入后提示。</p>
      <p style="text-align:center;"><a class="button" href="{{.InvitationLink}}">打开数据室</a></p>
      <p>或复制以下链接到浏览器：</p>
      <p style="word-break:break-all;"><a href="{{.InvitationLink}}">{{.InvitationLink}}</a></p>
      <p>如非本人操作，请忽略此邮件。</p>
    </div>
    <div class="footer">
      &copy; {{.BrandName}}
    </div>
  </div>
</body>
</html>`,
		Text: `你好，

{{if .InviterEmail}}{{.InviterEmail}} 已将你加入{{else}}你已被加入{{end}}数据室 {{.RoomName}}{{if .WorkspaceName}}（工作区 {{.WorkspaceName}}）{{end}}。

打开数据室即可继续。如需签署 NDA，会在进入后提示。

{{.InvitationLink}}

如非本人操作，请忽略此邮件。

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateLinkInvite, Template{
		Subject: "You've been invited to view {{.LinkName}} on {{.BrandName}}",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Document Invitation</title>
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
      <p>You've been invited to view <strong>{{.LinkName}}</strong>.</p>
      <p style="text-align:center;"><a class="button" href="{{.InvitationLink}}">View document</a></p>
      <p>Or copy and paste this link into your browser:</p>
      <p style="word-break:break-all;"><a href="{{.InvitationLink}}">{{.InvitationLink}}</a></p>
      <p>If you were not expecting this invitation, you can safely ignore this email.</p>
    </div>
    <div class="footer">
      &copy; {{.BrandName}}. All rights reserved.
    </div>
  </div>
</body>
</html>`,
		Text: `Hello,

You've been invited to view {{.LinkName}} on {{.BrandName}}.

View document:
{{.InvitationLink}}

If you were not expecting this invitation, you can safely ignore this email.

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateLinkInvite+".zh-CN", Template{
		Subject: "您被邀请查看 {{.BrandName}} 上的 {{.LinkName}}",
		HTML: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>文档邀请</title>
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
    <div class="header"><h1>{{.BrandName}}</h1></div>
    <div class="body">
      <p>您好，</p>
      <p>您被邀请查看 <strong>{{.LinkName}}</strong>。</p>
      <p style="text-align:center;"><a class="button" href="{{.InvitationLink}}">查看文档</a></p>
      <p>或复制以下链接到浏览器打开：</p>
      <p style="word-break:break-all;"><a href="{{.InvitationLink}}">{{.InvitationLink}}</a></p>
      <p>如果您没有期待此邀请，可以忽略此邮件。</p>
    </div>
    <div class="footer">&copy; {{.BrandName}}. 保留所有权利。</div>
  </div>
</body>
</html>`,
		Text: `您好，

您被邀请查看 {{.BrandName}} 上的 {{.LinkName}}。

查看文档：
{{.InvitationLink}}

如果您没有期待此邀请，可以忽略此邮件。

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateLinkAccess, Template{
		Subject: "Someone viewed {{.LinkName}} on {{.BrandName}}",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Link viewed</title>
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
    <div class="header"><h1>{{.BrandName}}</h1></div>
    <div class="body">
      <p>Hello,</p>
      <p><strong>{{.VisitorEmail}}</strong> just viewed <strong>{{.LinkName}}</strong>.</p>
      <p style="text-align:center;"><a class="button" href="{{.LinkURL}}">View link analytics</a></p>
      <p>If you do not want to receive these notifications, you can disable "Notify on access" in the link settings.</p>
    </div>
    <div class="footer">&copy; {{.BrandName}}. All rights reserved.</div>
  </div>
</body>
</html>`,
		Text: `Hello,

{{.VisitorEmail}} just viewed {{.LinkName}} on {{.BrandName}}.

View link analytics:
{{.LinkURL}}

If you do not want to receive these notifications, disable "Notify on access" in the link settings.

- {{.BrandName}}`,
	})

	_ = e.Register(TemplateLinkAccess+".zh-CN", Template{
		Subject: "有人查看了 {{.BrandName}} 上的 {{.LinkName}}",
		HTML: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>链接被查看</title>
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
    <div class="header"><h1>{{.BrandName}}</h1></div>
    <div class="body">
      <p>您好，</p>
      <p><strong>{{.VisitorEmail}}</strong> 刚刚查看了 <strong>{{.LinkName}}</strong>。</p>
      <p style="text-align:center;"><a class="button" href="{{.LinkURL}}">查看链接分析</a></p>
      <p>如果您不想继续收到此类通知，可以在链接设置中关闭“访问时通知”。</p>
    </div>
    <div class="footer">&copy; {{.BrandName}}. 保留所有权利。</div>
  </div>
</body>
</html>`,
		Text: `您好，

{{.VisitorEmail}} 刚刚查看了 {{.BrandName}} 上的 {{.LinkName}}。

查看链接分析：
{{.LinkURL}}

如果您不想继续收到此类通知，可以在链接设置中关闭“访问时通知”。

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

	_ = e.Register(TemplatePasswordReset, Template{
		Subject: "Reset your {{.BrandName}} password",
		HTML: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Reset your password</title>
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
      <p>We received a request to reset the password for this account. Click the button below to choose a new password:</p>
      <p style="text-align:center;">
        <a class="button" href="{{.ResetLink}}">Reset password</a>
      </p>
      <p>Or copy and paste this link into your browser:</p>
      <p><a class="link" href="{{.ResetLink}}">{{.ResetLink}}</a></p>
      <p>This link expires in {{.ExpiryMinutes}} minutes and can be used only once.</p>
      <p>If you did not request a password reset, you can ignore this email. Your password will not change.</p>
    </div>
    <div class="footer">
      &copy; {{.BrandName}}. All rights reserved.
    </div>
  </div>
</body>
</html>`,
		Text: `Hello,

We received a request to reset the password for this account. Open the link below to choose a new password:

{{.ResetLink}}

This link expires in {{.ExpiryMinutes}} minutes and can be used only once.

If you did not request a password reset, you can ignore this email. Your password will not change.

- {{.BrandName}}`,
	})

	_ = e.Register(TemplatePasswordReset+".zh-CN", Template{
		Subject: "重置您的 {{.BrandName}} 密码",
		HTML: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>重置密码</title>
  <style>
    body { margin: 0; padding: 0; background-color: #f4f6f8; font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif; }
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
      <p>我们收到了重置此账号密码的请求。请点击下方按钮设置新密码：</p>
      <p style="text-align:center;">
        <a class="button" href="{{.ResetLink}}">重置密码</a>
      </p>
      <p>或将此链接复制到浏览器打开：</p>
      <p><a class="link" href="{{.ResetLink}}">{{.ResetLink}}</a></p>
      <p>此链接 {{.ExpiryMinutes}} 分钟后失效，且只能使用一次。</p>
      <p>如果这不是您本人的操作，请忽略此邮件。您的密码不会被更改。</p>
    </div>
    <div class="footer">&copy; {{.BrandName}}. 保留所有权利。</div>
  </div>
</body>
</html>`,
		Text: `您好，

我们收到了重置此账号密码的请求。请打开以下链接设置新密码：

{{.ResetLink}}

此链接 {{.ExpiryMinutes}} 分钟后失效，且只能使用一次。

如果这不是您本人的操作，请忽略此邮件。您的密码不会被更改。

- {{.BrandName}}`,
	})
}
