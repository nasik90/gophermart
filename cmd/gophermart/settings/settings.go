package settings

import (
	"flag"
	"os"
)

type Options struct {
	ServerAddress string
	LogLevel      string
	DatabaseURI   string
}

func ParseFlags(o *Options) {
	flag.StringVar(&o.ServerAddress, "a", ":8181", "address and port to run server")
	flag.StringVar(&o.LogLevel, "l", "debug", "log level")
	flag.StringVar(&o.DatabaseURI, "d", "host=localhost user=postgres password=xxxx dbname=gophermart sslmode=disable", "database connection string")
	//flag.StringVar(&o.DatabaseURI, "d", "", "database connection string")
	flag.Parse()

	if serverAddress := os.Getenv("RUN_ADDRESS"); serverAddress != "" {
		o.ServerAddress = serverAddress
	}
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		o.LogLevel = envLogLevel
	}
	if databaseURI := os.Getenv("DATABASE_URI"); databaseURI != "" {
		o.DatabaseURI = databaseURI
	}
}
