package plugin

import goplugin "github.com/hashicorp/go-plugin"

// Handshake is what a runner and a plugin agree on before they talk, in the terms of hashicorp/go-plugin,
// which the runner uses to start plugins. The protocol version is the version of the plugin contract.
// The magic cookie is set in the environment of every plugin a runner starts: a plugin that does not find
// it was started by hand, not by a runner, and says so and exits.
//
// A plugin in another language does the same with its own code. It reads STEWARD_PLUGIN from its
// environment, prints the handshake line, and, when PLUGIN_UNIX_SOCKET_DIR is set, listens on a unix
// socket created in that directory.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "STEWARD_PLUGIN",
	MagicCookieValue: "b6d7a1f2-steward-plugin",
}

// Name is the name a plugin is served under, and the name of the service its health check answers for.
const Name = "plugin"
