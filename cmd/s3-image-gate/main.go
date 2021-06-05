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
	var debug, showVersion, viewIndex bool
	var bucket, keyPrefix string

	flag.BoolVar(&debug, "debug", false, "enable debug log")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&viewIndex, "index", false, "handle index")
	flag.StringVar(&bucket, "bucket", "", "upload dest bucket")
	flag.StringVar(&keyPrefix, "key-prefix", "", "upload dest key prefix")
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

	if debug {
		filter.MinLevel = logutils.LogLevel("debug")
	}
	log.SetOutput(filter)

	cfg := imagegate.DefaultConfig()
	cfg.ViewIndex = viewIndex
	cfg.Bucket = bucket
	cfg.KeyPrefix = keyPrefix
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
