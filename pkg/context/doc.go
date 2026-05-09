// Package turnctx is the maven module path github.com/ageneralai/maven/pkg/context.
// Per-inbound-turn routing identity (channel + chat id) lives on context.Context for tools
// and downstream code via a single private typed key. Extend TurnContext for budgets and
// metadata when needed.
package turnctx
