package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	smartSSHCleanup()
}

func smartSSHCleanup() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Failed to detect home directory:", err)
	}

	sshPath := filepath.Join(home, ".ssh")

	fmt.Println("Starting Professional SSH Environment Cleanup...")
	fmt.Println("--------------------------------------------------")

	// 1️⃣ Protected files
	protected := map[string]bool{
		filepath.Join(sshPath, "id_ed25519"):     true,
		filepath.Join(sshPath, "id_ed25519.pub"): true,
		filepath.Join(sshPath, "authorized_keys"): true,
	}

	// 2️⃣ Cleanup patterns
	patterns := []string{"*.old", "*.tmp", "*.bak", "known_hosts"}

	filesToClean := []string{}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(sshPath, pattern))
		filesToClean = append(filesToClean, matches...)
	}

	cleaned := 0

	// 3️⃣ Delete phase
	for _, file := range filesToClean {

		if protected[file] {
			continue
		}

		if _, err := os.Stat(file); err == nil {
			err := os.Remove(file)
			if err == nil {
				fmt.Println("Removed:", filepath.Base(file))
				cleaned++
			} else {
				fmt.Println("Error removing:", filepath.Base(file), "-", err)
			}
		}
	}

	// 4️⃣ Identity verification
	fmt.Println("\nVerifying SSH Identity Keys:")

	for key := range protected {
		if _, err := os.Stat(key); err == nil {
			fmt.Println("[SAFE]", filepath.Base(key), "is preserved.")
		} else {
			fmt.Println("[INFO]", filepath.Base(key), "not present (Skipping).")
		}
	}

	// 5️⃣ Reset known_hosts
	hostsPath := filepath.Join(sshPath, "known_hosts")

	file, err := os.Create(hostsPath)
	if err == nil {
		file.Close()
		os.Chmod(hostsPath, 0600)
		fmt.Println("\nknown_hosts has been securely reset.")
	} else {
		fmt.Println("Failed to reset known_hosts:", err)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("Cleanup Complete! Total junk files removed:", cleaned)
	fmt.Println("Your SSH directory is now clean and optimized.")
}
