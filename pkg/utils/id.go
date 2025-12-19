package utils

import (
	"regexp"
)

func ExtractVideoID(input string) string {
	regexPattern := `(?:https?://)?(?:www\.)?(?:youtube|youtu|youtube-nocookie)\.(?:com|be)/(?:watch\?v=|embed/|v/|.+\?v=|shorts/)?([^&=%\?]{11})`

	re := regexp.MustCompile(regexPattern)
	matches := re.FindStringSubmatch(input)

	if len(matches) >= 2 {
		return matches[1]
	}

	if len(input) == 11 {
		validIDRe := regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
		if validIDRe.MatchString(input) {
			return input
		}
	}

	return ""
}
