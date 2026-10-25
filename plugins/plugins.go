// Package plugins lists the compile-time extensions built into Pionus.
//
// To add a plugin:
//  1. implement plugin.Plugin (and optionally Initializer/Starter/Stopper and
//     events.Handler) in its own package;
//  2. add its factory to Factories below;
//  3. enable it in config.yaml under plugins.enabled.
//
// Plugins are trusted code and share the host process. There is no runtime
// .so loading or plugin installation.
package plugins

import (
	"github.com/pionus/pionus-go/internal/plugin"
	"github.com/pionus/pionus-go/plugins/eventlog"
)

// Factories returns the built-in plugin factories. The list is intentionally
// explicit so tests and production use the same startup order.
func Factories() []plugin.Factory {
	return []plugin.Factory{
		eventlog.New,
	}
}
