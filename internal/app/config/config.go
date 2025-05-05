package config

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type AccrualConfig struct {
	RateLimit     float64       // 10 запросов/сек
	WorkerCount   int           // Количество воркеров
	BatchSize     int           // Размер батча (например, 50)
	BatchTimeout  time.Duration // Таймаут формирования батча (например, 1s)
	QueueCapacity int           // Размер буфера очередей // 5 * BatchSize
	PollInterval  time.Duration // Интервал загрузки новых задач
}

type Config struct {
	HTTPServerAddr       string
	LogLevel             string
	DBConnect            string
	JwtKey               string
	AccrualSystemAddress string
	Accrual              AccrualConfig
}

// ValidateServerAddress проверяет валидность HTTP адреса сервера
func ValidateServerAddress(addr string) (string, error) {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	u, err := url.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported protocol scheme: %s", u.Scheme)
	}

	if u.Host == "" {
		return "", fmt.Errorf("missing host in address")
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		// Если ошибка из-за отсутствия порта, используем весь host
		if strings.Contains(err.Error(), "missing port") {
			host = u.Host
		} else {
			return "", fmt.Errorf("invalid host:port format: %w", err)
		}
	}

	// Проверка IP или домена
	if ip := net.ParseIP(host); ip == nil {
		if !isValidHostname(host) { // Обновленная функция проверки
			return "", fmt.Errorf("invalid domain or IP address: %s", host)
		}
	}

	// Проверка порта (если указан)
	if port != "" {
		if _, err := net.LookupPort("tcp", port); err != nil {
			return "", fmt.Errorf("invalid port: %s", port)
		}
	}

	// Нормализация URL
	normalized := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	if port == "" {
		if u.Scheme == "http" {
			normalized += ":80"
		} else if u.Scheme == "https" {
			normalized += ":443"
		}
	}

	return normalized, nil
}

// Обновленная проверка имени хоста
func isValidHostname(host string) bool {
	// Локальные имена
	if host == "localhost" || strings.HasPrefix(host, "localhost.") {
		return true
	}

	// Регулярное выражение для доменов:
	// 1. Допускает буквы, цифры, дефисы и точки
	// 2. Минимум 2 части (example.com)
	// 3. В каждой части не может начинаться/заканчиваться на дефис или точку
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return domainRegex.MatchString(host)
}

func Load() *Config {

	cfg := new(Config)
	cfg.HTTPServerAddr = "localhost:8080"
	cfg.JwtKey = "your_strong_secret_here"

	cfgAccrual := new(AccrualConfig)
	cfgAccrual.RateLimit = 10
	cfgAccrual.WorkerCount = 3
	cfgAccrual.BatchSize = 50
	cfgAccrual.PollInterval = 10 * time.Second
	cfgAccrual.BatchTimeout = 5 * time.Second
	cfgAccrual.QueueCapacity = 250

	cfg.Accrual = *cfgAccrual

	flag.Func("a", "Gofermart Server address host:port", func(flagValue string) error {

		/*
			flagValue, err := ValidateServerAddress(flagValue)

			if err != nil {
				return err
			}
		*/

		cfg.HTTPServerAddr = flagValue
		return nil
	})

	flag.Func("r", "Accrual server address host:port", func(flagValue string) error {

		flagValue, err := ValidateServerAddress(flagValue)

		if err != nil {
			return err
		}

		cfg.AccrualSystemAddress = flagValue
		return nil
	})

	flag.StringVar(&cfg.LogLevel, "l", "info", "log level")

	flag.StringVar(&cfg.DBConnect, "d", "", "Database connect string")

	flag.Parse()

	if envHTTPServerAddr := os.Getenv("RUN_ADDRESS"); envHTTPServerAddr != "" {

		/*
			envHTTPServerAddr, err := ValidateServerAddress(envHTTPServerAddr)
			if err != nil {
				log.Fatalf("error validate RUN_ADDRESS: %s", err)
			}
		*/

		cfg.HTTPServerAddr = envHTTPServerAddr
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

	if envHTTPAccrualAddr := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envHTTPAccrualAddr != "" {

		envHTTPAccrualAddr, err := ValidateServerAddress(envHTTPAccrualAddr)
		if err != nil {
			log.Fatalf("error validate ACCRUAL_SYSTEM_ADDRESS: %s", err)
		}

		cfg.AccrualSystemAddress = envHTTPAccrualAddr
	}

	return cfg

}
