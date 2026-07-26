// Command uptime-ping is a self-hosted status page and uptime monitor shipped
// as a single Go binary. It polls HTTP/TCP targets from a YAML config, serves
// a live status page with per-service uptime and latency, exposes Prometheus
// metrics, and fires a webhook on up/down transitions.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/moveeeax/uptime-ping/internal/alert"
	"github.com/moveeeax/uptime-ping/internal/config"
	"github.com/moveeeax/uptime-ping/internal/monitor"
	"github.com/moveeeax/uptime-ping/internal/web"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("uptime-ping", flag.ContinueOnError)
	var (
		cfgPath     = fs.String("config", "checks.yaml", "path to YAML config file")
		listen      = fs.String("listen", "", "override listen address (e.g. :8080)")
		once        = fs.Bool("once", false, "run every check once, print a report, exit non-zero if any is down")
		validate    = fs.Bool("validate", false, "validate the config file and exit")
		showVersion = fs.Bool("version", false, "print version and exit")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	logger := log.New(os.Stderr, "", log.LstdFlags)

	if *showVersion {
		fmt.Printf("uptime-ping %s\n", version)
		return 0
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Printf("config error: %v", err)
		return 1
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *validate {
		fmt.Printf("config OK: %d target(s)\n", len(cfg.Targets))
		return 0
	}

	prober := monitor.NewNetProber()
	notifier := alert.NewWebhook(cfg.Webhook.URL, logger)
	mon := monitor.New(cfg, prober, notifier)

	if *once {
		return runOnce(mon)
	}
	return serve(cfg, mon, logger)
}

// runOnce probes every target a single time and prints a report table.
func runOnce(mon *monitor.Monitor) int {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	allUp := mon.RunOnce(ctx)

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SERVICE\tSTATE\tLATENCY\tDETAIL")
	for _, st := range mon.Snapshot() {
		detail := st.LastError
		if detail == "" {
			detail = "ok"
		}
		fmt.Fprintf(tw, "%s\t%s\t%.0fms\t%s\n", st.Name, st.State, st.LastLatMS, detail)
	}
	_ = tw.Flush()

	if allUp {
		return 0
	}
	return 1
}

// serve runs the scheduler plus the HTTP status server until interrupted.
func serve(cfg *config.Config, mon *monitor.Monitor, logger *log.Logger) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go mon.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           web.New(mon).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	logger.Printf("uptime-ping %s monitoring %d target(s), status page on %s", version, len(cfg.Targets), cfg.Listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Printf("server error: %v", err)
		return 1
	}
	logger.Printf("shutdown complete")
	return 0
}
