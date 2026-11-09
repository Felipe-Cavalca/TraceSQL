// Package progress exibe uma linha de progresso no terminal durante a exportação.
//
// A linha é reescrita com "\r" e mostra a fase, o contador de linhas e tabelas
// e o tempo decorrido. Não há percentual porque o total é desconhecido.
package progress

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Phase identifica a etapa atual da exportação.
type Phase int

const (
	PhaseMetadata Phase = iota
	PhaseCollect
	PhaseGenerate
	PhaseWrite
)

// Label devolve o texto da fase, sem as reticências.
func (p Phase) Label() string {
	switch p {
	case PhaseMetadata:
		return "Lendo metadados"
	case PhaseCollect:
		return "Coletando relações"
	case PhaseGenerate:
		return "Gerando SQL"
	case PhaseWrite:
		return "Gravando arquivo"
	default:
		return "Processando"
	}
}

// DefaultInterval é o intervalo de atualização da linha.
const DefaultInterval = 200 * time.Millisecond

// FormatCount formata um inteiro com ponto como separador de milhar (1.240).
func FormatCount(n int) string {
	raw := strconv.Itoa(n)
	neg := strings.HasPrefix(raw, "-")
	if neg {
		raw = raw[1:]
	}
	var b strings.Builder
	for i, r := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// FormatElapsed formata a duração como MM:SS, ou HH:MM:SS a partir de 1 hora.
func FormatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Second)
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// Format monta a linha de progresso. Os contadores só aparecem quando há linhas.
func Format(phase Phase, rows, tables int, elapsed time.Duration) string {
	line := phase.Label() + "..."
	if rows > 0 {
		line += " " + FormatCount(rows) + " " + plural(rows, "linha", "linhas")
		line += " · " + FormatCount(tables) + " " + plural(tables, "tabela", "tabelas")
	}
	return line + " · " + FormatElapsed(elapsed)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// Statter é o que *os.File oferece para descobrir o tipo do arquivo.
type Statter interface {
	Stat() (fs.FileInfo, error)
}

// IsTTY informa se o arquivo é um terminal (dispositivo de caracteres).
// Pipes, arquivos e erros de Stat contam como "não é terminal".
func IsTTY(f Statter) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&fs.ModeCharDevice != 0
}

// Reporter reescreve uma única linha com o andamento. Todos os métodos aceitam
// receptor nil e então não fazem nada, o que permite desligar o progresso sem
// ifs espalhados pelo código.
type Reporter struct {
	w        io.Writer
	interval time.Duration
	start    time.Time

	mu       sync.Mutex
	phase    Phase
	rows     int
	tables   map[string]struct{}
	lastLen  int
	stopped  bool
	done     chan struct{}
	finished chan struct{}
}

// Start cria o Reporter e inicia a goroutine de atualização. Chame Stop
// (idempotente) para encerrá-la e limpar a linha.
func Start(w io.Writer, interval time.Duration) *Reporter {
	if interval <= 0 {
		interval = DefaultInterval
	}
	r := &Reporter{
		w:        w,
		interval: interval,
		start:    time.Now(),
		tables:   map[string]struct{}{},
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	r.render()
	go r.loop()
	return r
}

func (r *Reporter) loop() {
	defer close(r.finished)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.render()
		case <-r.done:
			return
		}
	}
}

// SetPhase troca a fase exibida.
func (r *Reporter) SetPhase(p Phase) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.phase = p
	r.mu.Unlock()
}

// AddRow conta uma linha coletada da tabela informada.
func (r *Reporter) AddRow(table string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.rows++
	r.tables[table] = struct{}{}
	r.mu.Unlock()
}

func (r *Reporter) render() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	line := Format(r.phase, r.rows, len(r.tables), time.Since(r.start))
	width := utf8.RuneCountInString(line)
	pad := ""
	if r.lastLen > width {
		pad = strings.Repeat(" ", r.lastLen-width)
	}
	r.lastLen = width
	fmt.Fprint(r.w, "\r"+line+pad)
}

// Stop encerra a goroutine, espera ela terminar e apaga a linha. Pode ser
// chamado mais de uma vez.
func (r *Reporter) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	close(r.done)
	r.mu.Unlock()
	<-r.finished

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastLen > 0 {
		fmt.Fprint(r.w, "\r"+strings.Repeat(" ", r.lastLen)+"\r")
	}
}

type ctxKey struct{}

// WithReporter guarda o Reporter no contexto, para o exportador usá-lo sem
// mudar a assinatura de export.Run.
func WithReporter(ctx context.Context, r *Reporter) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext devolve o Reporter do contexto, ou nil (que é seguro de usar).
func FromContext(ctx context.Context) *Reporter {
	r, _ := ctx.Value(ctxKey{}).(*Reporter)
	return r
}
