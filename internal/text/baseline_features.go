package text

import (
	"strconv"
	"strings"
)

// baselineFeatures disables ligature substitutions in a shifted span for
// engines without a shaping-run baseline attribute. Its other effective
// OpenType features remain in force.
func baselineFeatures(list string) string {
	var b strings.Builder
	for _, f := range features(list) {
		switch string(f.tag[:]) {
		case "liga", "clig", "dlig", "hlig", "rlig":
			continue
		}
		b.Write(f.tag[:])
		b.WriteByte('=')
		b.WriteString(strconv.FormatUint(uint64(f.value), 10))
		b.WriteByte(',')
	}
	b.WriteString("liga=0,clig=0,dlig=0,hlig=0,rlig=0")
	return b.String()
}
