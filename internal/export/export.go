package export

import (
	"fmt"
	"os"
	"time"
)

// WritePlaceholder gera um arquivo SQL provisório para sinalizar que o fluxo funcionou.
func WritePlaceholder(path, table, column string, now time.Time) error {
	content := fmt.Sprintf(`-- TraceSQL export placeholder
-- gerado em %s
-- tabela: %s
-- coluna de referência: %s

-- TODO: implementar coleta de relações e inserts encadeados.
`, now.Format(time.RFC3339), table, column)

	return os.WriteFile(path, []byte(content), 0o644)
}
