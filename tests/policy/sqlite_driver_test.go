package policy

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestGoModUsaModerncSqliteNaVersaoMinima(t *testing.T) {
	mod := readRepoFile(t, "go.mod")
	m := regexp.MustCompile(`(?m)^\s*modernc\.org/sqlite\s+v(\d+)\.(\d+)\.(\d+)(.*)$`).FindStringSubmatch(mod)
	if m == nil {
		t.Fatal("go.mod deveria exigir modernc.org/sqlite")
	}
	if strings.Contains(m[4], "// indirect") {
		t.Error("modernc.org/sqlite deveria ser dependência direta, sem '// indirect'")
	}
	got := [3]int{}
	for i := range got {
		got[i], _ = strconv.Atoi(m[i+1])
	}
	want := [3]int{1, 62, 0}
	for i := range got {
		if got[i] > want[i] {
			return
		}
		if got[i] < want[i] {
			t.Fatalf("modernc.org/sqlite v%d.%d.%d é menor que v1.62.0", got[0], got[1], got[2])
		}
	}
}

func TestGoModSemGlebarezGormEJinzhu(t *testing.T) {
	for _, file := range []string{"go.mod", "go.sum"} {
		content := readRepoFile(t, file)
		for _, banned := range []string{"glebarez", "gorm", "jinzhu"} {
			if regexp.MustCompile(banned).MatchString(content) {
				t.Errorf("%s não deveria mencionar %s", file, banned)
			}
		}
	}
}
