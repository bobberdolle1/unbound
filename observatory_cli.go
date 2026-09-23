package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"unbound/engine/observatory"
)

func runObservation(rawURL, protocol, family string, overallTimeout time.Duration, save, jsonOutput bool) {
	transport, err := parseObservationTransport(protocol)
	if err != nil {
		log.Fatalf("%v", err)
	}
	addressFamily, err := parseObservationFamily(family)
	if err != nil {
		log.Fatalf("%v", err)
	}
	timeouts := observatory.DefaultTimeouts()
	if overallTimeout > 0 {
		timeouts.Overall = overallTimeout
	}
	result, err := observe(context.Background(), rawURL, observatory.Options{
		Transport:     transport,
		AddressFamily: addressFamily,
		Timeouts:      timeouts,
	})
	if err != nil {
		log.Fatalf("observe %q: %v", rawURL, err)
	}
	var savedPath string
	if save {
		savedPath, err = observatory.SaveObservation(result)
		if err != nil {
			log.Fatalf("save observation: %v", err)
		}
	}
	writeObservation(result, jsonOutput)
	if savedPath != "" && !jsonOutput {
		fmt.Printf("Saved redacted observation: %s\n", savedPath)
	}
}
func parseObservationTransport(value string) (observatory.Transport, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "tcp":
		return observatory.TransportTCP, nil
	case "quic":
		return observatory.TransportQUIC, nil
	default:
		return "", fmt.Errorf("unsupported observation protocol %q", value)
	}
}

func parseObservationFamily(value string) (observatory.AddressFamily, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "any":
		return observatory.AddressFamilyAny, nil
	case "4", "ipv4":
		return observatory.AddressFamilyIPv4, nil
	case "6", "ipv6":
		return observatory.AddressFamilyIPv6, nil
	default:
		return "", fmt.Errorf("unsupported observation address family %q", value)
	}
}

func writeObservation(result observatory.ObservationResult, jsonOutput bool) {
	if jsonOutput {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			log.Fatalf("encode observation: %v", err)
		}
		return
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("format observation: %v", err)
	}
	fmt.Println(string(data))
}

func observe(ctx context.Context, rawURL string, options observatory.Options) (observatory.ObservationResult, error) {
	return observatory.NewDirectTCPHTTPSObserver().Observe(ctx, rawURL, options)
}
