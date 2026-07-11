package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

// main runs the CLI and reports user-facing errors with a non-zero exit code.
func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "uc:", err)
		os.Exit(1)
	}
}

// run parses arguments, resolves a utility, and writes that utility's result.
func run(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return runInteractive(in, out)
	}
	if isHelp(args[0]) {
		fmt.Fprint(out, strings.TrimPrefix(usage, ""))
		return nil
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintln(out, version)
		return nil
	}

	command := canonicalCommand(args[0])
	commandArgs := args[1:]
	if command == "" {
		return fmt.Errorf("unknown command %q; run 'uc --help' for commands", args[0])
	}

	input, err := readInput(command, commandArgs, in)
	if err != nil {
		return err
	}
	result, err := execute(command, input)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, result)
	return nil
}

const usage = `uc — command-line CS utilities

Usage:
  uc                        Open the interactive terminal UI
  uc <command> [input]       Run an explicit utility (or pipe input through stdin)

Tools:
  b64-encode      Encode text as Unicode-safe Base64.
                  uc b64-encode "hello"
  b64-decode      Decode padded or unpadded Base64 text.
                  uc b64-decode aGVsbG8
  b64url-encode   Encode URL-safe Base64 without padding.
                  uc b64url-encode "hello"
  b64url-decode   Decode URL-safe Base64 text.
                  uc b64url-decode aGVsbG8
  url-encode      Percent-encode a URL component using RFC 3986 rules.
                  uc url-encode "hello world"
  url-decode      Decode a percent-encoded URL component.
                  uc url-decode hello%20world
  html-encode     Escape HTML special characters.
                  uc html-encode '<a href="/">'
  html-decode     Decode common HTML entities.
                  uc html-decode '&lt;hello&gt;'
  jwt             Decode a JWT header and payload; does not verify its signature.
                  uc jwt '<header>.<payload>.<signature>'
  saml            Decode Base64 SAML XML, including Redirect-binding deflate.
                  uc saml '<base64-saml-message>'
  json-pretty     Validate and indent JSON.
                  uc json-pretty '{"enabled":true}'
  json-minify     Validate JSON and remove insignificant whitespace.
                  uc json-minify '{ "enabled": true }'
  xml-pretty      Validate and indent XML.
                  uc xml-pretty '<root><item>one</item></root>'
  xml-minify      Validate XML and remove whitespace-only text nodes.
                  uc xml-minify '<root>  <item>one</item> </root>'
  hash            Generate MD5, SHA-1, SHA-256, and SHA-512 hashes.
                  uc hash "hello"
  uuid            Generate a version 4 UUID; supports --count up to 100.
                  uc uuid --count 3
  password        Generate a random password; supports --length and --symbols.
                  uc password --length 24 --symbols
  timestamp       Convert a Unix timestamp or an RFC 3339 date.
                  uc timestamp 1710000000
`

// isHelp reports whether an argument requests the built-in help text.
func isHelp(s string) bool { return s == "-h" || s == "--help" || s == "help" }

// readInput returns positional text or piped input, while allowing generators to use defaults.
func readInput(command string, args []string, in io.Reader) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	// These generators have useful defaults and deliberately need no text input.
	if command == "uuid" || command == "password" {
		return "", nil
	}
	if file, ok := in.(*os.File); ok {
		stat, err := file.Stat()
		if err == nil && stat.Mode()&os.ModeCharDevice != 0 {
			return "", errors.New("input is required (pass text or pipe stdin)")
		}
	}
	b, err := io.ReadAll(in)
	return strings.TrimSuffix(string(b), "\n"), err
}

// canonicalCommand resolves a public command name or alias to its internal command.
func canonicalCommand(s string) string {
	switch strings.ToLower(s) {
	case "b64", "base64", "base64-encode", "b64-encode":
		return "b64-encode"
	case "base64-decode", "b64-decode":
		return "b64-decode"
	case "base64url-encode", "b64url-encode":
		return "b64url-encode"
	case "base64url-decode", "b64url-decode":
		return "b64url-decode"
	case "url-encode", "urlencode":
		return "url-encode"
	case "url-decode", "urldecode":
		return "url-decode"
	case "html-encode", "htmlencode":
		return "html-encode"
	case "html-decode", "htmldecode":
		return "html-decode"
	case "json-pretty", "json-beautify":
		return "json-pretty"
	case "json-minify":
		return "json-minify"
	case "xml-pretty", "xml-beautify":
		return "xml-pretty"
	case "xml-minify":
		return "xml-minify"
	case "jwt", "jwt-decode":
		return "jwt"
	case "saml", "saml-decode":
		return "saml"
	case "hash", "uuid", "password", "timestamp":
		return strings.ToLower(s)
	default:
		return ""
	}
}

// execute runs a canonical command against its string input.
func execute(command, input string) (string, error) {
	switch command {
	case "b64-encode":
		return base64.StdEncoding.EncodeToString([]byte(input)), nil
	case "b64-decode":
		return decodeBase64(input, false)
	case "b64url-encode":
		return base64.RawURLEncoding.EncodeToString([]byte(input)), nil
	case "b64url-decode":
		return decodeBase64(input, true)
	case "url-encode":
		return encodeRFC3986(input), nil
	case "url-decode":
		return url.PathUnescape(input)
	case "html-encode":
		return encodeHTML(input), nil
	case "html-decode":
		return decodeHTML(input), nil
	case "json-pretty":
		return prettyJSON(input)
	case "json-minify":
		return minifyJSON(input)
	case "xml-pretty":
		return prettyXML(input)
	case "xml-minify":
		return minifyXML(input)
	case "jwt":
		return decodeJWT(input)
	case "saml":
		return decodeSAML(input)
	case "hash":
		return hashes(input), nil
	case "uuid":
		return generateUUID(input)
	case "password":
		return password(input)
	case "timestamp":
		return convertTimestamp(input)
	}
	return "", fmt.Errorf("unknown command %q", command)
}

// decodeBase64 decodes standard or URL-safe Base64, accepting padded and raw forms.
func decodeBase64(value string, rawURL bool) (string, error) {
	value = strings.TrimSpace(value)
	var b []byte
	var err error
	if rawURL {
		b, err = base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			b, err = base64.URLEncoding.DecodeString(value)
		}
	} else {
		b, err = base64.StdEncoding.DecodeString(value)
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(value)
		}
	}
	if err != nil {
		return "", fmt.Errorf("invalid Base64: %w", err)
	}
	return string(b), nil
}

// encodeRFC3986 percent-encodes text using the RFC 3986 unreserved character set.
func encodeRFC3986(s string) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for _, b := range []byte(s) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.ContainsRune("-._~", rune(b)) {
			out.WriteByte(b)
		} else {
			out.WriteByte('%')
			out.WriteByte(hex[b>>4])
			out.WriteByte(hex[b&15])
		}
	}
	return out.String()
}

// encodeHTML escapes the five HTML characters that have special markup meaning.
func encodeHTML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;").Replace(s)
}

// decodeHTML restores the HTML entities produced by encodeHTML.
func decodeHTML(s string) string {
	return strings.NewReplacer("&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&apos;", "'", "&amp;", "&").Replace(s)
}

// prettyJSON validates and indents JSON with two spaces.
func prettyJSON(input string) (string, error) {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(input), "", "  "); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	return out.String(), nil
}

// minifyJSON validates and removes insignificant whitespace from JSON.
func minifyJSON(input string) (string, error) {
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(input)); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	return out.String(), nil
}

// prettyXML validates and indents an XML document with two spaces.
func prettyXML(input string) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(input))
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	enc.Indent("", "  ")
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("invalid XML: %w", err)
		}
		if err := enc.EncodeToken(tok); err != nil {
			return "", err
		}
	}
	if err := enc.Flush(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// minifyXML validates XML and removes whitespace-only text nodes.
func minifyXML(input string) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(input))
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("invalid XML: %w", err)
		}
		if c, ok := tok.(xml.CharData); ok && strings.TrimSpace(string(c)) == "" {
			continue
		}
		if err := enc.EncodeToken(tok); err != nil {
			return "", err
		}
	}
	if err := enc.Flush(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// decodeJWT displays decoded JSON header and payload without verifying its signature.
func decodeJWT(token string) (string, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return "", errors.New("invalid JWT: expected three dot-separated parts")
	}
	h, err := decodeBase64(parts[0], true)
	if err != nil {
		return "", fmt.Errorf("JWT header: %w", err)
	}
	p, err := decodeBase64(parts[1], true)
	if err != nil {
		return "", fmt.Errorf("JWT payload: %w", err)
	}
	h, err = prettyJSON(h)
	if err != nil {
		return "", err
	}
	p, err = prettyJSON(p)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Header:\n%s\n\nPayload:\n%s\n\nSignature:\n%s", h, p, parts[2]), nil
}

// decodeSAML handles URL-encoded plain or raw-deflate Base64 SAML XML and formats it.
func decodeSAML(value string) (string, error) {
	value, _ = url.QueryUnescape(strings.TrimSpace(value))
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(value)
		if err != nil {
			return "", fmt.Errorf("invalid SAML Base64: %w", err)
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(string(b)), "<") {
		reader := flate.NewReader(bytes.NewReader(b))
		decompressed, inflateErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if inflateErr != nil || closeErr != nil {
			return "", errors.New("decoded SAML is neither XML nor a raw-deflate SAML request")
		}
		b = decompressed
	}
	return prettyXML(string(b))
}

// hashes calculates the MD5, SHA-1, SHA-256, and SHA-512 digests for text.
func hashes(s string) string {
	a := md5.Sum([]byte(s))
	b := sha1.Sum([]byte(s))
	c := sha256.Sum256([]byte(s))
	d := sha512.Sum512([]byte(s))
	return fmt.Sprintf("MD5     %x\nSHA-1   %x\nSHA-256 %x\nSHA-512 %x", a, b, c, d)
}

// newUUID returns count cryptographically random RFC 4122 version 4 UUIDs.
func newUUID(count int) (string, error) {
	var values []string
	for i := 0; i < count; i++ {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		values = append(values, fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]))
	}
	return strings.Join(values, "\n"), nil
}

// generateUUID parses UUID generator flags and enforces the bulk-generation limit.
func generateUUID(input string) (string, error) {
	fs := flag.NewFlagSet("uuid", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	count := fs.Int("count", 1, "number of UUIDs")
	if err := fs.Parse(strings.Fields(input)); err != nil {
		return "", err
	}
	if *count < 1 || *count > 100 {
		return "", errors.New("count must be 1–100")
	}
	return newUUID(*count)
}

// password generates a cryptographically random password from selected character classes.
func password(input string) (string, error) {
	fs := flag.NewFlagSet("password", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	length := fs.Int("length", 20, "password length")
	upper := fs.Bool("upper", true, "include uppercase")
	lower := fs.Bool("lower", true, "include lowercase")
	numbers := fs.Bool("numbers", true, "include digits")
	symbols := fs.Bool("symbols", false, "include symbols")
	if err := fs.Parse(strings.Fields(input)); err != nil {
		return "", err
	}
	chars := ""
	if *upper {
		chars += "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}
	if *lower {
		chars += "abcdefghijklmnopqrstuvwxyz"
	}
	if *numbers {
		chars += "0123456789"
	}
	if *symbols {
		chars += "!@#$%^&*()_+-=[]{}|;:,.<>?"
	}
	if chars == "" {
		return "", errors.New("select at least one character class")
	}
	if *length < 1 || *length > 1024 {
		return "", errors.New("length must be 1–1024")
	}
	b := make([]byte, *length)
	for i := range b {
		n := make([]byte, 1)
		if _, err := rand.Read(n); err != nil {
			return "", err
		}
		b[i] = chars[int(n[0])%len(chars)]
	}
	return string(b), nil
}

// convertTimestamp converts a Unix timestamp or RFC 3339 date to common representations.
func convertTimestamp(s string) (string, error) {
	s = strings.TrimSpace(s)
	var t time.Time
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 100000000000 {
			t = time.UnixMilli(n)
		} else {
			t = time.Unix(n, 0)
		}
	} else {
		var e error
		t, e = time.Parse(time.RFC3339, s)
		if e != nil {
			return "", errors.New("pass a Unix timestamp or RFC3339 date")
		}
	}
	return fmt.Sprintf("UTC: %s\nLocal: %s\nISO 8601: %s\nUnix (seconds): %d\nUnix (ms): %d", t.UTC().Format(time.RFC1123), t.Local().Format(time.RFC1123), t.UTC().Format(time.RFC3339Nano), t.Unix(), t.UnixMilli()), nil
}

// Kept referenced so go vet recognizes the import as intentional in older toolchains.
var _ = hex.EncodeToString
var _ = bufio.NewReader
