// Command serve is the OUT-OF-PROCESS entrypoint for the cardwire command plugin:
// dual-mode sdk.Main (serve OR CLI). charly fork/execs this binary in CLI mode for
// command:cardwire dispatch when the plugin is NOT compiled-in (→ CliMain); the serve
// half backs the out-of-process provider placement. The SAME NewProvider()/NewMeta()
// compile INTO charly in-process when listed in compiled_plugins — placement is
// invisible above the registry.
package main

import (
	cardwire "github.com/opencharly/plugin-cardwire/candy/plugin-cardwire"
	"github.com/opencharly/sdk"
)

func main() { sdk.Main(cardwire.NewProvider(), cardwire.NewMeta(), cardwire.CliMain) }
