package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"time"

	"cryptotape/internal/listings"
	"cryptotape/internal/storage"
)

func main() {
	duration := flag.Duration("duration", 30*time.Second, "observation duration")
	flag.Parse()
	if *duration <= 0 || *duration > time.Hour {
		fmt.Fprintln(os.Stderr, "duration must be between 0 and 1 hour")
		os.Exit(1)
	}
	path, err := storage.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot locate app config")
		os.Exit(1)
	}
	config, err := listings.LoadConfig(filepath.Join(filepath.Dir(path), "listings-source.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if key := os.Getenv("NLF_KEY"); key != "" {
		config.Key = key
	}
	started := time.Now()
	received, eligible := 0, 0
	var age, delivery []int64
	client, err := listings.NewClient(config, listings.Callbacks{
		Ready: func(subscription listings.Subscription) {
			fmt.Printf("ready: plan=%s added_delay_ms=%d connect_ms=%d\n", subscription.Plan, *subscription.DelayMs, time.Since(started).Milliseconds())
		},
		Status: func(state, code string) { fmt.Printf("source: %s %s\n", state, code) },
		Event: func(event listings.Event, at time.Time) {
			received++
			assets, err := event.Listings(at)
			if err == nil {
				eligible += len(assets)
			}
			if event.Detected > 0 && len(age) < 10000 {
				age = append(age, at.Sub(time.UnixMicro(event.Detected)).Milliseconds())
				delivery = append(delivery, at.Sub(time.UnixMicro(event.Sent)).Milliseconds())
			}
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "configure a listing key in app settings or NLF_KEY")
		os.Exit(1)
	}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(root, *duration)
	defer cancel()
	err = client.Run(ctx)
	fmt.Printf("events=%d classified_non_binance_listing_assets=%d observed_ms=%d\n", received, eligible, time.Since(started).Milliseconds())
	printMetric("detected_to_received_ms_estimate", age)
	printMetric("sent_to_received_ms_estimate", delivery)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printMetric(name string, values []int64) {
	if len(values) == 0 {
		fmt.Printf("%s: no samples\n", name)
		return
	}
	slices.Sort(values)
	fmt.Printf("%s: n=%d p50=%d p99=%d max=%d\n", name, len(values), values[(len(values)-1)/2], values[(99*len(values)+99)/100-1], values[len(values)-1])
}
