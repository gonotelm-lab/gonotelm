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
)

// verifyEmailSubject has no code, subjects show up on lock screens
const verifyEmailSubject = "验证你的 GoNoteLM 邮箱"

type verifyEmailData struct {
	Link string
	Code string
}

// idempotency key includes recipient and both credentials: a resend rotates them and gets
// a new key, retries of the same email keep the key
func renderEmailVerification(to, link, code string) (mail.Message, error) {
	data := verifyEmailData{Link: link, Code: code}

	var htmlBody, textBody bytes.Buffer
	if err := verifyEmailHTML.Execute(&htmlBody, data); err != nil {
		return mail.Message{}, err
	}
	if err := verifyEmailText.Execute(&textBody, data); err != nil {
		return mail.Message{}, err
	}

	return mail.Message{
		To:      []string{to},
		Subject: verifyEmailSubject,
		HTML:    htmlBody.String(),
		Text:    textBody.String(),
		Headers: map[string]string{
			// RFC 3834: mark as auto-generated to stop auto-responder loops
			"Auto-Submitted": "auto-generated",
		},
		IdempotencyKey: verifyIdempotencyKey(to, link, code),
	}, nil
}

func verifyIdempotencyKey(to, link, code string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{"verify-email", to, link, code}, "\x00")))

	return hex.EncodeToString(sum[:])
}
