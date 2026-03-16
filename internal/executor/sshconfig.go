package executor

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// sshHostConfig holds resolved settings for an SSH host from ~/.ssh/config.
type sshHostConfig struct {
	Hostname      string
	User          string
	Port          int
	IdentityFile  string
	IdentityAgent string
}

// resolveSSHConfig reads ~/.ssh/config and resolves settings for the given host alias.
func resolveSSHConfig(alias string) *sshHostConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var result *sshHostConfig
	var current *sshHostConfig
	matched := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into keyword and argument
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			// Try tab separator
			parts = strings.SplitN(line, "\t", 2)
			if len(parts) < 2 {
				continue
			}
		}
		keyword := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		if keyword == "host" {
			// Check if any pattern in the Host line matches
			current = &sshHostConfig{}
			matched = false
			for _, pattern := range strings.Fields(value) {
				if matchHostPattern(pattern, alias) {
					matched = true
					result = current
					break
				}
			}
			continue
		}

		if !matched || current == nil {
			continue
		}

		switch keyword {
		case "hostname":
			if current.Hostname == "" {
				current.Hostname = value
			}
		case "user":
			if current.User == "" {
				current.User = value
			}
		case "port":
			if current.Port == 0 {
				if p, err := strconv.Atoi(value); err == nil {
					current.Port = p
				}
			}
		case "identityfile":
			if current.IdentityFile == "" {
				if strings.HasPrefix(value, "~/") {
					value = filepath.Join(home, value[2:])
				}
				current.IdentityFile = value
			}
		case "identityagent":
			if current.IdentityAgent == "" {
				// Strip quotes and expand ~
				value = strings.Trim(value, `"'`)
				if strings.HasPrefix(value, "~/") {
					value = filepath.Join(home, value[2:])
				}
				current.IdentityAgent = value
			}
		}
	}

	return result
}

// matchHostPattern matches a host alias against an SSH config Host pattern.
// Supports * wildcards and ? single-char wildcards.
func matchHostPattern(pattern, host string) bool {
	if pattern == "*" {
		return true
	}
	// Simple exact match (covers the vast majority of cases)
	if !strings.ContainsAny(pattern, "*?") {
		return pattern == host
	}
	// Basic wildcard matching
	return wildcardMatch(pattern, host)
}

func wildcardMatch(pattern, s string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Try matching rest of pattern at every position
			for i := 0; i <= len(s); i++ {
				if wildcardMatch(pattern[1:], s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		default:
			if len(s) == 0 || pattern[0] != s[0] {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		}
	}
	return len(s) == 0
}
