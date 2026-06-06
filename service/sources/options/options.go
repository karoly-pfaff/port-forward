package options

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const (
	DefaultHost       = "127.0.0.1"
	DefaultPort       = 47831
	DefaultConfigPath = "data/forwards.json"
	DefaultStaticDir  = "web"
)

type Options struct {
	Service         bool
	ConfigPath      string
	Host            string
	Port            int
	StaticDir       string
	StaticDirSource string
}

type Env map[string]string

func FromOSEnv() Env {
	env := make(Env)
	for _, entry := range os.Environ() {
		for index, char := range entry {
			if char == '=' {
				env[entry[:index]] = entry[index+1:]
				break
			}
		}
	}
	return env
}

func Resolve(argv []string, env Env, cwd string) (Options, error) {
	cli, err := parseCLI(argv)
	if err != nil {
		return Options{}, err
	}

	configPath := pick(cli.configPath, env, "PORTIER_CONFIG", DefaultConfigPath)
	host := pick(cli.host, env, "PORTIER_HOST", DefaultHost)
	portValue := pick(cli.port, env, "PORTIER_PORT", strconv.Itoa(DefaultPort))
	staticDir, staticSource := pickStaticDir(cli.staticDir, env)

	port, err := parsePort(portValue)
	if err != nil {
		return Options{}, err
	}

	return Options{
		Service:         cli.service,
		ConfigPath:      resolvePath(cwd, configPath),
		Host:            host,
		Port:            port,
		StaticDir:       resolvePath(cwd, staticDir),
		StaticDirSource: staticSource,
	}, nil
}

type cliOptions struct {
	service    bool
	configPath *string
	host       *string
	port       *string
	staticDir  *string
}

func parseCLI(argv []string) (cliOptions, error) {
	var configPath string
	var host string
	var port string
	var staticDir string
	result := cliOptions{}

	flags := flag.NewFlagSet("portier-service", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&result.service, "service", false, "run in service mode")
	flags.StringVar(&configPath, "config", "", "path to rules config")
	flags.StringVar(&host, "host", "", "management host")
	flags.StringVar(&port, "port", "", "management port")
	flags.StringVar(&staticDir, "static-dir", "", "built client directory")

	if err := flags.Parse(argv); err != nil {
		return cliOptions{}, err
	}
	if flags.NArg() > 0 {
		return cliOptions{}, fmt.Errorf("unknown CLI argument: %s", flags.Arg(0))
	}

	flags.Visit(func(fl *flag.Flag) {
		switch fl.Name {
		case "config":
			result.configPath = &configPath
		case "host":
			result.host = &host
		case "port":
			result.port = &port
		case "static-dir":
			result.staticDir = &staticDir
		}
	})

	return result, nil
}

func pick(cliValue *string, env Env, envName string, fallback string) string {
	if cliValue != nil {
		return *cliValue
	}
	if value, ok := env[envName]; ok {
		return value
	}
	return fallback
}

func pickStaticDir(cliValue *string, env Env) (string, string) {
	if cliValue != nil {
		return *cliValue, "cli"
	}
	if value, ok := env["PORTIER_STATIC_DIR"]; ok {
		return value, "env"
	}
	return DefaultStaticDir, "default"
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("invalid port: " + value)
	}
	return port, nil
}

func resolvePath(cwd string, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(cwd, value))
}
