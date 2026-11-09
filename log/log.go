package log

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
)

const (
	LOGGER_PROMPT   = "[%s] %s "
	LOG_LEVEL_TRACE = iota
	LOG_LEVEL_DEBUG
	LOG_LEVEL_INFO
	LOG_LEVEL_WARN
	LOG_LEVEL_ERROR
	LOG_LEVEL_FATAL
)

var (
	red         = color.New(color.FgRed).Add(color.Bold)
	yellow      = color.New(color.FgYellow)
	blue        = color.New(color.FgHiBlue)
	criticalRed = color.New(color.BgRed).Add(color.Bold)
	faintWhite  = color.New(color.FgWhite).Add(color.Faint)
	logLevel    = LOG_LEVEL_INFO
)

// SetLevel sets the standard logger level.
func SetLevel(level int) {
	if level > LOG_LEVEL_FATAL ||
		level < LOG_LEVEL_TRACE {
		return
	}
	logLevel = level
}

// GetLevel returns the standard logger level.
func GetLevel() int {
	return logLevel
}

// Print logs a message at level Info on the standard logger.
func Print(args ...interface{}) {
	fmt.Print(
		fmt.Sprintf(
			"[%s] ",
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
		),
		fmt.Sprint(args...),
		"\n",
	)
}

// Trace logs a message at level Trace on the standard logger.
func Trace(args ...interface{}) {
	if logLevel != LOG_LEVEL_TRACE {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			faintWhite.Sprint("[TRACE]"),
		),
		fmt.Sprint(args...),
		"\n",
	)
}

// Debug logs a message at level Debug on the standard logger.
func Debug(args ...interface{}) {
	if logLevel > LOG_LEVEL_DEBUG {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			"[DEBUG]",
		),
		fmt.Sprint(args...),
		"\n",
	)
}

// Info logs a message at level Info on the standard logger.
func Info(args ...interface{}) {
	if logLevel > LOG_LEVEL_INFO {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			blue.Sprint("[INFO]"),
		),
		fmt.Sprint(args...),
		"\n",
	)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(args ...interface{}) {
	if logLevel > LOG_LEVEL_WARN {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			yellow.Sprint("[WARN]"),
		),
		fmt.Sprint(args...),
		"\n",
	)
}

// Error logs a message at level Error on the standard logger.
func Error(args ...interface{}) {
	if logLevel > LOG_LEVEL_ERROR {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			red.Sprint("[ERROR]"),
		),
		fmt.Sprint(args...),
		"\n",
	)
}

// Fatal logs a message at level Fatal on the standard logger then the process will exit with status set to 1.
//
// Fatal is the highest level, it is printed at every log level (quiet mode
// included) and always exits.
func Fatal(args ...interface{}) {
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			criticalRed.Sprint("[FATAL]"),
		),
		red.Sprint(args...),
		"\n",
	)
	os.Exit(1)
}

// Tracef logs a message at level Trace on the standard logger.
func Tracef(format string, args ...interface{}) {
	if logLevel != LOG_LEVEL_TRACE {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			faintWhite.Sprint("[TRACE]"),
		),
		fmt.Sprintf(format, args...),
		"\n",
	)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	if logLevel > LOG_LEVEL_DEBUG {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			"[DEBUG]",
		),
		fmt.Sprintf(format, args...),
		"\n",
	)
}

// Printf logs a message at level Info on the standard logger.
func Printf(format string, args ...interface{}) {
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			"",
		),
		fmt.Sprintf(format, args...),
		"\n",
	)
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	if logLevel > LOG_LEVEL_INFO {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			blue.Sprint("[INFO]"),
		),
		fmt.Sprintf(format, args...),
		"\n",
	)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	if logLevel > LOG_LEVEL_WARN {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			yellow.Sprint("[WARN]"),
		),
		fmt.Sprintf(format, args...),
		"\n",
	)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	if logLevel > LOG_LEVEL_ERROR {
		return
	}
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			red.Sprint("[ERROR]"),
		),
		fmt.Sprintf(format, args...),
		"\n",
	)
}

// Fatalf logs a message at level Fatal on the standard logger then the process will exit with status set to 1.
//
// Fatalf is the highest level, it is printed at every log level (quiet mode
// included) and always exits.
func Fatalf(format string, args ...interface{}) {
	fmt.Print(
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			criticalRed.Sprint("[FATAL]"),
		),
		red.Sprintf(format, args...),
		"\n",
	)
	os.Exit(1)
}

// WarnStderr logs a message at level Warn on the standard error, it is used
// for warnings that must not end up in the data stream of a session written
// to the standard output.
func WarnStderr(args ...interface{}) {
	if logLevel > LOG_LEVEL_WARN {
		return
	}
	fmt.Fprint(
		os.Stderr,
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			yellow.Sprint("[WARN]"),
		),
		fmt.Sprint(args...),
		"\n",
	)
}

// Alert prints a security warning on the standard error regardless of the
// log level, quiet mode included, and does not exit.
//
// It is reserved for configurations that leave a session readable and
// changeable on the network path (--insecure-cert with --no-e2e). Any other
// use needs a design change, so that quiet mode stays meaningful.
func Alert(args ...interface{}) {
	fmt.Fprint(
		os.Stderr,
		fmt.Sprintf(
			LOGGER_PROMPT,
			faintWhite.Sprint(time.Now().Format("03:04:05.000")),
			criticalRed.Sprint("[!]"),
		),
		red.Sprint(args...),
		"\n",
	)
}
