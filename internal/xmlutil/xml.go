package xmlutil

import (
	"fmt"
	"strings"
)

type Attr struct {
	Key   string
	Value string
}

func TagOpen(name string, attrs ...Attr) string {
	if len(attrs) == 0 {
		return fmt.Sprintf("<%s>", name)
	}

	parts := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		parts = append(parts, fmt.Sprintf("%s=%q", attr.Key, attr.Value))
	}

	return fmt.Sprintf("<%s %s>", name, strings.Join(parts, " "))
}

func TagClose(name string) string {
	return fmt.Sprintf("</%s>", name)
}
