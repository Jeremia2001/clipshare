package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	UseTLS   bool
}

type Service struct {
	config Config
}

func NewService(config Config) *Service {
	return &Service{config: config}
}

func (s *Service) SendMagicLink(toEmail, token, appURL string) error {
	verifyURL := fmt.Sprintf("%s/verify?token=%s&app=%s", appURL, token, appURL)

	data := struct {
		VerifyURL string
		AppName   string
	}{
		VerifyURL: verifyURL,
		AppName:   "ClipShare",
	}

	subject := "Sign in to ClipShare"
	body, err := s.renderTemplate(magicLinkTemplate, data)
	if err != nil {
		return err
	}

	return s.sendEmail(toEmail, subject, body)
}

func (s *Service) sendEmail(to, subject, body string) error {
	from := fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)
	msg := []byte(fmt.Sprintf(
		"To: %s\r\n"+
			"From: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"\r\n%s",
		to, from, subject, body,
	))

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	if s.config.UseTLS {
		return smtp.SendMail(addr, auth, s.config.From, []string{to}, msg)
	}

	// For non-TLS (development), use a simpler approach
	conn, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.Mail(s.config.From); err != nil {
		return err
	}
	if err := conn.Rcpt(to); err != nil {
		return err
	}

	wc, err := conn.Data()
	if err != nil {
		return err
	}
	defer wc.Close()

	_, err = wc.Write(msg)
	return err
}

func (s *Service) renderTemplate(tmpl string, data interface{}) (string, error) {
	t := template.Must(template.New("email").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var magicLinkTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign in to {{.AppName}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .container { background: #f9f9f9; border-radius: 8px; padding: 30px; }
        h1 { color: #6366f1; }
        .button { display: inline-block; background: #6366f1; color: white; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: 500; }
        .button:hover { background: #4f46e5; }
        .link { word-break: break-all; color: #6366f1; }
        .footer { margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Sign in to {{.AppName}}</h1>
        <p>Click the button below to sign in to your {{.AppName}} account:</p>
        <p><a href="{{.VerifyURL}}" class="button">Sign In</a></p>
        <p>Or copy and paste this link into your browser:</p>
        <p><a href="{{.VerifyURL}}" class="link">{{.VerifyURL}}</a></p>
        <p>This link will expire in 15 minutes.</p>
        <div class="footer">
            <p>If you didn't request this email, you can safely ignore it.</p>
            <p>{{.AppName}}</p>
        </div>
    </div>
</body>
</html>
`

func (s *Service) IsConfigured() bool {
	return s.config.Host != "" && s.config.Username != ""
}

func (s *Service) SendTestEmail(toEmail string) error {
	if !s.IsConfigured() {
		return fmt.Errorf("email service not configured")
	}

	return s.sendEmail(toEmail, "ClipShare Test", "<p>This is a test email from ClipShare.</p>")
}
