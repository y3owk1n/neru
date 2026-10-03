package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"

	consoleTimeLayout = "2006-01-02 15:04:05.000Z07:00"
)

// consoleCore writes one tracing-style line per entry:
//
//	2026-10-03 14:02:11.482+08:00  INFO app.eventtap: mode activated mode=hints elapsed=3.1ms
//
// It is a Core rather than an Encoder so it can hold context fields as a slice
// in the order they were added, instead of implementing every ObjectEncoder
// method to record them.
type consoleCore struct {
	zapcore.LevelEnabler

	out    zapcore.WriteSyncer
	mu     *sync.Mutex
	color  bool
	fields []zapcore.Field
}

func newConsoleCore(out zapcore.WriteSyncer, level zapcore.LevelEnabler, color bool) *consoleCore {
	return &consoleCore{LevelEnabler: level, out: out, mu: &sync.Mutex{}, color: color}
}

func (c *consoleCore) With(fields []zapcore.Field) zapcore.Core {
	clone := *c
	clone.fields = append(append([]zapcore.Field(nil), c.fields...), fields...)

	return &clone
}

func (c *consoleCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}

	return ce
}

func (c *consoleCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	var line strings.Builder

	line.WriteString(c.paint(ansiDim, ent.Time.Format(consoleTimeLayout)))
	line.WriteByte(' ')
	line.WriteString(c.paint(levelColor(ent.Level), fmt.Sprintf("%5s", ent.Level.CapitalString())))
	line.WriteByte(' ')

	target := ent.LoggerName
	if target == "" && ent.Caller.Defined {
		target = ent.Caller.TrimmedPath()
	}

	if target != "" {
		line.WriteString(c.paint(ansiBold, target+":"))
		line.WriteByte(' ')
	}

	line.WriteString(strings.ReplaceAll(ent.Message, "\n", `\n`))
	c.writeFields(&line, c.fields)
	c.writeFields(&line, fields)
	line.WriteByte('\n')

	if ent.Stack != "" {
		line.WriteString(c.paint(ansiDim, ent.Stack))
		line.WriteByte('\n')
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := io.WriteString(c.out, line.String())
	if err != nil {
		return err //nolint:wrapcheck // zap reports sink errors to its ErrorOutput as-is.
	}

	if ent.Level > zapcore.ErrorLevel {
		return c.out.Sync() //nolint:wrapcheck // zap reports sink errors to its ErrorOutput as-is.
	}

	return nil
}

func (c *consoleCore) Sync() error {
	return c.out.Sync() //nolint:wrapcheck // genuineSyncFailure inspects the raw error.
}

// writeFields appends each field as key=value. Each field goes through a
// MapObjectEncoder, so errors, durations, objects and namespaces produce the
// same values zap's own encoders would.
func (c *consoleCore) writeFields(line *strings.Builder, fields []zapcore.Field) {
	for _, field := range fields {
		enc := zapcore.NewMapObjectEncoder()
		field.AddTo(enc)

		for key, value := range enc.Fields {
			line.WriteByte(' ')
			line.WriteString(c.paint(ansiDim, key+"="))
			line.WriteString(formatValue(value))
		}
	}
}

func (c *consoleCore) paint(code, text string) string {
	if !c.color {
		return text
	}

	return code + text + ansiReset
}

var levelColors = map[zapcore.Level]string{
	zapcore.DebugLevel: ansiBlue,
	zapcore.InfoLevel:  ansiGreen,
	zapcore.WarnLevel:  ansiYellow,
}

func levelColor(level zapcore.Level) string {
	if color, ok := levelColors[level]; ok {
		return color
	}

	return ansiBold + ansiRed
}

// formatValue renders a field value the way logfmt does. Scalars print bare
// unless they hold spaces, quotes or '=', and composites print as JSON.
func formatValue(value any) string {
	switch typed := value.(type) {
	case string:
		return quoteIfNeeded(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case fmt.Stringer, bool,
		int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64, complex64, complex128:
		return quoteIfNeeded(fmt.Sprint(typed))
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return quoteIfNeeded(fmt.Sprint(typed))
		}

		return string(encoded)
	}
}

func quoteIfNeeded(text string) string {
	if text == "" || strings.ContainsAny(text, " \t\n\"=") {
		return strconv.Quote(text)
	}

	return text
}
