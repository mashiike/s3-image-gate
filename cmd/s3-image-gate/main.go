package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hashicorp/logutils"
	imagegate "github.com/mashiike/s3-image-gate"
)

var version = "0.0.0"

var filter = &logutils.LevelFilter{
	Levels:   []logutils.LogLevel{"debug", "info", "warn", "error"},
	MinLevel: logutils.LogLevel("info"),
	Writer:   os.Stderr,
}

func main() {
	var debug, showHelp, showVersion bool
	cfg := imagegate.DefaultConfig()

	flag.BoolVar(&debug, "debug", false, "enable debug log")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showHelp, "help", false, "show help")
	flag.BoolVar(&cfg.ViewIndex, "index", cfg.ViewIndex, "handle index")
	flag.StringVar(&cfg.Bucket, "bucket", cfg.Bucket, "upload dest bucket (required)")
	flag.StringVar(&cfg.KeyPrefix, "key-prefix", cfg.KeyPrefix, "upload dest key prefix")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "http port")
	flag.VisitAll(func(f *flag.Flag) {
		if s := os.Getenv(strings.ToUpper("S3_IMAGE_GATE_" + strings.ReplaceAll(f.Name, "-", "_"))); s != "" {
			f.Value.Set(s)
		}
	})
	flag.Parse()

	if showVersion {
		fmt.Println("s3-image-gate version", version)
		return
	}
	if showHelp {
		fmt.Println("Usage of s3-image-gate")
		flag.PrintDefaults()
		return
	}

	if debug {
		filter.MinLevel = logutils.LogLevel("debug")
	}
	log.SetOutput(filter)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM|syscall.SIGHUP|syscall.SIGINT)
	defer stop()

	err := imagegate.Run(ctx, cfg)
	switch err {
	case nil:
		log.Println("[info] success.")
	case context.Canceled:
		log.Panicln("[info] signal catch.")
	default:
		log.Fatalf("[error] %v\n", err)
	}
}
