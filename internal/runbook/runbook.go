package runbook

import "time"

// StepType identifies the kind of step to execute.
type StepType string

const (
	StepShell StepType = "shell"
	StepSSH   StepType = "ssh"
	StepHTTP  StepType = "http"
)

// ErrorPolicy determines what happens when a step fails.
type ErrorPolicy string

const (
	PolicyAbort    ErrorPolicy = "abort"
	PolicyContinue ErrorPolicy = "continue"
	PolicyRetry    ErrorPolicy = "retry"
)

// Runbook is a named sequence of steps loaded from a YAML definition.
type Runbook struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Variables   []VariableDef `yaml:"variables"`
	Steps       []Step        `yaml:"steps"`
	Notify      *NotifyConfig `yaml:"notify,omitempty"`
	FilePath    string        `yaml:"-"` // resolved path on disk
}

// NotifyConfig controls notifications sent after a runbook completes.
type NotifyConfig struct {
	On    string       `yaml:"on,omitempty"` // "always", "failure", "success" (default: "always")
	Slack *SlackConfig `yaml:"slack,omitempty"`
	MacOS bool         `yaml:"macos,omitempty"`
	Email *EmailConfig `yaml:"email,omitempty"`
}

// SlackConfig holds Slack webhook notification settings.
type SlackConfig struct {
	Webhook string `yaml:"webhook"` // URL or op:// reference
	Channel string `yaml:"channel,omitempty"`
}

// EmailConfig holds SMTP email notification settings.
type EmailConfig struct {
	To       string `yaml:"to"`
	From     string `yaml:"from"`
	Host     string `yaml:"host"` // host:port
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"` // can be op:// reference
}

// VariableDef defines a variable that a runbook accepts.
type VariableDef struct {
	Name     string `yaml:"name"`
	Default  string `yaml:"default"`
	Required bool   `yaml:"required"`
	Prompt   string `yaml:"prompt"`
	Secret   bool   `yaml:"secret,omitempty"` // resolve op:// refs via 1Password + keychain
}

// Step is a single unit of work within a runbook.
type Step struct {
	Name      string        `yaml:"name"`
	Type      StepType      `yaml:"type"`
	Shell     *ShellStep    `yaml:"shell,omitempty"`
	SSH       *SSHStep      `yaml:"ssh,omitempty"`
	HTTP      *HTTPStep     `yaml:"http,omitempty"`
	Condition string        `yaml:"condition,omitempty"`
	OnError   ErrorPolicy   `yaml:"on_error,omitempty"`
	Retries   int           `yaml:"retries,omitempty"`
	Timeout   Duration      `yaml:"timeout,omitempty"`
	Parallel  bool          `yaml:"parallel,omitempty"`
	Confirm   string        `yaml:"confirm,omitempty"`
	Capture   string        `yaml:"capture,omitempty"`
}

// ShellStep runs a command in the local shell.
type ShellStep struct {
	Command string `yaml:"command"`
	Dir     string `yaml:"dir,omitempty"`
}

// SSHStep runs a command on a remote host.
type SSHStep struct {
	Host      string `yaml:"host"`
	User      string `yaml:"user"`
	Port      int    `yaml:"port,omitempty"`
	KeyFile   string `yaml:"key_file,omitempty"`
	Command   string `yaml:"command"`
	AgentAuth bool   `yaml:"agent_auth,omitempty"`
}

// HTTPStep makes an HTTP request.
type HTTPStep struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

// Duration wraps time.Duration for YAML unmarshaling of strings like "5m", "30s".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}
