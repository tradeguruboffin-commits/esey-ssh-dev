package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const githubSSHURL = "https://github.com/settings/keys"

func main() {
	for {
		authenticated, shouldExit := checkAuth()
		if shouldExit {
			os.Exit(0)
		}
		if authenticated {
			break
		}
	}
}

func checkAuth() (bool, bool) {

	printInfo("Checking GitHub SSH Authentication...")
	runSpinner(2 * time.Second)

	cmd := exec.Command("ssh", "-T", "git@github.com")
	output, _ := cmd.CombinedOutput()
	outStr := string(output)

	// ✅ Success detection (GitHub returns exit code 1 on success)
	if strings.Contains(outStr, "successfully authenticated") ||
		(strings.Contains(outStr, "Hi ") && strings.Contains(outStr, "GitHub")) {

		printSuccess("Authenticated successfully with GitHub.")
		return true, true
	}

	printError("SSH Authentication failed.")

	// 🔍 Detect existing local key
	if detectLocalKey() {
		printInfo("Local SSH key found.")
	} else {
		printWarning("No local SSH key detected.")
	}

	// 🧠 Detect SSH agent
	if detectSSHAgent() {
		printInfo("SSH Agent is running.")
	} else {
		printWarning("SSH Agent not running or no key loaded.")
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print(colorCyan("Run 'sshx-key' to generate and copy SSH key? (y/n): "))
	input := readInput(reader)

	if input != "y" && input != "yes" {
		printWarning("Setup cancelled.")
		return false, true
	}

	fmt.Print(colorCyan("Enter your GitHub Email: "))
	email := readInput(reader)

	if email == "" {
		printError("Email cannot be empty.")
		return false, true
	}

	if !runKeySetup(email) {
		return false, true
	}

	showActionMessage()

	fmt.Print(colorCyan("\nOpen GitHub SSH settings in browser now? (y/n): "))
	openChoice := readInput(reader)

	if openChoice == "y" || openChoice == "yes" {
		openBrowser(githubSSHURL)
	} else {
		fmt.Println("👉", githubSSHURL)
	}

	fmt.Println("\nPress [Enter] after adding key to GitHub...")
	reader.ReadString('\n')

	printInfo("Re-verifying connection...")
	runSpinner(2 * time.Second)

	return false, false
}

////////////////////////////////////////////////////////////
// 🔄 Spinner
////////////////////////////////////////////////////////////

func runSpinner(duration time.Duration) {
	done := make(chan bool)
	go func() {
		chars := []string{"|", "/", "-", "\\"}
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Printf("\r%s Checking... ", chars[i%len(chars)])
				time.Sleep(120 * time.Millisecond)
				i++
			}
		}
	}()

	time.Sleep(duration)
	done <- true
	fmt.Print("\r")
}

////////////////////////////////////////////////////////////
// 🔍 Local SSH Key Detection
////////////////////////////////////////////////////////////

func detectLocalKey() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	keys := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
	}

	for _, key := range keys {
		if _, err := os.Stat(key); err == nil {
			return true
		}
	}

	return false
}

////////////////////////////////////////////////////////////
// 🧠 SSH Agent Detection
////////////////////////////////////////////////////////////

func detectSSHAgent() bool {
	cmd := exec.Command("ssh-add", "-l")
	err := cmd.Run()
	return err == nil
}

////////////////////////////////////////////////////////////
// 🔑 Run sshx-key
////////////////////////////////////////////////////////////

func runKeySetup(email string) bool {
	keyTool, err := exec.LookPath("sshx-key")
	if err != nil {
		printError("'sshx-key' not found in PATH.")
		return false
	}

	printInfo("Generating SSH key...")

	setupCmd := exec.Command(keyTool, email)
	setupCmd.Stdout = os.Stdout
	setupCmd.Stderr = os.Stderr
	setupCmd.Stdin = os.Stdin
	setupCmd.Env = os.Environ()

	if err := setupCmd.Run(); err != nil {
		printError(fmt.Sprintf("Error running sshx-key: %v", err))
		return false
	}

	return true
}

////////////////////////////////////////////////////////////
// 🌐 Browser
////////////////////////////////////////////////////////////

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		printWarning("Unsupported OS. Open manually:")
		fmt.Println(url)
		return
	}

	if err := cmd.Start(); err != nil {
		printError("Failed to open browser automatically.")
		fmt.Println(url)
		return
	}

	printSuccess("Browser opened.")
}

////////////////////////////////////////////////////////////
// 🎨 Colored Output Helpers
////////////////////////////////////////////////////////////

func colorRed(s string) string    { return "\033[31m" + s + "\033[0m" }
func colorGreen(s string) string  { return "\033[32m" + s + "\033[0m" }
func colorYellow(s string) string { return "\033[33m" + s + "\033[0m" }
func colorCyan(s string) string   { return "\033[36m" + s + "\033[0m" }

func printSuccess(msg string) { fmt.Println(colorGreen("✔ " + msg)) }
func printError(msg string)   { fmt.Println(colorRed("✖ " + msg)) }
func printWarning(msg string) { fmt.Println(colorYellow("⚠ " + msg)) }
func printInfo(msg string)    { fmt.Println(colorCyan("➜ " + msg)) }

////////////////////////////////////////////////////////////
// 📢 Message
////////////////////////////////////////////////////////////

func showActionMessage() {
	fmt.Println("\n--------------------------------------------------")
	printInfo("Action Required:")
	fmt.Println("1️⃣  Go to:", githubSSHURL)
	fmt.Println("2️⃣  Click 'New SSH Key'")
	fmt.Println("3️⃣  Paste and Save")
	fmt.Println("--------------------------------------------------")
}

////////////////////////////////////////////////////////////
// Input
////////////////////////////////////////////////////////////

func readInput(reader *bufio.Reader) string {
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(input))
}
