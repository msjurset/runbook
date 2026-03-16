package notify

import (
	"fmt"
	"os/exec"
)

func sendDesktop(runbookName, subject string) error {
	script := fmt.Sprintf(
		`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; `+
			`[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null; `+
			`$xml = [Windows.Data.Xml.Dom.XmlDocument]::new(); `+
			`$xml.LoadXml('<toast><visual><binding template="ToastText02"><text id="1">%s</text><text id="2">%s</text></binding></visual></toast>'); `+
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($xml); `+
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("runbook").Show($toast)`,
		runbookName, subject,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("powershell toast: %s: %w", string(out), err)
	}
	return nil
}
