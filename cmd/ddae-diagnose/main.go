package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/portable"
)

var version = "ddae7-local"
var revision = "unknown"
var source = "unknown"

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }
func run(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(out, "Commands: prepare, verify-bundle, self-test, run, keygen, decrypt, replay")
		return 2
	}
	flags := flag.NewFlagSet("ddae-diagnose", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "bundle directory")
	cfgPath := flags.String("config", "config.yaml", "diagnostic YAML")
	output := flags.String("output", "results", "protected output directory")
	capture := flags.String("capture", "", "encrypted capture file")
	privateKey := flags.String("private-key", "", "analysis private-key path")
	publicKey := flags.String("public-key", "", "recipient public-key path")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		fmt.Fprintln(out, "Invalid command arguments.")
		return 2
	}
	allowed := map[string]string{
		"prepare": " root ", "verify-bundle": " root ", "self-test": " output ", "run": " config ",
		"keygen": " private-key public-key ", "decrypt": " capture private-key output ", "replay": " capture private-key output ",
	}
	badFlag, explicitOutput := false, false
	flags.Visit(func(f *flag.Flag) {
		if !strings.Contains(allowed[args[0]], " "+f.Name+" ") {
			badFlag = true
		}
		if f.Name == "output" {
			explicitOutput = true
		}
	})
	if badFlag || ((args[0] == "decrypt" || args[0] == "replay") && !explicitOutput) {
		fmt.Fprintln(out, "Invalid command arguments. Decrypt/replay require an explicit output directory.")
		return 2
	}
	build := portable.BuildInfo{Version: version, Revision: revision, Source: source}
	result := ""
	code := 0
	var err error
	switch args[0] {
	case "prepare":
		err = portable.Prepare(*root)
		if err == nil {
			fmt.Fprintln(out, "Prepared. Edit config.yaml / exporter.yaml and create UTF-8 credential files. Keep private keys on the analysis host.")
		}
	case "verify-bundle":
		err = portable.VerifyBundle(*root)
	case "keygen":
		if *privateKey == "" || *publicKey == "" {
			err = portable.ErrInput
		} else {
			err = portable.GenerateKeys(*privateKey, *publicKey)
		}
		if err == nil {
			fmt.Fprintln(out, "Key pair created. Transfer only the public key; keep the private key separately.")
		}
	case "self-test":
		result, code = portable.SelfTest(*output, build)
	case "run":
		cfg, e := portable.LoadConfig(*cfgPath)
		if e != nil {
			err = e
			break
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		result, code = portable.Run(ctx, cfg, build)
	case "decrypt", "replay":
		if *capture == "" || *privateKey == "" {
			err = portable.ErrInput
			break
		}
		result, code = portable.Analyze(*capture, *privateKey, *output, args[0] == "decrypt", build)
	default:
		err = portable.ErrInput
	}
	if err != nil {
		fmt.Fprintln(out, "Operation failed: check command, configuration, file permissions, keys and bundle hashes.")
		return 2
	}
	if result != "" {
		fmt.Fprintln(out, "Result directory:", filepath.Base(result))
	}
	fmt.Fprintln(out, "Exit code:", code)
	return code
}
