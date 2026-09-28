// Package docs embeds the agent-facing language guide so `veld spec` can
// print it without network or repository access.
package docs

import _ "embed"

//go:embed LANGUAGE.md
var Language string
