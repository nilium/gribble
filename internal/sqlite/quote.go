package sqlite

import "strings"

func QuoteText(s string) string {
	return `'` + strings.Replace(s, `'`, `''`, -1) + `'`
}

func QuoteIdentifier(s string) string {
	return `"` + strings.Replace(s, `"`, `""`, -1) + `"`
}
