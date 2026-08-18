package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Massnaev/jinay-server-panel/agent/internal/api"
	"github.com/Massnaev/jinay-server-panel/agent/internal/audit"
	"github.com/Massnaev/jinay-server-panel/agent/internal/auth"
	"github.com/Massnaev/jinay-server-panel/agent/internal/config"
	"github.com/Massnaev/jinay-server-panel/agent/internal/powercontrol"
)

var version = "0.1.0-dev"

func main() {
	cfg := config.FromEnv()
	cfg.Version = version
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	var err error
	switch command {
	case "serve":
		err = serve(cfg)
	case "user":
		err = userCommand(cfg, os.Args[2:])
	case "power":
		err = powerCommand(cfg, os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println(version)
		return
	default:
		err = fmt.Errorf("unknown command %q", command)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func powerCommand(cfg config.Config, args []string) error {
	if len(args) == 0 || args[0] != "apply" {
		return fmt.Errorf("usage: serverpanel power apply --profile eco|balanced|turbo")
	}
	flags := flag.NewFlagSet("power apply", flag.ContinueOnError)
	profile := flags.String("profile", "", "eco, balanced, or turbo")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	*profile = strings.ToLower(strings.TrimSpace(*profile))
	if err := powercontrol.ValidateProfile(*profile); err != nil {
		return fmt.Errorf("usage: serverpanel power apply --profile eco|balanced|turbo")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	result, err := (powercontrol.Client{Enabled: cfg.EnablePowerActions, SocketPath: cfg.PowerHelperSocket}).Apply(ctx, *profile)
	if err != nil {
		return err
	}
	fmt.Printf("Applied %s: governor=%s max=%.0fMHz turbo=%t policies=%d\n", result.Profile, result.Governor, result.MaximumFrequencyMHz, result.TurboAllowed, result.PoliciesChanged)
	return nil
}

func serve(cfg config.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return err
	}
	users, err := auth.Open(filepath.Join(cfg.DataDir, "users.json"))
	if err != nil {
		return err
	}
	if len(users.List()) == 0 {
		log.Printf("warning: no users exist; create one with serverpanel user add")
	}
	auditLog := audit.New(filepath.Join(cfg.DataDir, "audit.jsonl"))
	server := api.New(cfg, users, auditLog)
	server.StartHistoryCollector(context.Background(), 30*time.Second)
	httpServer := &http.Server{
		Addr: cfg.Listen, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("Jinay agent %s listening at %s", version, api.ListenAddress(cfg))
	return httpServer.ListenAndServe()
}

func userCommand(cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: serverpanel user add|list")
	}
	store, err := auth.Open(filepath.Join(cfg.DataDir, "users.json"))
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		flags := flag.NewFlagSet("user add", flag.ContinueOnError)
		username := flags.String("username", "", "username")
		role := flags.String("role", "admin", "admin, operator, or viewer")
		passwordStdin := flags.Bool("password-stdin", false, "read password from standard input")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *username == "" || !*passwordStdin {
			return fmt.Errorf("usage: serverpanel user add --username NAME --role admin --password-stdin")
		}
		content, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
		if err != nil {
			return err
		}
		password := strings.TrimRight(string(content), "\r\n")
		if err := store.Add(*username, *role, password); err != nil {
			return err
		}
		fmt.Printf("Created %s user %q.\n", *role, *username)
		return nil
	case "list":
		for _, user := range store.List() {
			fmt.Printf("%s\t%s\tdisabled=%t\n", user.Username, user.Role, user.Disabled)
		}
		return nil
	default:
		return fmt.Errorf("unknown user command %q", args[0])
	}
}
