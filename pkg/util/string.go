package util

import "strings"

func StripIndent(multilineStr string) string {
	return strings.Replace(multilineStr, "\t", "", -1)
}
