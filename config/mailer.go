package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

var mailerSettings MailerSettings

type MailerSettings struct {
	Mailer                 string        `envconfig:"MAIL_MAILER" default:"smtp"`
	Encryption             string        `envconfig:"MAIL_ENCRYPTION" default:"tls"`
	Host                   string        `envconfig:"MAIL_HOST" required:"true"`
	Port                   string        `envconfig:"MAIL_PORT" required:"true"`
	Username               string        `envconfig:"MAIL_USERNAME" required:"true"`
	Password               string        `envconfig:"MAIL_PASSWORD" required:"true"`
	MailFromAddress        string        `envconfig:"MAIL_FROM_ADDRESS" required:"true"`
	MailFromName           string        `envconfig:"MAIL_FROM_NAME" required:"true"`
	MailAdminAddress       string        `envconfig:"MAIL_ADMIN" required:"true"`
	DefaultTemplatesDir    string        `envconfig:"DEFAULT_TEMPLATE_DIR" default:"data/mailer/html/"`
	DefaultTranslationsDir string        `envconfig:"DEFAULT_TRANSLATIONS_DIR" default:"data/mailer/i18n/"`
	DefaultAppName         string        `envconfig:"DEFAULT_APP_NAME" default:"app"`
	DefaultAppUrl          string        `envconfig:"DEFAULT_APP_URL" default:"https://app.com"`
	ConnectTimeout         time.Duration `envconfig:"MAIL_CONNECT_TIMEOUT" default:"5s"`
	SendTimeout            time.Duration `envconfig:"MAIL_SEND_TIMEOUT" default:"15s"`
}

// Mailer returns the Mailer settings
func Mailer() MailerSettings {
	return mailerSettings
}

// loadMailerConfig loads the Mailer settings from the environment variables
func loadMailerConfig() error {
	if err := envconfig.Process("", &mailerSettings); err != nil {
		return fmt.Errorf("[loadMailerConfig][err: %w]", err)
	}

	return nil
}
