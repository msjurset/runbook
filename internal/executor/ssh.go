package executor

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/msjurset/runbook/internal/credentials"
	"github.com/msjurset/runbook/internal/runbook"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHExecutor runs a command on a remote host via SSH.
type SSHExecutor struct {
	Step *runbook.SSHStep
}

func (e *SSHExecutor) Execute(ctx context.Context, vars map[string]string, stdout, stderr io.Writer) (*ExecResult, error) {
	command, err := runbook.Expand(e.Step.Command, vars)
	if err != nil {
		return nil, fmt.Errorf("expanding command: %w", err)
	}

	alias, err := runbook.Expand(e.Step.Host, vars)
	if err != nil {
		return nil, fmt.Errorf("expanding host: %w", err)
	}

	host := alias // actual hostname to connect to (may differ from alias)
	user := e.Step.User
	port := e.Step.Port
	keyFile := e.Step.KeyFile
	agentSock := "" // custom agent socket (e.g., 1Password)

	// Resolve SSH config for host alias
	if sshCfg := resolveSSHConfig(alias); sshCfg != nil {
		if sshCfg.Hostname != "" {
			host = sshCfg.Hostname
		}
		if user == "" && sshCfg.User != "" {
			user = sshCfg.User
		}
		if port == 0 && sshCfg.Port != 0 {
			port = sshCfg.Port
		}
		if keyFile == "" && sshCfg.IdentityFile != "" {
			keyFile = sshCfg.IdentityFile
		}
		if sshCfg.IdentityAgent != "" {
			agentSock = sshCfg.IdentityAgent
		}
	}

	if user == "" {
		user = os.Getenv("USER")
	}
	expandedUser, err := runbook.Expand(user, vars)
	if err != nil {
		return nil, fmt.Errorf("expanding user: %w", err)
	}
	user = expandedUser

	if port == 0 {
		port = 22
	}

	authMethods, err := e.authMethods(keyFile, agentSock)
	if err != nil {
		return nil, fmt.Errorf("ssh auth: %w", err)
	}

	hostKeyCallback, err := knownHostsCallback()
	if err != nil {
		// Fall back to insecure if known_hosts isn't available
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Dial with context support
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}

	// Wrap in context cancellation
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	// Try known_hosts with the resolved hostname first, then the alias
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil && alias != host {
		// Retry with alias address for known_hosts matching
		conn.Close()
		conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return nil, fmt.Errorf("connecting to %s: %w", addr, dialErr)
		}
		go func() {
			<-ctx.Done()
			conn.Close()
		}()
		aliasAddr := net.JoinHostPort(alias, strconv.Itoa(port))
		sshConn, chans, reqs, err = ssh.NewClientConn(conn, aliasAddr, config)
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake with %s: %w", addr, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("creating ssh session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	if stdout != nil {
		session.Stdout = io.MultiWriter(&stdoutBuf, stdout)
	} else {
		session.Stdout = &stdoutBuf
	}
	if stderr != nil {
		session.Stderr = io.MultiWriter(&stderrBuf, stderr)
	} else {
		session.Stderr = &stderrBuf
	}

	err = session.Run(command)
	result := &ExecResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		}
		return result, fmt.Errorf("ssh command failed: %w", err)
	}
	return result, nil
}

func (e *SSHExecutor) authMethods(keyFile, agentSock string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try cached SSH key from keychain (pre-resolved via `runbook auth`)
	cacheKey := "ssh_key_" + e.Step.Host
	if cached, _ := credentials.Load(cacheKey); cached != "" {
		if signer, err := parsePrivateKey([]byte(cached)); err == nil {
			methods = append(methods, ssh.PublicKeys(signer))
			return methods, nil // Cached key takes priority — no agent needed
		}
	}

	// Try key file — if it's an op:// ref, try keychain cache first
	if keyFile != "" {
		if credentials.IsOpRef(keyFile) {
			// Try keychain cache for op:// key references
			stepCacheKey := "ssh_key_" + e.Step.Host
			if cached, _ := credentials.Load(stepCacheKey); cached != "" {
				if signer, err := parsePrivateKey([]byte(cached)); err == nil {
					methods = append(methods, ssh.PublicKeys(signer))
					return methods, nil
				}
			}
			return nil, fmt.Errorf("ssh key %s not cached — run `runbook auth` first", keyFile)
		}

		if strings.HasPrefix(keyFile, "~/") {
			home, _ := os.UserHomeDir()
			keyFile = filepath.Join(home, keyFile[2:])
		}
		key, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("reading key file %s: %w", keyFile, err)
		}
		signer, err := parsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parsing key file %s: %w", keyFile, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	// Try SSH agent — use IdentityAgent from config, then SSH_AUTH_SOCK
	if e.Step.AgentAuth && len(methods) == 0 {
		sock := agentSock
		if sock == "" {
			sock = os.Getenv("SSH_AUTH_SOCK")
		}
		if sock != "" {
			conn, err := net.Dial("unix", sock)
			if err == nil {
				agentClient := agent.NewClient(conn)
				methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
			}
		}
	}

	// Try default key locations if nothing else configured
	if len(methods) == 0 {
		home, _ := os.UserHomeDir()
		for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
			keyPath := filepath.Join(home, ".ssh", name)
			key, err := os.ReadFile(keyPath)
			if err != nil {
				continue
			}
			signer, err := parsePrivateKey(key)
			if err != nil {
				continue
			}
			methods = append(methods, ssh.PublicKeys(signer))
			break
		}
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no ssh authentication methods available (set agent_auth, key_file, or have ~/.ssh/id_* keys)")
	}
	return methods, nil
}

// parsePrivateKey handles both OpenSSH and PKCS#8 (1Password export) formats.
func parsePrivateKey(keyData []byte) (ssh.Signer, error) {
	signer, err := ssh.ParsePrivateKey(keyData)
	if err == nil {
		return signer, nil
	}

	// Fall back to PKCS#8 format (1Password exports keys this way)
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	key, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if pkcs8Err != nil {
		return nil, fmt.Errorf("parsing key: OpenSSH: %v, PKCS#8: %v", err, pkcs8Err)
	}
	return ssh.NewSignerFromKey(key)
}

func knownHostsCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	return knownhosts.New(path)
}
