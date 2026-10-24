package app

import (
	"fmt"
	"strconv"

	"notification-service/config"

	"github.com/whagency/core-common/logger"
	"github.com/wneessen/go-mail"
)

// setupMailerClient initializes the SMTP client connection based on the configuration file.
func (app *Application) setupMailerClient() error {
	if config.Mailer().Mailer == "log" {
		logger.Log().Infof("[Application:setupMailerClient][use log mailer]")
		return nil
	}

	logger.Log().Infof("[Application:setupMailerClient][host:%s][port:%s][user:%s][encryption:%s]",
		config.Mailer().Host,
		config.Mailer().Port,
		config.Mailer().Username,
		config.Mailer().Encryption)

	encryption := mail.WithTLSPolicy(mail.NoTLS)
	switch config.Mailer().Encryption {
	case "tls":
		encryption = mail.WithTLSPolicy(mail.TLSMandatory)
	case "ssl":
		encryption = mail.WithSSL()
	}

	port, err := strconv.Atoi(config.Mailer().Port)
	if err != nil {
		return fmt.Errorf("[Application:setupMailerClient][invalid port: %v]", err)
	}

	client, err := mail.NewClient(
		config.Mailer().Host,
		mail.WithPort(port),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithUsername(config.Mailer().Username),
		mail.WithPassword(config.Mailer().Password),
		mail.WithTimeout(config.Mailer().ConnectTimeout),
		encryption,
	)
	if err != nil {
		return fmt.Errorf("[Application:setupMailerClient][mail.NewClient][err: %w]", err)
	}

	app.mailerClient = client

	logger.Log().Info("[Application:setupMailerClient][msg: completed]")

	return nil
}
