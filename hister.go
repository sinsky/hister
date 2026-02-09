package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/ui"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var cfgFile string
var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:     "hister",
	Short:   "Web history on steroids",
	Long:    ui.Banner,
	Version: "v0.1.0",
	//Run: func(_ *cobra.Command, _ []string) {
	//},
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Start server",
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
	Short: "Create default configuration file",
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
	Short: "List indexed URLs",
	Long:  `List indexed URLs - server should be stopped`,
	PreRun: func(_ *cobra.Command, _ []string) {
		initIndex()
	},
	Run: func(_ *cobra.Command, _ []string) {
		indexer.Iterate(func(d *indexer.Document) {
			fmt.Println(d.URL)
		})
	},
}

var importCmd = &cobra.Command{
	Use:   "import BROWSER_TYPE DB_PATH",
	Short: "Import Chrome or Firefox browsing history",
	Long: `
The Firefox URL database file is usually located at /home/[USER]/.mozilla/[PROFILE]/places.sqlite
The Chrome/Chromium URL database fiel is usually located at /home/[USER]/.config/chromium/Default/History
`,
	Args: cobra.ExactArgs(2),
	Run:  importHistory,
}

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Command line search interface",
	Long:  "",
	Run: func(_ *cobra.Command, _ []string) {
		if err := ui.SearchTUI(cfg); err != nil {
			exit(1, err.Error())
		}
	},
}

var indexCmd = &cobra.Command{
	Use:   "index URL",
	Short: "Index URL",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		setStrArg(cmd, "server-url", &cfg.Server.BaseURL)
		u := args[0]
		if err := indexURL(u); err != nil {
			exit(1, "Failed to index URL: "+err.Error())
		}
	},
}

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Reindex",
	Long:  `Recreate index - server should be stopped`,
	Run: func(cmd *cobra.Command, args []string) {
		err := indexer.Reindex(cfg.IndexPath(), cfg.FullPath("tmp_index.db"))
		if err != nil {
			exit(1, err.Error())
		}
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
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(reindexCmd)

	dcfg := config.CreateDefaultConfig()
	listenCmd.Flags().StringP("address", "a", dcfg.Server.Address, "Listen address")
	indexCmd.Flags().StringP("server-url", "u", dcfg.Server.BaseURL, "hister server URL")

	importCmd.Flags().IntP("min-visit", "m", 1, "only import URLs that were opened at least 'min-visit' times")

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

func yesNoPrompt(label string, def bool) bool {
	choices := "Y/n"
	if !def {
		choices = "y/N"
	}

	prompt := []byte(fmt.Sprintf("%s [%s] ", label, choices))
	r := bufio.NewReader(os.Stdin)
	var s string

	for {
		os.Stderr.Write(prompt)
		s, _ = r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		s = strings.ToLower(s)
		if s == "y" || s == "yes" {
			return true
		}
		if s == "n" || s == "no" {
			return false
		}
	}
}

//func stringPrompt(label string) string {
//	var s string
//	r := bufio.NewReader(os.Stdin)
//	for {
//		fmt.Fprint(os.Stderr, label+" ")
//		s, _ = r.ReadString('\n')
//		if s != "" {
//			break
//		}
//	}
//	return strings.TrimSpace(s)
//}
//
//func intPrompt(label string, def int64) int64 {
//	var s string
//	r := bufio.NewReader(os.Stdin)
//	prompt := fmt.Sprintf("%s [%d] ", label, def)
//	for {
//		fmt.Fprint(os.Stderr, prompt)
//		s, _ = r.ReadString('\n')
//		s = strings.TrimSpace(s)
//		if s == "" {
//			return def
//		}
//		i, err := strconv.ParseInt("12345", 10, 64)
//		if err != nil {
//			log.Error().Err(err).Msg("Invalid integer")
//		} else {
//			return i
//		}
//	}
//}
//
//func choicePrompt(label string, choices []string) string {
//	prompt := []byte(fmt.Sprintf("%s [%s,%s] ", label, strings.ToUpper(choices[0]), strings.Join(choices[1:], ",")))
//
//	r := bufio.NewReader(os.Stdin)
//	var s string
//
//	for {
//		os.Stderr.Write(prompt)
//		s, _ = r.ReadString('\n')
//		s = strings.TrimSpace(s)
//		if s == "" {
//			return choices[0]
//		}
//		s = strings.ToLower(s)
//		if slices.Contains(choices, s) {
//			return s
//		}
//	}
//}

func indexURL(u string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return errors.New(`failed to download file: ` + err.Error())
	}
	req.Header.Set("User-Agent", "Hister")
	r, err := client.Do(req)
	if err != nil {
		return errors.New(`failed to download file: ` + err.Error())
	}
	defer r.Body.Close()
	contentType := r.Header.Get("Content-type")
	if !strings.Contains(contentType, "html") {
		errors.New("invalid content type: " + contentType)
	}
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, r.Body)
	d := &indexer.Document{
		URL:  u,
		HTML: string(buf.Bytes()),
	}
	if err := d.Process(); err != nil {
		return errors.New(`failed to process document: ` + err.Error())
	}
	dj, err := json.Marshal(d)
	if err != nil {
		errors.New(`failed to encode document to JSON: ` + err.Error())
	}
	req, err = http.NewRequest("POST", cfg.BaseURL("/add"), bytes.NewBuffer(dj))
	req.Header.Set("content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return errors.New(`failed to send page to hister: ` + err.Error())
	}
	if resp.StatusCode != http.StatusCreated {
		return errors.New(fmt.Sprintf("failed to send page to hister: Invalid status code (%d)", resp.StatusCode))
	}
	defer resp.Body.Close()
	return nil
}

func importHistory(cmd *cobra.Command, args []string) {
	browser := args[0]
	if browser != "firefox" && browser != "chrome" {
		exit(1, "Invalid browser type it should be 'firefox' or 'chrome'")
	}
	dbFile := args[1]
	table := "urls"
	if browser == "firefox" {
		table = "moz_places"
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?immutable=1", dbFile))
	if err != nil {
		exit(1, "Failed to open database: "+err.Error())
	}
	defer db.Close()
	q := fmt.Sprintf("SELECT DISTINCT url FROM %s WHERE 1=1", table)
	if i, err := cmd.Flags().GetInt("min-visit"); err == nil && i > 1 {
		q += fmt.Sprintf(" AND visit_count >= %d", i)
	}

	cq := strings.Replace(q, "DISTINCT url", "DISTINCT count(url)", 1)
	row := db.QueryRow(cq)
	var count int
	if err := row.Scan(&count); err != nil {
		log.Debug().Str("query", cq).Msg("count query")
		exit(1, "Failed to execute database query: "+err.Error())
	}

	if count < 1 {
		exit(1, "No URLs found")
	}

	if !yesNoPrompt(fmt.Sprintf("%d URLs found. Start import", count), true) {
		return
	}

	q += " ORDER BY visit_count DESC"

	fmt.Println("IMPORTING")

	rows, err := db.Query(q)
	if err != nil {
		exit(1, "Failed to execute database query: "+err.Error())
	}
	defer rows.Close()
	i := 1
	for rows.Next() {
		var u string
		err = rows.Scan(&u)
		if err != nil {
			exit(1, "Failed to retreive URL: "+err.Error())
		}
		fmt.Printf("[%d/%d] %s\n", i, count, u)
		if err := indexURL(u); err != nil {
			log.Warn().Err(err).Msg("Failed to index URL")
		}
		i += 1
	}

	// TODO optional date filter
	//vf := "last_visit_time"
	//if browser == "firefox" {
	//	vf = "last_visit_date"
	//}
	//q += fmt.Sprintf(" AND %s >= datetime('now', 'localtime', '-1 month')", vf)
}

func main() {
	rootCmd.Execute()
}
