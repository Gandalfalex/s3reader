package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"s3reader/internal/config"
	"s3reader/internal/logging"
	"s3reader/internal/store"
	"s3reader/internal/ui"
)

func main() {
	cfgFlag := flag.String("config", "", "path to settings.json (default: ./settings.json or ~/.config/s3reader/settings.json)")
	profileFlag := flag.String("profile", "", "open this profile directly")
	logFlag := flag.String("log", logging.DefaultPath(), `log file path ("off" to disable)`)
	debugFlag := flag.Bool("debug", false, "verbose logging incl. AWS SDK retries/responses")
	flag.Parse()

	logFile, err := logging.Setup(*logFlag, *debugFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "log:", err)
		os.Exit(1)
	}
	defer logFile.Close()
	store.Debug = *debugFlag

	if err := run(*cfgFlag, *profileFlag, *logFlag); err != nil {
		slog.Error("fatal", "err", err)
		fmt.Fprintln(os.Stderr, err)
		logFile.Close()
		os.Exit(1)
	}
}

func run(cfgFlag, profileFlag, logPath string) error {

	path, err := config.Path(cfgFlag)
	if err != nil {
		return err
	}
	settings, err := config.Load(path)
	if err != nil {
		return err
	}
	slog.Info("start", "config", path, "profiles", len(settings.Profiles))

	if profileFlag != "" {
		found := false
		for _, p := range settings.Profiles {
			if p.Name == profileFlag {
				settings.Profiles = []config.Profile{p}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("profile %q not found in %s", profileFlag, path)
		}
	}

	err = ui.Run(settings, logPath)
	slog.Info("exit")
	return err
}
