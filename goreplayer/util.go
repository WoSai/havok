package replayer

import (
	"github.com/google/uuid"
	"strings"
)

func generateReplayerId() string {
	prefix := "replayer-"
	id := strings.Join(strings.Split(uuid.NewString(), "-")[:2], "-")
	return prefix + id
}

func removeDuplicateElement(targets []string) []string {
	result := make([]string, 0, len(targets))
	temp := map[string]struct{}{}
	for _, item := range targets {
		if _, ok := temp[item]; !ok {
			temp[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
