// Package agent bridges the agentsdk runtime ([Runtime], [NewSDKRuntime]), session id
// resolution ([SessionResolver]), and turn helpers. Post-turn effects live in
// [github.com/ageneralai/maven/internal/agent/postaction]. Gateway and pipeline own wiring;
// this package stays the single place for SDK-shaped helpers that are not channel or bus code.
package agent
