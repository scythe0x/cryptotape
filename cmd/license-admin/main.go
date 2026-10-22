package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"cryptotape/internal/licenseserver"
)

func initialize(path string) error {
	if os.Getenv("MYTHRA_BOT_TOKEN") == "" {
		return fmt.Errorf("MYTHRA_BOT_TOKEN is required")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	seed := make([]byte, 32)
	encryption := make([]byte, 32)
	lookup := make([]byte, 32)
	for _, b := range [][]byte{seed, encryption, lookup} {
		if _, err = rand.Read(b); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(file, "MYTHRA_DATABASE=/var/lib/mythra-licensing/licenses.sqlite\nMYTHRA_SIGNING_SEED=%s\nMYTHRA_ENCRYPTION_KEY=%s\nMYTHRA_LOOKUP_KEY=%s\nMYTHRA_BOT_TOKEN=%s\n", base64.StdEncoding.EncodeToString(seed), base64.StdEncoding.EncodeToString(encryption), base64.StdEncoding.EncodeToString(lookup), os.Getenv("MYTHRA_BOT_TOKEN"))
	if err != nil {
		return err
	}
	fmt.Println(base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)))
	return file.Sync()
}
func main() {
	action := flag.String("action", "", "init, extend, revoke or backup")
	telegram := flag.Int64("telegram-id", 0, "telegram user id")
	days := flag.Int("days", 30, "days to add")
	output := flag.String("output", "", "configuration or backup path")
	flag.Parse()
	if *action == "init" {
		if err := initialize(*output); err != nil {
			log.Fatal(err)
		}
		return
	}
	c, err := licenseserver.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	s, err := licenseserver.Open(c.Database, c.Encryption, c.Lookup)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	switch *action {
	case "extend":
		err = s.Extend(context.Background(), *telegram, *days, time.Now())
	case "revoke":
		err = s.Revoke(context.Background(), *telegram)
	case "backup":
		err = s.Backup(context.Background(), *output)
	default:
		log.Fatal("expected -action init, extend, revoke or backup")
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("completed")
}
