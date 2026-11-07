package policy

import (
	"regexp"
	"strings"
	"testing"
)

func TestDependabotCobreGomodEGithubActions(t *testing.T) {
	cfg := readRepoFile(t, ".github/dependabot.yml")
	for _, eco := range []string{"gomod", "github-actions"} {
		re := regexp.MustCompile(`(?m)package-ecosystem:\s*["']?` + eco + `["']?\s*$`)
		if !re.MatchString(cfg) {
			t.Errorf("dependabot.yml não declara o ecossistema %s", eco)
		}
	}
}

func TestDependabotIntervaloSemanal(t *testing.T) {
	cfg := readRepoFile(t, ".github/dependabot.yml")
	n := len(regexp.MustCompile(`(?m)interval:\s*["']?weekly["']?\s*$`).FindAllString(cfg, -1))
	if n < 2 {
		t.Errorf("esperava intervalo weekly nos 2 ecossistemas, achei %d", n)
	}
	if !regexp.MustCompile(`(?m)timezone:\s*["']?America/Sao_Paulo["']?\s*$`).MatchString(cfg) {
		t.Error("dependabot.yml deveria usar o fuso America/Sao_Paulo")
	}
}

func TestDependabotLabelDependenciesENuncaUpgrade(t *testing.T) {
	cfg := readRepoFile(t, ".github/dependabot.yml")
	if !regexp.MustCompile(`(?m)^\s*-\s*["']?dependencies["']?\s*$`).MatchString(cfg) {
		t.Error("dependabot.yml deveria aplicar a label dependencies")
	}
	if strings.Contains(cfg, "upgrade") {
		t.Error("dependabot.yml não pode usar a label upgrade (ela sobe a versão major)")
	}
}

func TestDependabotPrefixoChoreDeps(t *testing.T) {
	cfg := readRepoFile(t, ".github/dependabot.yml")
	n := len(regexp.MustCompile(`(?m)prefix:\s*["']?chore\(deps\)["']?\s*$`).FindAllString(cfg, -1))
	if n < 2 {
		t.Errorf("esperava prefixo chore(deps) nos 2 ecossistemas, achei %d", n)
	}
}

func TestDependabotLimiteDePRs(t *testing.T) {
	cfg := readRepoFile(t, ".github/dependabot.yml")
	n := len(regexp.MustCompile(`(?m)open-pull-requests-limit:\s*\d+\s*$`).FindAllString(cfg, -1))
	if n < 2 {
		t.Errorf("esperava open-pull-requests-limit nos 2 ecossistemas, achei %d", n)
	}
}

func TestDependabotAgrupaMinorEPatch(t *testing.T) {
	cfg := readRepoFile(t, ".github/dependabot.yml")
	if !regexp.MustCompile(`(?m)^\s*groups:\s*$`).MatchString(cfg) {
		t.Error("dependabot.yml deveria ter groups para agrupar minor e patch")
	}
	for _, kind := range []string{"minor", "patch"} {
		if !regexp.MustCompile(`(?m)^\s*-\s*["']?` + kind + `["']?\s*$`).MatchString(cfg) {
			t.Errorf("o grupo deveria incluir update-types %s", kind)
		}
	}
}

func TestTagOnMergeIgnoraDependabot(t *testing.T) {
	wf := readRepoFile(t, ".github/workflows/tag_on_merge.yml")
	job := jobBlock(t, wf, "create_tag_and_draft_release")
	re := regexp.MustCompile(`github\.event\.pull_request\.user\.login\s*!=\s*'dependabot\[bot\]'`)
	if !re.MatchString(job) {
		t.Error("o job create_tag_and_draft_release deveria ignorar PRs de dependabot[bot]")
	}
}

func TestSecurityMdExiste(t *testing.T) {
	doc := readRepoFile(t, "docs/SECURITY.md")
	if strings.TrimSpace(doc) == "" {
		t.Error("docs/SECURITY.md está vazio")
	}
}

func TestSecurityMdOrientaReporteSemEmailNemIssuePublica(t *testing.T) {
	doc := readRepoFile(t, "docs/SECURITY.md")
	if !strings.Contains(doc, "Report a vulnerability") {
		t.Error("docs/SECURITY.md deveria orientar o uso de Report a vulnerability")
	}
	if regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`).MatchString(doc) {
		t.Error("docs/SECURITY.md não deve indicar e-mail para reporte")
	}
}
