package web

import (
	"fmt"
	"net/http"
	"strings"

	mavsession "github.com/ageneralai/maven/internal/session"
)

func resolveMavenSessionID(r *http.Request, previousResponseID string) (string, error) {
	headerSession := strings.TrimSpace(r.Header.Get(HeaderMavenSessionID))
	if headerSession == "" {
		headerSession = strings.TrimSpace(r.URL.Query().Get("session"))
	}
	prev := strings.TrimSpace(previousResponseID)
	if prev != "" {
		if !isMavenResponseID(prev) {
			return "", fmt.Errorf("invalid previous_response_id")
		}
		mapped, ok := lookupMavenResponseSession(prev)
		if !ok {
			return "", fmt.Errorf("unknown previous_response_id")
		}
		if headerSession != "" && mavsession.ChatSessionID(mavsession.WebChannelName, headerSession) != mapped {
			return "", fmt.Errorf("session mismatch")
		}
		return mapped, nil
	}
	if headerSession != "" {
		return mavsession.ChatSessionID(mavsession.WebChannelName, headerSession), nil
	}
	return "", fmt.Errorf("Maven-Session-Id required")
}
