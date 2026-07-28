package mail

import (
	"fmt"
	"html/template"
	"strings"
)

// 极简品牌邮件壳:站点名头 + 正文 + 按钮链接。内嵌模板不做用户自定义(spec §5)。
const shellTpl = `<!doctype html><html><body style="margin:0;padding:32px;background:#f5f5f4;font-family:sans-serif">
<div style="max-width:480px;margin:0 auto;background:#fff;border:1px solid #e5e5e3;padding:32px">
<div style="font-weight:800;font-size:18px;margin-bottom:20px">{{.SiteName}}</div>
<p style="font-size:14px;line-height:1.7;color:#333">{{.Body}}</p>
<p style="margin:28px 0"><a href="{{.Link}}" style="background:#111;color:#fff;padding:10px 22px;text-decoration:none;font-size:14px">{{.Button}}</a></p>
<p style="font-size:12px;color:#999">{{.LinkHint}}:<br>{{.Link}}</p>
<p style="font-size:12px;color:#999">{{.Ignore}}</p>
</div></body></html>`

var shell = template.Must(template.New("shell").Parse(shellTpl))

func render(siteName, body, link, button, linkHint, ignore string) string {
	var b strings.Builder
	_ = shell.Execute(&b, map[string]string{
		"SiteName": siteName, "Body": body, "Link": link, "Button": button,
		"LinkHint": linkHint, "Ignore": ignore,
	})
	return b.String()
}

func isEN(lang string) bool { return lang == "en" }

// RenderResetPassword 重置密码邮件(链接 1 小时有效)。lang=="en" 英文,否则中文。
func RenderResetPassword(siteName, link, lang string) (subject, html string) {
	if isEN(lang) {
		return fmt.Sprintf("Reset your %s password", siteName),
			render(siteName,
				"We received a request to reset your password. Click the button below to set a new password. This link is valid for 1 hour.",
				link, "Reset password",
				"If the button does not work, copy this link into your browser",
				"If you did not request this, you can ignore this email.")
	}
	return fmt.Sprintf("重置你的 %s 密码", siteName),
		render(siteName,
			"我们收到了你的密码重置请求,点击下方按钮设置新密码。链接 1 小时内有效。",
			link, "重置密码",
			"若按钮无法点击,复制链接到浏览器打开",
			"若这不是你发起的操作,忽略本邮件即可。")
}

// RenderImageRejected 内容审核未通过通知（克制文案，不写违规细节）。
func RenderImageRejected(siteName, imageKey, imageName, lang string) (subject, html string) {
	label := imageKey
	if imageName != "" {
		label = imageName + " (" + imageKey + ")"
	}
	if isEN(lang) {
		body := "An image you uploaded did not pass review and is no longer publicly available: " + label + "."
		return fmt.Sprintf("[%s] Image review update", siteName),
			render(siteName, body, "#", "Open site", "You can sign in to manage your library", "This is an automated message.")
	}
	body := "你上传的图片未通过审核，已不可公开访问：" + label + "。"
	return fmt.Sprintf("[%s] 图片审核通知", siteName),
		render(siteName, body, "#", "打开站点", "可登录后管理图库", "本邮件由系统自动发送。")
}

// RenderChangeEmail 换绑邮箱确认（链接 1 小时有效）。
func RenderChangeEmail(siteName, link, lang string) (subject, html string) {
	if isEN(lang) {
		return fmt.Sprintf("Confirm your new %s email", siteName),
			render(siteName,
				"Click the button below to confirm this email address for your account. This link is valid for 1 hour.",
				link, "Confirm email",
				"If the button does not work, copy this link into your browser",
				"If you did not request this, you can ignore this email.")
	}
	return fmt.Sprintf("确认你的 %s 新邮箱", siteName),
		render(siteName,
			"点击下方按钮确认将此邮箱绑定到你的账号。链接 1 小时内有效。",
			link, "确认邮箱",
			"若按钮无法点击,复制链接到浏览器打开",
			"若这不是你发起的操作,忽略本邮件即可。")
}

// RenderVerifyEmail 邮箱验证邮件(链接 24 小时有效)。lang=="en" 英文,否则中文。
func RenderVerifyEmail(siteName, link, lang string) (subject, html string) {
	if isEN(lang) {
		return fmt.Sprintf("Verify your %s email", siteName),
			render(siteName,
				"Thanks for signing up! Click the button below to verify your email. This link is valid for 24 hours.",
				link, "Verify email",
				"If the button does not work, copy this link into your browser",
				"If you did not request this, you can ignore this email.")
	}
	return fmt.Sprintf("验证你的 %s 邮箱", siteName),
		render(siteName,
			"感谢注册!点击下方按钮完成邮箱验证。链接 24 小时内有效。",
			link, "验证邮箱",
			"若按钮无法点击,复制链接到浏览器打开",
			"若这不是你发起的操作,忽略本邮件即可。")
}
