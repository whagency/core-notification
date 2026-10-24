package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"notification-service/config"

	dbcommon "github.com/whagency/core-common/db"
	"github.com/whagency/core-common/db/migrator"
)

// _defaultTimeout is the default duration to wait before timing out a migration command.
const _defaultTimeout = 30 * time.Second

func main() {
	if err := runCommand(); err != nil {
		log.Printf("[commands:migrate][err: %s]", err)
		os.Exit(1)
	}
}

func runCommand() error {
	var (
		command = flag.String("command", migrator.CommandUp.String(), fmt.Sprintf("migration command (%s|%s|%s|%s|%s|%s)", migrator.CommandUp, migrator.CommandDown, migrator.CommandSteps, migrator.CommandVersion, migrator.CommandForce, migrator.CommandDrop))
		steps   = flag.Int("steps", 1, "number of steps to migrate (used with steps command)")
		timeout = flag.Duration("timeout", _defaultTimeout, "command timeout")
		version = flag.Int("version", 0, "set version (used with force command)")
	)
	flag.Parse()

	if err := config.InitForDBMigration(); err != nil {
		return err
	}

	cfgDB := config.DB()
	commons := config.Commons()

	var cfg migrator.Config
	switch config.DB().DriverName {
	case dbcommon.DriverNamePostgres:
		cfg = migrator.Config{
			Driver: dbcommon.DriverPostgres,
			DSN: fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
				cfgDB.Host,
				cfgDB.Port,
				cfgDB.User,
				cfgDB.Password,
				cfgDB.DBName,
				cfgDB.SSLMode,
				commons.TimeZone,
			),
		}
	default:
		return fmt.Errorf("unsupported DB_DRIVER value: %s", cfgDB.DriverName)
	}

	manager, err := migrator.New(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cmd, err := migrator.ParseCommand(*command)
	if err != nil {
		return err
	}

	status, err := manager.Execute(ctx, migrator.ExecuteConfig{Command: cmd, Steps: *steps, Version: *version})
	if err != nil {
		return err
	}

	if cmd == migrator.CommandVersion {
		log.Printf("[commands:migrate][command: %s][version: %d][dirty: %t]", *command, status.Version, status.Dirty)
	} else {
		log.Printf("[commands:migrate][command: %s][msg: completed successfully]", *command)
	}

	return nil
}
