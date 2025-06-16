package goopt

import (
	"errors"
	"fmt"
	"testing"

	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
)

func TestExecutionHooks(t *testing.T) {
	t.Run("global pre-hook success", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre-hook")
			return nil
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		assert.Equal(t, []string{"pre-hook", "command"}, executed)
	})

	t.Run("global pre-hook prevents execution", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre-hook")
			return errors.New("auth failed")
		})

		parser.Parse([]string{"test"})
		errCount := parser.ExecuteCommands()

		assert.Equal(t, 1, errCount)
		assert.Equal(t, []string{"pre-hook"}, executed)

		err := parser.GetCommandExecutionError("test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auth failed")
	})

	t.Run("global post-hook runs after command", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, fmt.Sprintf("post-hook(err=%v)", err))
			return nil
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		assert.Equal(t, []string{"command", "post-hook(err=<nil>)"}, executed)
	})

	t.Run("post-hook runs even on command failure", func(t *testing.T) {
		var executed []string
		cmdErr := errors.New("command failed")

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return cmdErr
			},
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, fmt.Sprintf("post-hook(err=%v)", err))
			return nil
		})

		parser.Parse([]string{"test"})
		errors := parser.ExecuteCommands()

		assert.Equal(t, 1, errors)
		assert.Equal(t, []string{"command", "post-hook(err=command failed)"}, executed)
	})

	t.Run("post-hook runs even on pre-hook failure", func(t *testing.T) {
		var executed []string
		preErr := errors.New("pre-hook failed")

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre-hook")
			return preErr
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, fmt.Sprintf("post-hook(err=%v)", err))
			return nil
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		// Command should not run, but post-hook should
		assert.Equal(t, []string{"pre-hook", "post-hook(err=pre-hook failed)"}, executed)
	})

	t.Run("command-specific hooks", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test1",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "test1")
				return nil
			},
		})
		parser.AddCommand(&Command{
			Name: "test2",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "test2")
				return nil
			},
		})

		// Add hook only for test1
		parser.AddCommandPreHook("test1", func(p *Parser, c *Command) error {
			executed = append(executed, "pre-test1")
			return nil
		})

		parser.AddCommandPostHook("test1", func(p *Parser, c *Command, err error) error {
			executed = append(executed, "post-test1")
			return nil
		})

		// Execute test1
		executed = []string{}
		parser.Parse([]string{"test1"})
		parser.ExecuteCommands()
		assert.Equal(t, []string{"pre-test1", "test1", "post-test1"}, executed)

		// Execute test2 (no hooks)
		executed = []string{}
		parser.Parse([]string{"test2"})
		parser.ExecuteCommands()
		assert.Equal(t, []string{"test2"}, executed)
	})

	t.Run("multiple hooks execute in order", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		// Add multiple pre-hooks
		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre1")
			return nil
		})
		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre2")
			return nil
		})

		// Add multiple post-hooks
		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "post1")
			return nil
		})
		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "post2")
			return nil
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		assert.Equal(t, []string{"pre1", "pre2", "command", "post1", "post2"}, executed)
	})

	t.Run("hook order - global first", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.SetHookOrder(OrderGlobalFirst)

		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "global-pre")
			return nil
		})
		parser.AddCommandPreHook("test", func(p *Parser, c *Command) error {
			executed = append(executed, "cmd-pre")
			return nil
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "global-post")
			return nil
		})
		parser.AddCommandPostHook("test", func(p *Parser, c *Command, err error) error {
			executed = append(executed, "cmd-post")
			return nil
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		// Pre: global first, then command
		// Post: command first, then global (reverse for cleanup)
		assert.Equal(t, []string{"global-pre", "cmd-pre", "command", "cmd-post", "global-post"}, executed)
	})

	t.Run("hook order - command first", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.SetHookOrder(OrderCommandFirst)

		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "global-pre")
			return nil
		})
		parser.AddCommandPreHook("test", func(p *Parser, c *Command) error {
			executed = append(executed, "cmd-pre")
			return nil
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "global-post")
			return nil
		})
		parser.AddCommandPostHook("test", func(p *Parser, c *Command, err error) error {
			executed = append(executed, "cmd-post")
			return nil
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		// Pre: command first, then global
		// Post: global first, then command (reverse for cleanup)
		assert.Equal(t, []string{"cmd-pre", "global-pre", "command", "global-post", "cmd-post"}, executed)
	})

	t.Run("hooks have access to parser state", func(t *testing.T) {
		parser, _ := NewParserWith(
			WithFlag("verbose", NewArg(WithType(types.Standalone))),
		)

		var capturedVerbose bool
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			// Hook can read parser state
			if val, found := p.Get("verbose"); found && val == "true" {
				capturedVerbose = true
			}
			return nil
		})

		parser.Parse([]string{"--verbose", "test"})
		parser.ExecuteCommands()

		assert.True(t, capturedVerbose)
	})

	t.Run("hooks with nested commands", func(t *testing.T) {
		var executed []string

		parser := NewParser()

		// Create nested command structure
		serverCmd := &Command{
			Name: "server",
			Subcommands: []Command{
				{
					Name: "start",
					Callback: func(p *Parser, c *Command) error {
						executed = append(executed, "server-start")
						return nil
					},
				},
			},
		}
		parser.AddCommand(serverCmd)

		// Add hooks for nested command
		parser.AddCommandPreHook("server start", func(p *Parser, c *Command) error {
			executed = append(executed, "pre-server-start")
			assert.Equal(t, "server start", c.Path())
			return nil
		})

		parser.Parse([]string{"server", "start"})
		parser.ExecuteCommands()

		assert.Equal(t, []string{"pre-server-start", "server-start"}, executed)
	})

	t.Run("clear hooks", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		// Add hooks
		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "global-pre")
			return nil
		})
		parser.AddCommandPreHook("test", func(p *Parser, c *Command) error {
			executed = append(executed, "cmd-pre")
			return nil
		})

		// Clear global hooks
		parser.ClearGlobalHooks()

		executed = []string{}
		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		// Only command hook should run
		assert.Equal(t, []string{"cmd-pre", "command"}, executed)

		// Clear command hooks
		parser.ClearCommandHooks("test")

		executed = []string{}
		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		// No hooks should run
		assert.Equal(t, []string{"command"}, executed)
	})

	t.Run("with configuration functions", func(t *testing.T) {
		var executed []string

		parser, _ := NewParserWith(
			WithGlobalPreHook(func(p *Parser, c *Command) error {
				executed = append(executed, "global-pre")
				return nil
			}),
			WithGlobalPostHook(func(p *Parser, c *Command, err error) error {
				executed = append(executed, "global-post")
				return nil
			}),
			WithCommandPreHook("test", func(p *Parser, c *Command) error {
				executed = append(executed, "cmd-pre")
				return nil
			}),
			WithCommandPostHook("test", func(p *Parser, c *Command, err error) error {
				executed = append(executed, "cmd-post")
				return nil
			}),
			WithHookOrder(OrderCommandFirst),
		)

		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		assert.Equal(t, OrderCommandFirst, parser.GetHookOrder())
		assert.Equal(t, []string{"cmd-pre", "global-pre", "command", "global-post", "cmd-post"}, executed)
	})
}

func TestHookUseCases(t *testing.T) {
	t.Run("logging use case", func(t *testing.T) {
		var logs []string

		parser := NewParser()

		// Global logging hooks
		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			logs = append(logs, fmt.Sprintf("[START] %s", c.Path()))
			return nil
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			if err != nil {
				logs = append(logs, fmt.Sprintf("[ERROR] %s: %v", c.Path(), err))
			} else {
				logs = append(logs, fmt.Sprintf("[SUCCESS] %s", c.Path()))
			}
			return nil
		})

		// Commands
		parser.AddCommand(&Command{
			Name: "success",
			Callback: func(p *Parser, c *Command) error {
				return nil
			},
		})
		parser.AddCommand(&Command{
			Name: "fail",
			Callback: func(p *Parser, c *Command) error {
				return errors.New("operation failed")
			},
		})

		// Test success
		parser.Parse([]string{"success"})
		parser.ExecuteCommands()

		// Test failure
		parser.Parse([]string{"fail"})
		parser.ExecuteCommands()

		assert.Equal(t, []string{
			"[START] success",
			"[SUCCESS] success",
			"[START] fail",
			"[ERROR] fail: operation failed",
		}, logs)
	})

	t.Run("authentication use case", func(t *testing.T) {
		authenticated := false

		parser := NewParser()

		// Auth check hook
		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			// Skip auth for certain commands
			if c.Name == "login" {
				return nil
			}

			if !authenticated {
				return errors.New("not authenticated")
			}
			return nil
		})

		// Commands
		parser.AddCommand(&Command{
			Name: "login",
			Callback: func(p *Parser, c *Command) error {
				authenticated = true
				return nil
			},
		})
		parser.AddCommand(&Command{
			Name: "protected",
			Callback: func(p *Parser, c *Command) error {
				return nil
			},
		})

		// Try protected without auth
		parser.Parse([]string{"protected"})
		errs := parser.ExecuteCommands()
		assert.Equal(t, 1, errs)

		// Login
		parser.Parse([]string{"login"})
		errs = parser.ExecuteCommands()
		assert.Equal(t, 0, errs)

		// Now protected should work
		parser.Parse([]string{"protected"})
		errs = parser.ExecuteCommands()
		assert.Equal(t, 0, errs)
	})

	t.Run("cleanup use case", func(t *testing.T) {
		var resources []string

		parser := NewParser()

		// Cleanup hook always runs
		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			if len(resources) > 0 {
				// Clean up resources
				resources = []string{}
			}
			return nil
		})

		parser.AddCommand(&Command{
			Name: "allocate",
			Callback: func(p *Parser, c *Command) error {
				resources = append(resources, "resource1", "resource2")
				return errors.New("failed after allocation")
			},
		})

		parser.Parse([]string{"allocate"})
		parser.ExecuteCommands()

		// Resources should be cleaned up even though command failed
		assert.Empty(t, resources)
	})

	t.Run("metrics use case", func(t *testing.T) {
		type metric struct {
			command  string
			success  bool
			duration string
		}
		var metrics []metric

		parser := NewParser()

		// Track command execution
		startTimes := make(map[string]string)

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			startTimes[c.Path()] = "start"
			return nil
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			metrics = append(metrics, metric{
				command:  c.Path(),
				success:  err == nil,
				duration: "100ms", // Simulated
			})
			return nil
		})

		// Commands
		parser.AddCommand(&Command{
			Name: "fast",
			Callback: func(p *Parser, c *Command) error {
				return nil
			},
		})
		parser.AddCommand(&Command{
			Name: "slow",
			Callback: func(p *Parser, c *Command) error {
				return nil
			},
		})

		// Execute commands
		parser.Parse([]string{"fast"})
		parser.ExecuteCommands()

		parser.Parse([]string{"slow"})
		parser.ExecuteCommands()

		// Check metrics
		assert.Len(t, metrics, 2)
		assert.True(t, metrics[0].success)
		assert.True(t, metrics[1].success)
	})
}

func TestExecuteCommandWithHooks(t *testing.T) {
	t.Run("single command execution with hooks", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre")
			return nil
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "post")
			return nil
		})

		parser.Parse([]string{"test"})
		err := parser.ExecuteCommand()

		assert.NoError(t, err)
		assert.Equal(t, []string{"pre", "command", "post"}, executed)
	})
}

func TestHookErrorHandling(t *testing.T) {
	t.Run("post-hook error after successful command", func(t *testing.T) {
		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				return nil
			},
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			return errors.New("post-hook error")
		})

		parser.Parse([]string{"test"})
		errs := parser.ExecuteCommands()

		// Should count as error
		assert.Equal(t, 1, errs)

		err := parser.GetCommandExecutionError("test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "post-hook error")
	})

	t.Run("post-hook error after failed command", func(t *testing.T) {
		cmdErr := errors.New("command error")

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				return cmdErr
			},
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			return errors.New("post-hook error")
		})

		parser.Parse([]string{"test"})
		errs := parser.ExecuteCommands()

		// Should only count command error
		assert.Equal(t, 1, errs)

		// Command error should be preserved
		err := parser.GetCommandExecutionError("test")
		assert.Equal(t, cmdErr, err)
	})

	t.Run("chain stops on first pre-hook error", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre1")
			return nil
		})
		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre2")
			return errors.New("pre2 failed")
		})
		parser.AddGlobalPreHook(func(p *Parser, c *Command) error {
			executed = append(executed, "pre3")
			return nil
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		// Should stop at pre2
		assert.Equal(t, []string{"pre1", "pre2"}, executed)
	})

	t.Run("all post-hooks run even with errors", func(t *testing.T) {
		var executed []string

		parser := NewParser()
		parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = append(executed, "command")
				return nil
			},
		})

		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "post1")
			return errors.New("post1 error")
		})
		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "post2")
			return nil
		})
		parser.AddGlobalPostHook(func(p *Parser, c *Command, err error) error {
			executed = append(executed, "post3")
			return errors.New("post3 error")
		})

		parser.Parse([]string{"test"})
		parser.ExecuteCommands()

		// All post-hooks should run
		assert.Equal(t, []string{"command", "post1", "post2", "post3"}, executed)
	})
}

func TestHooksWithStructTags(t *testing.T) {
	t.Run("hooks with struct-based commands", func(t *testing.T) {
		var executed []string

		// Use regular command registration for this test
		parser := NewParser()

		// Create command structure
		serverCmd := &Command{
			Name:        "server",
			Description: "Server management",
			Subcommands: []Command{
				{
					Name:        "start",
					Description: "Start server",
					Callback: func(p *Parser, c *Command) error {
						executed = append(executed, "start")
						return nil
					},
				},
				{
					Name:        "stop",
					Description: "Stop server",
					Callback: func(p *Parser, c *Command) error {
						executed = append(executed, "stop")
						return nil
					},
				},
			},
		}
		parser.AddCommand(serverCmd)

		// Add hooks for specific commands
		parser.AddCommandPreHook("server start", func(p *Parser, c *Command) error {
			executed = append(executed, "pre-start")
			return nil
		})

		parser.AddCommandPreHook("server stop", func(p *Parser, c *Command) error {
			executed = append(executed, "pre-stop")
			return nil
		})

		// Test start
		executed = []string{}
		parser.Parse([]string{"server", "start"})
		parser.ExecuteCommands()
		assert.Equal(t, []string{"pre-start", "start"}, executed)

		// Test stop
		executed = []string{}
		parser.Parse([]string{"server", "stop"})
		parser.ExecuteCommands()
		assert.Equal(t, []string{"pre-stop", "stop"}, executed)
	})
}
