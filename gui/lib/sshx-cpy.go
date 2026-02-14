package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultPort = "22"

func main() {
	if len(os.Args) != 2 {
		printError("Usage: sshx-cpy user@host[:port]")
		os.Exit(1)
	}

	userHost, port, err := parseTarget(os.Args[1])
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	keyPath, err := detectPrivateKey()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}
	printInfo("Using private key: " + keyPath)

	pubKey, err := getPublicKey(keyPath)
	if err != nil {
		printError("Failed to extract public key: " + err.Error())
		os.Exit(1)
	}

	printInfo(fmt.Sprintf("Installing key on %s (Port: %s)...", userHost, port))
	if err := installKey(userHost, port, pubKey); err != nil {
		printError("Failed to install key: " + err.Error())
		os.Exit(1)
	}

	printInfo("Verifying passwordless login...")
	if verifyLogin(userHost, port) {
		printSuccess("Passwordless SSH enabled successfully!")
		fmt.Printf("\nTest with:\n  ssh -p %s %s\n", port, userHost)
	} else {
		printError("Verification failed. Password may still be required.")
		os.Exit(1)
	}
}

////////////////////////////////////////////////////////////
// Target Parsing (IPv4 / IPv6 safe)
////////////////////////////////////////////////////////////

func parseTarget(input string) (string, string, error) {

	if !strings.Contains(input, "@") {
		return "", "", errors.New("Invalid format. Expected user@host[:port]")
	}

	userHost := input
	port := defaultPort

	// Handle IPv6 [host]:port
	if strings.Contains(input, "]") {
		host, p, err := net.SplitHostPort(input)
		if err == nil {
			return host, p, nil
		}
		return input, defaultPort, nil
	}

	// Handle normal host:port
	if strings.Count(input, ":") == 1 {
		host, p, err := net.SplitHostPort(input)
		if err == nil {
			return host, p, nil
		}
	}

	return userHost, port, nil
}

////////////////////////////////////////////////////////////
// Detect Private Key
////////////////////////////////////////////////////////////

func detectPrivateKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	sshDir := filepath.Join(home, ".ssh")

	candidates := []string{
		"id_ed25519",
		"id_rsa",
	}

	for _, name := range candidates {
		path := filepath.Join(sshDir, name)
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path, nil
		}
	}

	return "", errors.New("No private SSH key found in ~/.ssh/")
}

////////////////////////////////////////////////////////////
// Extract Public Key
////////////////////////////////////////////////////////////

func getPublicKey(privatePath string) (string, error) {

	pubPath := privatePath + ".pub"

	if data, err := os.ReadFile(pubPath); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	cmd := exec.Command("ssh-keygen", "-y", "-f", privatePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

////////////////////////////////////////////////////////////
// Install Key (Injection Safe)
////////////////////////////////////////////////////////////

func installKey(userHost, port, pubKey string) error {

	remoteCmd := `
mkdir -p ~/.ssh &&
chmod 700 ~/.ssh &&
touch ~/.ssh/authorized_keys &&
chmod 600 ~/.ssh/authorized_keys &&
grep -qxF "$KEY" ~/.ssh/authorized_keys || echo "$KEY" >> ~/.ssh/authorized_keys
`

	cmd := exec.Command(
		"ssh",
		"-p", port,
		"-o", "StrictHostKeyChecking=accept-new",
		userHost,
		"KEY='"+escapeForShell(pubKey)+"' bash -c '"+remoteCmd+"'",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

////////////////////////////////////////////////////////////
// Verify Passwordless
////////////////////////////////////////////////////////////

func verifyLogin(userHost, port string) bool {

	cmd := exec.Command(
		"ssh",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-p", port,
		userHost,
		"exit",
	)

	return cmd.Run() == nil
}

////////////////////////////////////////////////////////////
// Escape Shell Single Quotes Safely
////////////////////////////////////////////////////////////

func escapeForShell(s string) string {
	return strings.ReplaceAll(s, `'`, `'\''`)
}

////////////////////////////////////////////////////////////
// Colored Output
////////////////////////////////////////////////////////////

func colorRed(s string) string   { return "\033[31m" + s + "\033[0m" }
func colorGreen(s string) string { return "\033[32m" + s + "\033[0m" }
func colorCyan(s string) string  { return "\033[36m" + s + "\033[0m" }

func printSuccess(msg string) { fmt.Println(colorGreen("✔ " + msg)) }
func printError(msg string)   { fmt.Println(colorRed("✖ " + msg)) }
func printInfo(msg string)    { fmt.Println(colorCyan("➜ " + msg)) }
