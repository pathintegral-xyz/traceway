package shared

import (
	"fmt"
	"sort"
	"strings"
)

// RootFilterClause returns the WHERE fragment for the issues/endpoints
// root-vs-non-root filter. Older D1 rows may contain TEXT booleans.
func RootFilterClause(qualifiedCol, rootFilter string) string {
	switch rootFilter {
	case "root":
		return " AND " + qualifiedCol + " IN (1, 'true')"
	case "non_root":
		return " AND " + qualifiedCol + " IN (0, 'false')"
	default:
		return ""
	}
}

func MethodPrefix(method string) string {
	return strings.ToUpper(method) + " "
}

// UPPER on the column side so stored lowercase methods still match; SUBSTR rather than LIKE so % and _ in the value stay literal.
func MethodFilterClause(qualifiedCol, method string) (clause string, param string) {
	if method == "" {
		return "", ""
	}
	prefix := MethodPrefix(method)
	return fmt.Sprintf(" AND UPPER(SUBSTR(%s, 1, %d)) = :method", qualifiedCol, len(prefix)), prefix
}

// SortedKeys returns map keys in stable order so generated SQL and its
// bound parameters line up deterministically.
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
