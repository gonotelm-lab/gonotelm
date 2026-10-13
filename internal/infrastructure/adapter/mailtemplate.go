package adapter

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	htmltmpl "html/template"
	"strings"
	texttmpl "text/template"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

//go:embed mailtemplate
var mailTemplateFS embed.FS

var (
	verifyEmailHTML = htmltmpl.Must(htmltmpl.ParseFS(mailTemplateFS, "mailtemplate/verify_email.gohtml"))
	verifyEmailText = texttmpl.Must(texttmpl.ParseFS(mailTemplateFS, "mailtemplate/verify_email.txt"))

	resetPasswordHTML = htmltmpl.Must(htmltmpl.ParseFS(mailTemplateFS, "mailtemplate/reset_password.gohtml"))
	resetPasswordText = texttmpl.Must(texttmpl.ParseFS(mailTemplateFS, "mailtemplate/reset_password.txt"))
)

// verifyEmailSubject has no code, subjects show up on lock screens
const verifyEmailSubject = "验证你的 GoNoteLM 邮箱"

// resetPasswordSubject has no credential in it either
const resetPasswordSubject = "重设你的 GoNoteLM 密码"

// the kind keeps the two emails on separate idempotency keys
const (
	verifyEmailKind        = "verify-email"
	resetPasswordEmailKind = "password-reset"
)

// verifyEmailData 注册验证邮件只带验证码，用户在验证页输入
type verifyEmailData struct {
	Code string
}

// resetPasswordData 找回密码邮件只带链接，点开就是重置页
type resetPasswordData struct {
	Link string
}

func renderEmailVerification(to, code string) (mail.Message, error) {
	htmlBody, textBody, err := renderBodies(verifyEmailHTML, verifyEmailText, verifyEmailData{Code: code})
	if err != nil {
		return mail.Message{}, err
	}

	return buildMessage(verifyEmailKind, verifyEmailSubject, to, code, htmlBody, textBody), nil
}

func renderPasswordReset(to, link string) (mail.Message, error) {
	htmlBody, textBody, err := renderBodies(resetPasswordHTML, resetPasswordText, resetPasswordData{Link: link})
	if err != nil {
		return mail.Message{}, err
	}

	return buildMessage(resetPasswordEmailKind, resetPasswordSubject, to, link, htmlBody, textBody), nil
}

func renderBodies(
	htmlTpl *htmltmpl.Template,
	textTpl *texttmpl.Template,
	data any,
) (htmlBody, textBody string, err error) {
	var htmlBuf, textBuf bytes.Buffer
	if err := htmlTpl.Execute(&htmlBuf, data); err != nil {
		return "", "", err
	}
	if err := textTpl.Execute(&textBuf, data); err != nil {
		return "", "", err
	}

	return htmlBuf.String(), textBuf.String(), nil
}

func buildMessage(kind, subject, to, credential, htmlBody, textBody string) mail.Message {
	return mail.Message{
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
		Headers: map[string]string{
			// RFC 3834: mark as auto-generated to stop auto-responder loops
			"Auto-Submitted": "auto-generated",
		},
		IdempotencyKey: emailIdempotencyKey(kind, to, credential),
	}
}

// idempotency key includes recipient and the credential: re-issuing rotates it and gets
// a new key, retries of the same email keep the key
func emailIdempotencyKey(kind, to, credential string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{kind, to, credential}, "\x00")))

	return hex.EncodeToString(sum[:])
}
