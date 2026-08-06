package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/optech/protocol-bridge/internal/security"
)

func main() {
	var value string
	flag.StringVar(&value, "value", "", "Value to encrypt (reads from stdin if not provided)")
	flag.Parse()

	if value == "" {
		fmt.Fprint(os.Stderr, "Enter value to encrypt: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			value = strings.TrimSpace(scanner.Text())
		}
	}

	if value == "" {
		fmt.Fprintln(os.Stderr, "error: no value provided")
		os.Exit(1)
	}

	masterKey, err := security.GetMasterKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting master key: %v\n", err)
		os.Exit(1)
	}

	encrypted, err := security.EncryptValue(value, masterKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error encrypting value: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(encrypted)
}
