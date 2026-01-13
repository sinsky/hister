package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/asciimoo/hister/config"
	//	"github.com/asciimoo/hister/gui"
	"github.com/asciimoo/hister/server"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var cfgFile string
var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:     "hister",
	Short:   "History search",
	Long:    `History search`,
	Version: "v0.1.0",
	//Run: func(_ *cobra.Command, _ []string) {
	//},
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "start server",
	Long:  ``,
	PreRun: func(_ *cobra.Command, _ []string) {
		initDB()
		initIndex()
	},
	Run: func(cmd *cobra.Command, _ []string) {
		setStrArg(cmd, "address", &cfg.Server.Address)
		server.Listen(cfg)
	},
}

var createConfigCmd = &cobra.Command{
	Use:   "create-config FILENAME",
	Short: "create default configuration file",
	Long:  `create-config FILENAME`,
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		fname := args[0]
		if _, err := os.Stat(fname); err == nil {
			exit(1, fmt.Sprintf(`File "%s" already exists`, fname))
		}
		dcfg := config.CreateDefaultConfig()
		cb, err := yaml.Marshal(dcfg)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(fname, cb, 0600); err != nil {
			exit(1, `Failed to create config file: `+err.Error())
		}
		fmt.Println("Config file created")
	},
}

var listURLsCmd = &cobra.Command{
	Use:   "list-urls",
	Short: "list indexed URLs",
	Long:  ``,
	PreRun: func(_ *cobra.Command, _ []string) {
		initIndex()
	},
	Run: func(cmd *cobra.Command, _ []string) {
		indexer.Iterate(func(d *indexer.Document) {
			fmt.Println(d.URL)
		})
	},
}

var indexCmd = &cobra.Command{
	Use:   "index URL",
	Short: "index URL",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		setStrArg(cmd, "server-url", &cfg.Server.BaseURL)
		u := args[0]
		client := &http.Client{}
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			exit(1, `Failed to download file: `+err.Error())
		}
		req.Header.Set("User-Agent", "Hister")
		r, err := client.Do(req)
		if err != nil {
			exit(1, `Failed to download file: `+err.Error())
		}
		defer r.Body.Close()
		contentType := r.Header.Get("Content-type")
		if !strings.Contains(contentType, "html") {
			exit(1, "Invalid content type: "+contentType)
		}
		buf := bytes.NewBuffer(nil)
		io.Copy(buf, r.Body)
		d := &indexer.Document{
			URL:  u,
			HTML: string(buf.Bytes()),
		}
		if err := d.Process(); err != nil {
			exit(1, `Failed to process document: `+err.Error())
		}
		dj, err := json.Marshal(d)
		if err != nil {
			exit(1, `Failed to encode document to JSON: `+err.Error())
		}
		req, err = http.NewRequest("POST", cfg.BaseURL("/add"), bytes.NewBuffer(dj))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			exit(1, `Failed to send page to hister: `+err.Error())
		}
		defer resp.Body.Close()
	},
}

func exit(errno int, msg string) {
	if errno != 0 {
		fmt.Println("Error!")
	}
	fmt.Println(msg)
	os.Exit(errno)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yml", "config file (default paths: ./config.yml or $HOME/.histerrc or $HOME/.config/hister/config.yml)")
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "set log level (possible options: error, warning, info, debug, trace)")
	rootCmd.PersistentFlags().StringP("search-url", "s", "https://google.com/search?q={query}", "set default search engine url")

	rootCmd.AddCommand(listenCmd)
	rootCmd.AddCommand(createConfigCmd)
	rootCmd.AddCommand(listURLsCmd)
	rootCmd.AddCommand(indexCmd)

	dcfg := config.CreateDefaultConfig()
	listenCmd.Flags().StringP("address", "a", dcfg.Server.Address, "Listen address")
	indexCmd.Flags().StringP("server-url", "u", dcfg.Server.BaseURL, "hister server URL")

	cobra.OnInitialize(initialize)

	lout := zerolog.ConsoleWriter{
		Out: os.Stderr,
		FormatTimestamp: func(i any) string {
			return i.(string)
		},
		FormatLevel: func(i any) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
		},
	}
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		dir, fn := filepath.Split(file)
		if dir == "" {
			return fn + ":" + strconv.Itoa(line)
		}
		_, subdir := filepath.Split(strings.TrimSuffix(dir, "/"))
		return subdir + "/" + fn + ":" + strconv.Itoa(line)
	}
	log.Logger = log.With().Caller().Logger()
	log.Logger = log.Output(lout)
}

func initialize() {
	initConfig()
	initLog()
	log.Debug().Str("filename", cfg.Filename()).Msg("Config initialization complete")
	log.Debug().Msg("Logging initialization complete")
}

func initConfig() {
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		exit(1, "Failed to initialize config: "+err.Error())
	}
	if v, _ := rootCmd.PersistentFlags().GetString("log-level"); v != "" && (rootCmd.Flags().Changed("log-level") || cfg.App.LogLevel == "") {
		cfg.App.LogLevel = v
	}
	if v, _ := rootCmd.PersistentFlags().GetString("search-url"); v != "" && (rootCmd.Flags().Changed("log-level") || cfg.App.SearchURL == "") {
		cfg.App.SearchURL = v
	}
}

func initLog() {
	switch cfg.App.LogLevel {
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Warn().Str("Invalid config log level", cfg.App.LogLevel)
	}
}

func setStrArg(cmd *cobra.Command, arg string, dest *string) {
	if v, err := cmd.Flags().GetString(arg); err == nil && (cmd.Flags().Changed(arg) || *dest == "") {
		*dest = v
	}
}

func initDB() {
	err := model.Init(cfg)
	if err != nil {
		exit(1, err.Error())
	}
	log.Debug().Msg("Database initialization complete")
}

func initIndex() {
	err := indexer.Init(cfg.IndexPath())
	if err != nil {
		exit(1, err.Error())
	}
	log.Debug().Msg("Indexer initialization complete")
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
