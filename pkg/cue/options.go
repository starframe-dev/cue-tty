package cue

import (
	"time"
)

// Options configure an App launch.
type Options struct {
	Args        []string
	Env         []string
	EnvReplaced bool
	Dir         string
	Width       int
	Height      int
	Timeout     time.Duration
}

// DefaultOptions returns sensible defaults (80x24, no timeout).
func DefaultOptions() Options {
	return Options{
		Width:   80,
		Height:  24,
		Timeout: 0,
	}
}

// Option is a functional option for configuring launch.
type Option func(*Options)

// WithArgs sets command arguments.
func WithArgs(args ...string) Option {
	return func(o *Options) {
		o.Args = args
	}
}

// WithSize sets terminal dimensions.
func WithSize(cols, rows int) Option {
	return func(o *Options) {
		o.Width = cols
		o.Height = rows
	}
}

// WithEnv sets environment variables.
// Variables are appended to the current process environment unless
// WithReplaceEnv is used.
func WithEnv(env ...string) Option {
	return func(o *Options) {
		o.Env = env
	}
}

// WithReplaceEnv replaces the entire environment for the child process.
func WithReplaceEnv(env ...string) Option {
	return func(o *Options) {
		o.Env = env
		o.EnvReplaced = true
	}
}

// WithDir sets working directory.
func WithDir(dir string) Option {
	return func(o *Options) {
		o.Dir = dir
	}
}

// WithTimeout sets a total timeout for the app.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.Timeout = d
	}
}
