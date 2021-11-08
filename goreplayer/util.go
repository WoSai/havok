package replayer

import "github.com/wosai/havok/processor"

func generateReplayerId() string {
	prefix := "replayer-"
	return prefix + string(processor.RandStringBytesMaskImprSrc(10))
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
