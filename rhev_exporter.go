// Prometheus exporter for RHEV version 3.
package main

import (
        "net/http"
        "os"
        "path"
        "strconv"
        "strings"

        "github.com/go-kit/kit/log"
        "github.com/go-kit/kit/log/level"
        "github.com/prometheus/client_golang/prometheus/promhttp"
        "github.com/spf13/viper"
        "gopkg.in/alecthomas/kingpin.v2"
)

type config struct {
        user     string
        password string
        endpoint string
        interval int
        host     string
        port     int
        path     string
}

func newConfig(fileName string, logger log.Logger) (config, error) {

        viper.SetDefault("endpoint", "") 
        viper.SetDefault("interval", 300)
        viper.SetDefault("path", "/metrics")
        viper.SetDefault("port", 9621)


        viper.SetConfigType("yaml")
        viper.AddConfigPath(".")

        if fileName != "" {
                level.Info(logger).Log("msg", "using provided configuration file", "file", fileName)

                dir, file := path.Split(fileName)
                viper.SetConfigName(file)
                viper.AddConfigPath(dir)
        }

	err := viper.ReadInConfig()
        if err != nil {
                return config{}, err
        }

        return config{
                user:     viper.Get("user").(string),
                password: viper.Get("password").(string),
                endpoint: viper.Get("endpoint").(string),
                interval: viper.Get("interval").(int),
                host:     viper.Get("host").(string),
                port:     viper.Get("port").(int),
                path:     viper.Get("path").(string),
        }, nil
}

func allowedLevel(logLevel string) level.Option {

        switch strings.ToLower(logLevel) {
        case "error":
                return level.AllowError()
        case "debug":
                return level.AllowDebug()
        default:
                return level.AllowInfo()
        }
}

func main() {

        logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
        logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

        cfgFile  := kingpin.Flag("config", "config file for one_exporter").Short('c').String()
        logLevel := kingpin.Flag("loglevel", "the log level to output").Short('l').Default("info").String()

        kingpin.Version(Version)
        kingpin.Parse()

        logger = level.NewFilter(logger, allowedLevel(*logLevel))
        level.Info(logger).Log("msg", "starting exporter for RHEV version 3")

        config, err := newConfig(*cfgFile, logger)
        if err != nil {
                level.Error(logger).Log("error", err)
                return
        }

        level.Debug(logger).Log("msg", "loaded config", "user", config.user, "endpoint", config.endpoint)

        initMetrics(config, logger)
        go recordMetrics(config, logger)

        level.Info(logger).Log("msg", "starting exporter", "host", config.host, "port", config.port, "path", config.path)
        http.Handle(config.path, promhttp.Handler())

        err = http.ListenAndServe(config.host+":"+strconv.Itoa(config.port), nil)
        if err != nil {
                level.Error(logger).Log("error", err)
        }
}
