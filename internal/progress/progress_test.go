package progress

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFormatCount(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 999: "999", 1000: "1.000", 1240: "1.240", 1234567: "1.234.567", -1500: "-1.500"}
	for in, want := range cases {
		if got := FormatCount(in); got != want {
			t.Errorf("FormatCount(%d) = %q, esperado %q", in, got, want)
		}
	}
}

func TestFormatElapsed(t *testing.T) {
	cases := map[time.Duration]string{
		0:                                     "00:00",
		12 * time.Second:                      "00:12",
		90*time.Second + 900*time.Millisecond: "01:30",
		time.Hour + 2*time.Minute + 3*time.Second: "01:02:03",
		-time.Second: "00:00",
	}
	for in, want := range cases {
		if got := FormatElapsed(in); got != want {
			t.Errorf("FormatElapsed(%v) = %q, esperado %q", in, got, want)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		phase      Phase
		rows, tabs int
		el         time.Duration
		want       string
	}{
		{PhaseCollect, 1240, 7, 12 * time.Second, "Coletando relações... 1.240 linhas · 7 tabelas · 00:12"},
		{PhaseMetadata, 0, 0, 2 * time.Second, "Lendo metadados... · 00:02"},
		{PhaseGenerate, 1, 1, 0, "Gerando SQL... 1 linha · 1 tabela · 00:00"},
		{PhaseWrite, 3, 2, 5 * time.Second, "Gravando arquivo... 3 linhas · 2 tabelas · 00:05"},
	}
	for _, c := range cases {
		if got := Format(c.phase, c.rows, c.tabs, c.el); got != c.want {
			t.Errorf("Format = %q, esperado %q", got, c.want)
		}
	}
}

type fakeInfo struct{ mode fs.FileMode }

func (f fakeInfo) Name() string       { return "fake" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

type fakeStat struct {
	info fs.FileInfo
	err  error
}

func (f fakeStat) Stat() (fs.FileInfo, error) { return f.info, f.err }

func TestIsTTY(t *testing.T) {
	if !IsTTY(fakeStat{info: fakeInfo{fs.ModeDevice | fs.ModeCharDevice}}) {
		t.Error("dispositivo de caracteres deveria ser TTY")
	}
	if IsTTY(fakeStat{info: fakeInfo{fs.ModeNamedPipe}}) {
		t.Error("pipe não é TTY")
	}
	if IsTTY(fakeStat{info: fakeInfo{0}}) {
		t.Error("arquivo comum não é TTY")
	}
	if IsTTY(fakeStat{err: errors.New("falhou")}) {
		t.Error("erro de Stat não é TTY")
	}
	if IsTTY(nil) {
		t.Error("nil não é TTY")
	}
}

func TestReporterNilEhSeguro(t *testing.T) {
	var r *Reporter
	r.SetPhase(PhaseWrite)
	r.AddRow("t")
	r.Stop()
	if FromContext(context.Background()) != nil {
		t.Error("contexto sem reporter deveria devolver nil")
	}
}

// syncBuffer evita corrida entre a goroutine do reporter e o teste.
type syncBuffer struct {
	mu  chan struct{}
	buf bytes.Buffer
}

func newSyncBuffer() *syncBuffer {
	b := &syncBuffer{mu: make(chan struct{}, 1)}
	return b
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu <- struct{}{}
	defer func() { <-b.mu }()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu <- struct{}{}
	defer func() { <-b.mu }()
	return b.buf.String()
}

func TestReporterEscreveELimpaALinha(t *testing.T) {
	buf := newSyncBuffer()
	r := Start(buf, time.Millisecond)
	r.SetPhase(PhaseCollect)
	r.AddRow("users")
	r.AddRow("users")
	r.AddRow("orders")
	time.Sleep(30 * time.Millisecond)
	r.Stop()
	r.Stop() // idempotente

	out := buf.String()
	if !strings.Contains(out, "\rColetando relações... 3 linhas · 2 tabelas · ") {
		t.Errorf("linha de progresso ausente: %q", out)
	}
	if !strings.HasSuffix(out, "\r") {
		t.Errorf("a saída deveria terminar com a linha apagada (\\r): %q", out)
	}
	after := len(out)
	time.Sleep(10 * time.Millisecond)
	if len(buf.String()) != after {
		t.Error("nada deve ser escrito depois do Stop")
	}
}

func TestReporterNaoVazaGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		Start(newSyncBuffer(), time.Millisecond).Stop()
	}
	time.Sleep(20 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutines vazadas: antes=%d depois=%d", before, after)
	}
}

func TestReporterNoContexto(t *testing.T) {
	r := Start(newSyncBuffer(), time.Hour)
	defer r.Stop()
	if FromContext(WithReporter(context.Background(), r)) != r {
		t.Error("FromContext deveria devolver o reporter guardado")
	}
}
