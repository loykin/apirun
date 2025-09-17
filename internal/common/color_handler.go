package common

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"
)

// ColorHandler implements a colorized text handler for slog
type ColorHandler struct {
	opts     *slog.HandlerOptions
	writer   io.Writer
	attrs    []slog.Attr
	groups   []string
	masker   *Masker
	useColor bool
}

// NewColorHandler creates a new color handler
func NewColorHandler(w io.Writer, opts *slog.HandlerOptions) *ColorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	// Auto-detect if we should use colors
	useColor := shouldUseColor(w)

	return &ColorHandler{
		opts:     opts,
		writer:   w,
		useColor: useColor,
		masker:   NewMasker(),
	}
}

// shouldUseColor determines if colors should be used based on the output
func shouldUseColor(w io.Writer) bool {
	// Don't use colors on Windows by default
	if runtime.GOOS == "windows" {
		return false
	}

	// Check if writing to a terminal
	if f, ok := w.(*os.File); ok {
		return isTerminal(f)
	}

	return false
}

// isTerminal checks if the file is a terminal
func isTerminal(f *os.File) bool {
	// Simple check for terminal - this is a basic implementation
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// Enabled reports whether the handler handles records at the given level
func (h *ColorHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle handles the Record
func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	buf := make([]byte, 0, 1024)

	// Format timestamp
	if !r.Time.IsZero() {
		buf = append(buf, h.colorize(Gray, r.Time.Format(time.RFC3339))...)
		buf = append(buf, " "...)
	}

	// Format level with color
	levelStr := h.formatLevel(r.Level)
	buf = append(buf, levelStr...)
	buf = append(buf, " "...)

	// Format component if present in groups
	if len(h.groups) > 0 {
		component := strings.Join(h.groups, ".")
		buf = append(buf, h.colorize(Cyan, fmt.Sprintf("[%s]", component))...)
		buf = append(buf, " "...)
	}

	// Format message
	buf = append(buf, h.colorize(White, r.Message)...)

	// Format attributes
	attrs := make([]slog.Attr, 0, r.NumAttrs()+len(h.attrs))
	attrs = append(attrs, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	if len(attrs) > 0 {
		// Apply masking to attributes
		maskedAttrs := h.maskAttributes(attrs)
		buf = append(buf, " "...)
		buf = h.formatAttributes(buf, maskedAttrs)
	}

	buf = append(buf, "\n"...)

	_, err := h.writer.Write(buf)
	return err
}

// formatLevel formats the log level with appropriate colors
func (h *ColorHandler) formatLevel(level slog.Level) string {
	var color string
	var levelStr string

	switch level {
	case slog.LevelDebug:
		color = Gray
		levelStr = "DEBUG"
	case slog.LevelInfo:
		color = Green
		levelStr = "INFO "
	case slog.LevelWarn:
		color = Yellow
		levelStr = "WARN "
	case slog.LevelError:
		color = Red
		levelStr = "ERROR"
	default:
		color = White
		levelStr = "UNKNOWN"
	}

	return h.colorize(color, fmt.Sprintf("[%s]", levelStr))
}

// formatAttributes formats attributes with colors
func (h *ColorHandler) formatAttributes(buf []byte, attrs []slog.Attr) []byte {
	for i, attr := range attrs {
		if i > 0 {
			buf = append(buf, " "...)
		}

		// Key in cyan
		buf = append(buf, h.colorize(Cyan, attr.Key)...)
		buf = append(buf, "="...)

		// Value formatting with appropriate colors
		valueStr := h.formatValue(attr.Value)
		buf = append(buf, valueStr...)
	}
	return buf
}

// formatValue formats a slog.Value with appropriate coloring
func (h *ColorHandler) formatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		str := v.String()
		// Different colors for different types of values
		if h.isErrorLike(str) {
			return h.colorize(Red, fmt.Sprintf("%q", str))
		} else if h.isSuccessLike(str) {
			return h.colorize(Green, fmt.Sprintf("%q", str))
		} else {
			return h.colorize(White, fmt.Sprintf("%q", str))
		}
	case slog.KindInt64:
		return h.colorize(Magenta, fmt.Sprintf("%d", v.Int64()))
	case slog.KindFloat64:
		return h.colorize(Magenta, fmt.Sprintf("%g", v.Float64()))
	case slog.KindBool:
		if v.Bool() {
			return h.colorize(Green, "true")
		} else {
			return h.colorize(Red, "false")
		}
	case slog.KindDuration:
		return h.colorize(Yellow, v.Duration().String())
	case slog.KindTime:
		return h.colorize(Gray, v.Time().Format(time.RFC3339))
	default:
		return h.colorize(White, v.String())
	}
}

// isErrorLike checks if a string looks like an error
func (h *ColorHandler) isErrorLike(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "error") || strings.Contains(s, "failed") ||
		strings.Contains(s, "fail") || strings.Contains(s, "exception")
}

// isSuccessLike checks if a string looks like success
func (h *ColorHandler) isSuccessLike(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "success") || strings.Contains(s, "complete") ||
		strings.Contains(s, "ok") || s == "applied"
}

// colorize applies color to text if colors are enabled
func (h *ColorHandler) colorize(color, text string) string {
	if !h.useColor {
		return text
	}
	return color + text + Reset
}

// maskAttributes applies masking to attributes
func (h *ColorHandler) maskAttributes(attrs []slog.Attr) []slog.Attr {
	if h.masker == nil || !h.masker.IsEnabled() {
		return attrs
	}

	masked := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		originalValue := attr.Value.Any()
		maskedValue := h.masker.MaskValue(attr.Key, originalValue)

		// If the value was masked (changed to string "***MASKED***"), create a StringValue
		if maskedStr, ok := maskedValue.(string); ok && maskedStr == "***MASKED***" {
			masked[i] = slog.Attr{
				Key:   attr.Key,
				Value: slog.StringValue(maskedStr),
			}
		} else {
			// Keep original value with type information
			masked[i] = attr
		}
	}
	return masked
}

// WithAttrs returns a new ColorHandler with the given attributes added
func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{
		opts:     h.opts,
		writer:   h.writer,
		attrs:    append(h.attrs, attrs...),
		groups:   h.groups,
		masker:   h.masker,
		useColor: h.useColor,
	}
}

// WithGroup returns a new ColorHandler with the given group name added
func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{
		opts:     h.opts,
		writer:   h.writer,
		attrs:    h.attrs,
		groups:   append(h.groups, name),
		masker:   h.masker,
		useColor: h.useColor,
	}
}

// SetMasker sets the masker for this handler
func (h *ColorHandler) SetMasker(masker *Masker) {
	h.masker = masker
}

// SetColorEnabled enables or disables colors
func (h *ColorHandler) SetColorEnabled(enabled bool) {
	h.useColor = enabled
}
