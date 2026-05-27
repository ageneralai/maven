// Package agent bridges the agentsdk runtime ([Runtime], [NewSDKRuntime]) and turn helpers.
// Session id resolution lives in [github.com/ageneralai/maven/kernel/session].
// Post-turn effects live in [github.com/ageneralai/maven/kernel/agent/postaction].
// Gateway and pipeline own wiring; this package stays the single place for SDK-shaped helpers.
package agent
