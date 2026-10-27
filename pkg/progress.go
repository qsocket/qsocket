package qsock

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/utils"
	"golang.org/x/term"
)

const (
	// PROGRESS_INTERVAL is the shortest time between two redraws.
	PROGRESS_INTERVAL = 100 * time.Millisecond
	// PROGRESS_NAME_WIDTH is the number of columns reserved for the name.
	PROGRESS_NAME_WIDTH = 20
	// PROGRESS_MIN_BAR is the narrowest bar worth drawing, the bar is left
	// out entirely on terminals with less room than this.
	PROGRESS_MIN_BAR = 8
	// DEFAULT_TERM_WIDTH is used when the terminal size is unavailable.
	DEFAULT_TERM_WIDTH = 80
)

// Progress renders the state of a single file transfer on the terminal.
//
// It implements io.Writer so that it can be chained onto the stream carrying
// the file contents, every write advances the bar. A disabled Progress
// renders nothing, which keeps the transfer code free of terminal checks.
type Progress struct {
	name    string
	total   uint64
	done    uint64
	start   time.Time
	drawn   time.Time
	enabled bool
}

// ShowProgress reports whether the transfers of this session can draw a
// progress bar, it needs a terminal nobody else is writing on.
func ShowProgress(opts *config.Options) bool {
	if opts.Quiet || opts.Listen {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// NewProgress returns a progress bar for a file of the given size.
func NewProgress(name string, total uint64, enabled bool) *Progress {
	p := &Progress{
		name:    name,
		total:   total,
		enabled: enabled,
		start:   time.Now(),
	}
	p.render(true)
	return p
}

// Write counts the given data towards the progress of the transfer, it never
// fails so that it can be chained with io.MultiWriter.
func (p *Progress) Write(b []byte) (int, error) {
	p.done += uint64(len(b))
	p.render(false)
	return len(b), nil
}

// Close draws the final state of the bar and ends its line.
func (p *Progress) Close() {
	if !p.enabled {
		return
	}
	p.render(true)
	fmt.Fprint(os.Stderr, "\n")
}

// render redraws the bar, it is throttled to PROGRESS_INTERVAL unless the
// redraw is forced.
func (p *Progress) render(force bool) {
	if !p.enabled {
		return
	}
	now := time.Now()
	if !force && now.Sub(p.drawn) < PROGRESS_INTERVAL {
		return
	}
	p.drawn = now

	ratio := 1.0
	if p.total > 0 {
		ratio = float64(p.done) / float64(p.total)
		if ratio > 1 {
			ratio = 1
		}
	}
	rate := uint64(0)
	if elapsed := now.Sub(p.start).Seconds(); elapsed > 0 {
		rate = uint64(float64(p.done) / elapsed)
	}

	// The bar is given whatever is left of the terminal line once the
	// fixed parts of the line have been laid out.
	stats := fmt.Sprintf("%3.0f%% %s/%s %s/s",
		ratio*100,
		humanBytes(p.done),
		humanBytes(p.total),
		humanBytes(rate),
	)
	name := fitName(p.name, PROGRESS_NAME_WIDTH)
	width := terminalWidth() - len(name) - len(stats) - 8
	line := fmt.Sprintf("[>] %s %s", name, stats)
	if width >= PROGRESS_MIN_BAR {
		filled := int(ratio * float64(width))
		line = fmt.Sprintf("[>] %s [%s%s] %s",
			name,
			strings.Repeat("=", filled),
			strings.Repeat(" ", width-filled),
			stats,
		)
	}

	if ratio*100 == 100 {
		line = fmt.Sprintf("%s %s", utils.BoldGreen.Sprintf("[+]"), name)
	}

	fmt.Fprintf(os.Stderr, "\r%s\x1b[K", line)
}

// fitName pads or truncates a file name into a fixed number of columns, a
// truncated name keeps its tail which carries the file name itself.
func fitName(name string, width int) string {
	runes := []rune(name)
	if len(runes) > width {
		return "..." + string(runes[len(runes)-width+3:])
	}
	return name + strings.Repeat(" ", width-len(runes))
}

// terminalWidth returns the number of columns of the terminal.
func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || width <= 0 {
		return DEFAULT_TERM_WIDTH
	}
	return width
}

// humanBytes renders a byte count in its closest binary unit.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	value := float64(n)
	for _, suffix := range []string{"K", "M", "G", "T", "P"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f%s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1fE", value/unit)
}

// Status reports what the process is waiting for on the terminal.
//
// A disabled Status is a no-op, so the callers do not need to care about
// whether there is a terminal to draw on.
type Status struct {
	spn *spinner.Spinner
}

// NewStatus returns a status indicator for the given options.
//
// A listening peer logs what it is doing instead, a spinner would interleave
// with the log lines of the sessions it is serving.
func NewStatus(opts *config.Options) *Status {
	if opts.Quiet || opts.Listen || !term.IsTerminal(int(os.Stderr.Fd())) {
		return &Status{}
	}
	return &Status{
		spn: spinner.New(
			spinner.CharSets[9],
			50*time.Millisecond,
			spinner.WithWriter(os.Stderr),
		),
	}
}

// Set updates the message shown next to the indicator.
func (s *Status) Set(format string, a ...any) {
	if s.spn == nil {
		return
	}
	s.spn.Suffix = fmt.Sprintf(" "+format, a...)
	s.spn.Start()
}

// Stop removes the indicator from the terminal.
func (s *Status) Stop() {
	if s.spn == nil {
		return
	}
	s.spn.Stop()
}
