package progress

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/containerd/console"
)

var (
	regexCleanLine = regexp.MustCompile("\x1b\\[[0-9]+m[\x1b]?")
)

type Writer struct {
	buf   bytes.Buffer
	w     io.Writer
	lines int
}

// NewWriter returns a writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w: w,
	}
}
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.buf.Write(p)
}
func (w *Writer) Flush() error {
	if w.buf.Len() == 0 {
		return nil
	}

	if err := w.clearLines(); err != nil {
		return err
	}
	w.lines = countLines(w.buf.String())

	if _, err := w.w.Write(w.buf.Bytes()); err != nil {
		return err
	}

	w.buf.Reset()
	return nil
}

func (w *Writer) clearLines() error {
	for i := 0; i < w.lines; i++ {
		if _, err := fmt.Fprintf(w.w, "\x1b[1A\x1b[2K\r"); err != nil {
			return err
		}
	}

	return nil
}

func countLines(output string) int {
	con, err := console.ConsoleFromFile(os.Stdin)
	if err != nil {
		return 0
	}
	ws, err := con.Size()
	if err != nil {
		return 0
	}
	width := int(ws.Width)
	if width <= 0 {
		return 0
	}
	strlines := strings.Split(output, "\n")
	lines := -1
	for _, line := range strlines {
		lines += (len(stripLine(line))-1)/width + 1
	}
	return lines
}

func stripLine(line string) string {
	return string(regexCleanLine.ReplaceAll([]byte(line), []byte{}))
}
