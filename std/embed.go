// Package std embeds the Veld standard library sources. Most functions are
// written in Veld itself; `extern fn` declarations are implemented natively
// by the interpreter (internal/interp/natives.go).
package std

import "embed"

//go:embed *.veld
var FS embed.FS
