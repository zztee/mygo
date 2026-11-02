package text

import "testing"

func TestBaselineFeaturesPreserveOtherTypography(t *testing.T) {
	for _, list := range []string{"", "kern=0,tnum=1,liga=1,rlig=1", "liga=1,liga=1,clig=1,dlig=1,hlig=1,rlig=1,ss01=3"} {
		got := features(baselineFeatures(list))
		seen := map[string]uint32{}
		for _, f := range got {
			tag := string(f.tag[:])
			if _, ok := seen[tag]; ok {
				t.Fatalf("duplicate feature %q", tag)
			}
			seen[tag] = f.value
		}
		for _, tag := range []string{"liga", "clig", "dlig", "hlig", "rlig"} {
			if value, ok := seen[tag]; !ok || value != 0 {
				t.Fatalf("%s not suppressed in %q", tag, list)
			}
		}
		for _, f := range features(list) {
			tag := string(f.tag[:])
			switch tag {
			case "liga", "clig", "dlig", "hlig", "rlig":
				continue
			}
			if seen[tag] != f.value {
				t.Fatalf("feature %q lost in %q", tag, list)
			}
		}
	}
}
