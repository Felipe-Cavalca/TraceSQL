package policy

import (
	"regexp"
	"strings"
	"testing"
)

func TestCIJobVulncheckExiste(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	job := jobBlock(t, ci, "vulncheck")
	if !regexp.MustCompile(`govulncheck@v\d+\.\d+\.\d+\s+\./\.\.\.`).MatchString(job) {
		t.Error("o job vulncheck deveria rodar govulncheck@vX.Y.Z ./... (versão fixa)")
	}
	if strings.Contains(job, "!= 'schedule'") {
		t.Error("o job vulncheck precisa rodar também no schedule")
	}
}

func TestCIRodaSemanalmenteAsSegundas0600UTC(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	if !regexp.MustCompile(`(?m)^\s*schedule:\s*$`).MatchString(ci) {
		t.Error("ci.yml deveria ter o gatilho schedule")
	}
	if !regexp.MustCompile(`(?m)cron:\s*['"]0 6 \* \* 1['"]`).MatchString(ci) {
		t.Error("ci.yml deveria ter cron '0 6 * * 1'")
	}
}

func TestCITestENotifyNaoRodamNoSchedule(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	for _, name := range []string{"test", "notify"} {
		job := jobBlock(t, ci, name)
		if !regexp.MustCompile(`github\.event_name\s*!=\s*'schedule'`).MatchString(job) {
			t.Errorf("o job %s deveria ter a condição github.event_name != 'schedule'", name)
		}
	}
}

func TestCINotifyIgnoraDependabot(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	job := jobBlock(t, ci, "notify")
	if !regexp.MustCompile(`github\.actor\s*!=\s*'dependabot\[bot\]'`).MatchString(job) {
		t.Error("o job notify deveria ter a condição github.actor != 'dependabot[bot]'")
	}
}

func TestCIUsaGoDoGoMod(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	if !regexp.MustCompile(`(?m)go-version-file:\s*['"]?go\.mod['"]?\s*$`).MatchString(ci) {
		t.Error("ci.yml deveria usar go-version-file: go.mod")
	}
	if regexp.MustCompile(`(?m)^\s*go-version:`).MatchString(ci) {
		t.Error("ci.yml não deve fixar go-version; use go-version-file")
	}
	for _, name := range []string{"test", "vulncheck"} {
		if !strings.Contains(jobBlock(t, ci, name), "go-version-file") {
			t.Errorf("o job %s deveria instalar o Go via go-version-file", name)
		}
	}
}

func TestReleaseUsaGoDoGoMod(t *testing.T) {
	rel := readRepoFile(t, ".github/workflows/release.yml")
	if !regexp.MustCompile(`(?m)go-version-file:\s*['"]?go\.mod['"]?\s*$`).MatchString(rel) {
		t.Error("release.yml deveria usar go-version-file: go.mod")
	}
	if regexp.MustCompile(`(?m)^\s*go-version:`).MatchString(rel) {
		t.Error("release.yml não deve fixar go-version; use go-version-file")
	}
}

// As actions do ci.yml ficam fixadas por SHA de 40 hex, com a versão em comentário.
func TestCIActionsFixadasPorSHAComComentarioDeVersao(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	usesRe := regexp.MustCompile(`(?m)^\s*(?:-\s*)?uses:\s*(\S+)(.*)$`)
	pinned := regexp.MustCompile(`^[\w.-]+/[\w./-]+@[0-9a-f]{40}$`)
	comment := regexp.MustCompile(`^\s+#\s*v\d+(\.\d+)*\S*\s*$`)
	matches := usesRe.FindAllStringSubmatch(ci, -1)
	if len(matches) == 0 {
		t.Fatal("nenhum uses: encontrado no ci.yml")
	}
	for _, m := range matches {
		ref, rest := m[1], m[2]
		if strings.HasPrefix(ref, "./") {
			continue
		}
		if !pinned.MatchString(ref) {
			t.Errorf("action sem SHA de 40 hex: %s", ref)
		}
		if !comment.MatchString(rest) {
			t.Errorf("action %s sem comentário '# vX.Y.Z' de versão (resto da linha: %q)", ref, rest)
		}
	}
}

func TestGoModDeclaraGo126(t *testing.T) {
	mod := readRepoFile(t, "go.mod")
	if !regexp.MustCompile(`(?m)^go 1\.26\.0\s*$`).MatchString(mod) {
		t.Error("go.mod deveria declarar 'go 1.26.0'")
	}
}
