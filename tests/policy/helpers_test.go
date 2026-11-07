// Package policy confere, lendo arquivos como texto, as regras de segurança e
// de dependências descritas em docs/SEGURANCA.md. Os testes conferem o formato
// (e não o valor exato) para não quebrar a cada PR do Dependabot.
package policy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot sobe as pastas até achar o go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod não encontrado subindo a partir do diretório de teste")
		}
		dir = parent
	}
}

// readRepoFile lê um arquivo do repositório e normaliza as quebras de linha.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("não foi possível ler %s: %v", rel, err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// jobBlock devolve o texto de um job de workflow (do "  nome:" até o próximo
// job no mesmo nível de indentação).
func jobBlock(t *testing.T, workflow, job string) string {
	t.Helper()
	start := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(job) + `:\s*$`).FindStringIndex(workflow)
	if start == nil {
		t.Fatalf("job %q não encontrado", job)
	}
	rest := workflow[start[1]:]
	if next := regexp.MustCompile(`(?m)^  [A-Za-z0-9_-]+:\s*$`).FindStringIndex(rest); next != nil {
		return workflow[start[0] : start[1]+next[0]]
	}
	return workflow[start[0]:]
}
