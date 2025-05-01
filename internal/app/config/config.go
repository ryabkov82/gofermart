package config

import (
	"errors"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPServerAddr  string
	HTTPAccrualAddr string
	LogLevel        string
	DBConnect       string
	JwtKey          string
}

func validateHTTPServerAddr(addr string) error {

	hp := strings.Split(addr, ":")
	if len(hp) != 2 {
		return errors.New("need address in a form host:port")
	}
	_, err := strconv.Atoi(hp[1])

	return err
}

func Load() *Config {

	cfg := new(Config)
	cfg.HTTPServerAddr = "localhost:8080"
	cfg.JwtKey = "your_strong_secret_here"

	flag.Func("a", "Gofermart Server address host:port", func(flagValue string) error {

		err := validateHTTPServerAddr(flagValue)

		if err != nil {
			return err
		}

		cfg.HTTPServerAddr = flagValue
		return nil
	})

	flag.Func("r", "Accrual server address host:port", func(flagValue string) error {

		err := validateHTTPServerAddr(flagValue)

		if err != nil {
			return err
		}

		cfg.HTTPAccrualAddr = flagValue
		return nil
	})

	flag.StringVar(&cfg.LogLevel, "l", "info", "log level")

	flag.StringVar(&cfg.DBConnect, "d", "", "Database connect string")

	flag.Parse()

	if envHTTPServerAddr := os.Getenv("RUN_ADDRESS"); envHTTPServerAddr != "" {

		err := validateHTTPServerAddr(envHTTPServerAddr)
		if err != nil {
			log.Fatalf("error validate RUN_ADDRESS: %s", err)
		}

		cfg.HTTPServerAddr = envHTTPServerAddr
	}

	if envHTTPAccrualAddr := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envHTTPAccrualAddr != "" {

		err := validateHTTPServerAddr(envHTTPAccrualAddr)
		if err != nil {
			log.Fatalf("error validate ACCRUAL_SYSTEM_ADDRESS: %s", err)
		}

		cfg.HTTPAccrualAddr = envHTTPAccrualAddr
	}

	if envDBConnect := os.Getenv("DATABASE_URI"); envDBConnect != "" {
		cfg.DBConnect = envDBConnect
	}

	if envJWTSECRET := os.Getenv("JWT_SECRET"); envJWTSECRET != "" {
		if len(envJWTSECRET) < 32 {
			log.Fatal("JWT_SECRET must be at least 32 characters long")
		}

		cfg.JwtKey = envJWTSECRET
	}

	return cfg

}
