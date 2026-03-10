package auth

import (
	"fmt"
	"os/exec"
	"strings"
)

// SendMagicLinkEmail sends a magic link login email using the Himalaya CLI.
// Returns an error if himalaya is not installed or sending fails.
func SendMagicLinkEmail(to, from, magicURL, domain string) error {
	// Check if himalaya binary exists
	himalayaPath, err := exec.LookPath("himalaya")
	if err != nil {
		return fmt.Errorf("himalaya not installed: %w", err)
	}

	subject := fmt.Sprintf("Your login link for %s", domain)
	body := fmt.Sprintf("Click the link below to log in:\n\n%s\n\nThis link expires in 15 minutes and can only be used once.\n\nIf you didn't request this link, you can safely ignore this email.\n", magicURL)

	// Build the email in RFC 2822 format
	email := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, to, subject, body)

	// Use himalaya to send
	cmd := exec.Command(himalayaPath, "send")
	cmd.Stdin = strings.NewReader(email)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("himalaya send failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// IsHimalayaAvailable checks if the himalaya binary is installed.
func IsHimalayaAvailable() bool {
	_, err := exec.LookPath("himalaya")
	return err == nil
}
