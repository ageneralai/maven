package web

import (
	"fmt"
	"net/http"
	"strings"
)

func resolveMavenSessionID(r *http.Request, previousResponseID string) (string, error) {
	headerSession := strings.TrimSpace(r.Header.Get(HeaderMavenSessionID))
	prev := strings.TrimSpace(previousResponseID)
	if prev != "" {
		if !isMavenResponseID(prev) {
			return "", fmt.Errorf("invalid previous_response_id")
		}
		mapped, ok := lookupMavenResponseSession(prev)
		if !ok {
			return "", fmt.Errorf("unknown previous_response_id")
		}
		if headerSession != "" && headerSession != mapped {
			return "", fmt.Errorf("session mismatch")
		}
		return mapped, nil
	}
	if headerSession != "" {
		return headerSession, nil
	}
	return "", fmt.Errorf("Maven-Session-Id required")
}
