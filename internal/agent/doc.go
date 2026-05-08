// Package agent bridges the agentsdk runtime ([Runtime], [NewSDKRuntime]), turn invocation
// ([RunText], [RunResponseWithMetadata]), session id resolution ([SessionResolver]), and
// slash-driven post-turn effects ([PostActionHandler]). Gateway and pipeline own wiring;
// this package stays the single place for SDK-shaped helpers that are not channel or bus code.
package agent
