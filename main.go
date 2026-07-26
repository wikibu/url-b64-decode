package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// decodeBase64UTF8 trims surrounding whitespace, decodes standard or
// URL-safe base64, and verifies the result is valid UTF-8.
func decodeBase64UTF8(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		var urlErr error
		data, urlErr = base64.URLEncoding.DecodeString(s)
		if urlErr != nil {
			return "", fmt.Errorf("invalid base64 (input starts with %q): %v", head(s, 100), err)
		}
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("decoded content is not valid UTF-8 (input starts with %q)", head(s, 100))
	}
	return string(data), nil
}

// head returns at most the first n bytes of s, for error messages.
func head(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// fetch GETs url, returning the response body on any 2xx status.
// Network errors, timeouts, and 5xx responses are retried up to
// `retries` times with `retryWait` between attempts; 4xx fails immediately.
func fetch(client *http.Client, url string, retries int, retryWait time.Duration) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryWait)
		}
		body, retryable, err := doGet(client, url)
		if err == nil {
			return body, nil
		}
		if !retryable {
			return "", err
		}
		lastErr = err
	}
	return "", fmt.Errorf("all %d attempts failed, last error: %v", retries+1, lastErr)
}

func doGet(client *http.Client, url string) (body string, retryable bool, err error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", true, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, err
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return string(data), false, nil
	case resp.StatusCode >= 500:
		return "", true, fmt.Errorf("server error: HTTP %d", resp.StatusCode)
	default:
		return "", false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
}

func main() {
	timeout := flag.Duration("timeout", 10*time.Second, "per-request timeout")
	retries := flag.Int("retries", 3, "retry count on network errors and 5xx")
	retryWait := flag.Duration("retry-wait", time.Second, "wait between retries")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: url-b64-decode [flags] <URL>\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	client := &http.Client{Timeout: *timeout}
	body, err := fetch(client, flag.Arg(0), *retries, *retryWait)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	decoded, err := decodeBase64UTF8(body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(decoded)
}
