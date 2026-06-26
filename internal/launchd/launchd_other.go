//go:build !darwin

package launchd

import "errors"

// On non-Darwin platforms the launchd backend isn't available — cron is
// the universal fallback and Linux's user cron service has no equivalent
// session-token problem that motivated this backend in the first place.

func Available() bool { return false }

func PlistPathFor(_ string) string { return "" }

func Install(_, _, _, _ string, _ []string) error {
	return errors.New("LaunchAgent backend is only available on macOS")
}

func Uninstall(_ string) error {
	return errors.New("LaunchAgent backend is only available on macOS")
}

func IsInstalled(_ string) bool { return false }
