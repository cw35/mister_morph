// Package integration provides a reusable wiring layer for embedding
// mistermorph capabilities into third-party Go programs.
//
// It wires common features such as built-in tools, plan/todo helpers,
// guard, skills prompt loading, and request/prompt inspect dumps.
// Built-in tools can be narrowed with Config.BuiltinToolNames.
//
// Configuration is explicit via Config.Set(...) / Config.Overrides.
// The embedding host owns env/config-file loading and passes resolved values in.
//
// Note: this package currently uses the process-global Viper instance.
package integration
