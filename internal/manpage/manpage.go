package manpage

import _ "embed"

//go:generate cp ../../runbook.1 runbook.1

// Content holds the roff-formatted manual page, embedded at build time.
//
//go:embed runbook.1
var Content string
