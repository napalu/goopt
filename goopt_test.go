package goopt

import (
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/iancoleman/strcase"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type arrayWriter struct {
	data *[]string
}

func newArrayWriter() *arrayWriter {
	return &arrayWriter{data: &[]string{}}
}

func (writer arrayWriter) Write(p []byte) (int, error) {
	*writer.data = append(*writer.data, string(p))

	return len(p), nil
}

func TestCmdLineOption_AcceptPattern(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("test2", NewArgument("t2", "", Single, false, Secure{}, ""))

	err := opts.AcceptPattern("test2", PatternValue{Pattern: `^[0-9]+$`, Description: "whole integers only"})
	assert.Nil(t, err, "constraint violation - 'Single' flags take values and therefore should PatternValue")
	assert.True(t, opts.Parse([]string{"--test2", "12344"}), "test2 should accept values which match whole integer patterns")
}

func TestCmdLineOption_AcceptPatterns(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("test", NewArgument("t", "", Single, false, Secure{}, ""))

	err := opts.AcceptPatterns("test", []PatternValue{
		{Pattern: `^[0-9]+$`, Description: "whole integers"},
		{Pattern: `^[0-9]+\.[0-9]+`, Description: "float numbers"},
	})
	assert.Nil(t, err, "should accept multiple AcceptPatterns")
	assert.True(t, opts.Parse([]string{"--test", "12344"}), "test should accept values which match whole integer patterns")
	assert.True(t, opts.Parse([]string{"--test", "12344.123"}), "test should accept values which match float patterns")
	assert.False(t, opts.Parse([]string{"--test", "alphabet"}), "test should not accept alphabetical values")

	for _, err := range opts.GetErrors() {
		assert.Contains(t, err.Error(), "whole integers, float numbers", "the errors should include the accepted value pattern descriptions")
	}
}

func TestCmdLineOption_AddPreValidationFilter(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("upper", NewArgument("t", "", Single, false, Secure{}, ""))
	err := opts.AddFlagPreValidationFilter("upper", strings.ToUpper)
	assert.Nil(t, err, "should be able to add a filter to a valid flag")

	_ = opts.AcceptPattern("upper", PatternValue{Pattern: "^[A-Z]+$", Description: "upper case only"})
	assert.True(t, opts.HasPreValidationFilter("upper"), "flag should have a filter defined")
	assert.True(t, opts.Parse([]string{"--upper", "lowercase"}), "parse should not fail and pass PatternValue properly")

	value, _ := opts.Get("upper")
	assert.Equal(t, "LOWERCASE", value, "the value of flag upper should be transformed to uppercase")
}

func TestCmdLineOption_AddPostValidationFilter(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("status", NewArgument("t", "", Single, false, Secure{}, ""))
	err := opts.AddFlagPostValidationFilter("status", func(s string) string {
		if strings.EqualFold(s, "active") {
			return "-1"
		} else if strings.EqualFold(s, "inactive") {
			return "0"
		}

		return s
	})

	assert.Nil(t, err, "should be able to add a filter to a valid flag")

	_ = opts.AcceptPattern("status", PatternValue{Pattern: "^(?:active|inactive)$", Description: "set status to either 'active' or 'inactive'"})
	assert.True(t, opts.HasPostValidationFilter("status"), "flag should have a filter defined")
	assert.False(t, opts.Parse([]string{"--status", "invalid"}), "parse should fail on invalid input")
	opts.ClearAll()
	assert.True(t, opts.Parse([]string{"--status", "active"}), "parse should not fail and pass PatternValue properly")

	value, _ := opts.Get("status")
	assert.Equal(t, "-1", value, "the value of flag status should have been transformed to -1 after PatternValue validation")
}

func TestCmdLineOption_DependsOnFlagValue(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("main", NewArgument("m", "", Single, false, Secure{}, ""))
	_ = opts.AddFlag("dependent", NewArgument("d", "", Single, false, Secure{}, ""))

	err := opts.DependsOnFlagValue("dependent", "main", "qww1113394")
	assert.Nil(t, err, "should set dependent flag value")

	assert.True(t, opts.ParseString("-d test"), "should parse since all flags are optional")

	for _, wrn := range opts.GetWarnings() {
		assert.Contains(t, wrn, "depends on", "should warn of missing dependency")
	}

	_ = opts.ParseString("-d test -m not")
	for _, wrn := range opts.GetWarnings() {
		assert.Contains(t, wrn, "depends on",
			"should warn of missing dependency because the value of the dependent flag does not match the expected value")
		assert.Contains(t, wrn, "qww1113394", "should mention the expected value of the dependent variable")
	}

	err = opts.DependsOnFlagValue("d", "m", "aee12ew4eee")
	assert.Nil(t, err, "should set dependent value on short flag")

	_ = opts.ParseString("-d test -m not")
	for _, wrn := range opts.GetWarnings() {
		assert.Contains(t, wrn, "depends on", "should warn of missing dependency because the value of the dependent flag does not match one of the expected values")
		assert.Contains(t, wrn, "'qww1113394' or 'aee12ew4eee'", "should mention the expected values of the dependent variable")
	}

	_ = opts.ParseString("-d test -m aee12ew4eee")
	assert.Equal(t, len(opts.GetWarnings()), 0, "should not warn as the dependent variable has one of the expected values")
}

func TestCmdLineOption_AddCommand(t *testing.T) {
	opts := NewCmdLineOption()

	cmd := &Command{
		Name:        "",
		Subcommands: nil,
		Callback:    nil,
		Description: "",
	}

	err := opts.AddCommand(cmd)
	assert.NotNil(t, err, "should not be able to create a nameless command")

	cmd.Name = "create"
	cmd.Subcommands = []Command{{
		Name: "user",
		Subcommands: []Command{{
			Name: "",
		}},
	}}

	err = opts.AddCommand(cmd)
	assert.NotNil(t, err, "should not be able to create a nameless sub-command")

	cmd.Subcommands[0].Subcommands[0].Name = "type"
	err = opts.AddCommand(cmd)
	assert.Nil(t, err, "should be able to create a command with nested commands")
	assert.True(t, opts.ParseString("create user type author"), "should parse command with nested subcommands properly")
}

func TestCmdLineOption_RegisterCommand(t *testing.T) {
	opts := NewCmdLineOption()

	cmd := &Command{
		Name: "create",
		Subcommands: []Command{{
			Name: "user",
			Subcommands: []Command{{
				Name: "type",
			}},
		},
		},
	}

	err := opts.AddCommand(cmd)
	assert.Nil(t, err, "should properly add named command chain")
	err = opts.AddFlag("author", NewArg(
		WithShortFlag("a"),
		WithDescription("specify author"),
		WithType(Single)))
	assert.Nil(t, err, "should properly add named flag chain")
	assert.True(t, opts.ParseString("create user type --author john"), "should parse well-formed command")
	assert.True(t, opts.HasCommand("create"), "should properly register root command")
	assert.True(t, opts.HasCommand("create user"), "should properly register sub-command")
	assert.True(t, opts.HasCommand("create user type"), "should properly register nested sub-command")
	value, ok := opts.Get("author")
	assert.True(t, ok, "should find value of sub-command")
	assert.Equal(t, "john", value, "value of nested sub-command should be that supplied via command line")
}

func TestCmdLineOption_GetCommandValues(t *testing.T) {
	opts, _ := NewCmdLine(
		WithCommand(
			NewCommand(
				WithName("test"),
				WithCommandDescription("management commands"),
				WithSubcommands(
					NewCommand(
						WithName("blobs"),
						WithCommandDescription("blob commands"),
						WithSubcommands(
							NewCommand(
								WithName("copy"),
								WithCommandDescription("test blob"),
							),
						),
					),
					NewCommand(
						WithName("repos"),
						WithCommandDescription("repo commands"),
						WithSubcommands(
							NewCommand(
								WithName("copy"),
								WithCommandDescription("copy repo"),
							),
						),
					),
					NewCommand(
						WithName("roles"),
						WithCommandDescription("role commands"),
						WithSubcommands(
							NewCommand(
								WithName("copy"),
								WithCommandDescription("copy role"),
							),
						),
					),
				),
			)))

	// current behavior: last command overwrites a previous one with the same path - TODO check if this is the desired behaviour
	assert.True(t, opts.ParseString("test blobs copy test repos copy test roles copy test blobs copy blob_name"), "should parse well-formed commands")
	paths := opts.GetCommands()
	assert.Len(t, paths, 3, "should have parsed 3 commands")
	for i, path := range paths {
		switch i {
		case 0:
			assert.Equal(t, path, "test blobs copy")
		case 1:
			assert.Equal(t, path, "test repos copy")
		case 2:
			assert.Equal(t, path, "test roles copy")
		}

	}
}

func TestCmdLineOption_ValueCallback(t *testing.T) {
	opts := NewCmdLineOption()

	shouldBeEqualToOneAfterExecute := 0
	cmd := &Command{
		Name: "create",
		Subcommands: []Command{{
			Name: "user",
			Subcommands: []Command{{
				Name: "type",
				Callback: func(cmdLine *CmdLineOption, command *Command) error {
					shouldBeEqualToOneAfterExecute = 1
					return nil
				},
			}},
		}},
	}

	err := opts.AddCommand(cmd)
	assert.Nil(t, err, "should properly add named command chain")

	opts.ParseString("create user type author")
	assert.Zero(t, opts.ExecuteCommands(), "execute should return 0 errors")
	assert.Equal(t, 1, shouldBeEqualToOneAfterExecute, "should call subcommand callback after parse")
}

func TestCmdLineOption_WithBindFlag(t *testing.T) {
	var s string
	var i int
	var ts []string

	_, err := NewCmdLine(
		WithBindFlag("test", &ts,
			NewArg(
				WithShortFlag("t"),
				WithType(Single))))
	assert.Nil(t, err, "should not fail to bind pointer to supported slice variable to flag when using option functions")

	cmdLine, err := NewCmdLine(
		WithBindFlag("test", &s,
			NewArg(WithShortFlag("t"),
				WithType(Single))),
		WithBindFlag("test1", &i,
			NewArg(WithShortFlag("i"),
				WithType(Single))))

	assert.Nil(t, err, "should not fail to bind multiple pointer variables to flag when using option functions")
	assert.True(t, cmdLine.ParseString("--test value --test1 12334"), "should be able to parse an argument configured via option function")
	assert.Equal(t, "value", s, "should not fail to assign command line string argument to variable")
	assert.Equal(t, 12334, i, "should not fail to assign command line integer argument to variable")
}

func TestCmdLineOption_BindFlag(t *testing.T) {
	var s string
	var i int

	opts := NewCmdLineOption()
	err := opts.BindFlag(s, "test", NewArgument("t", "", Single, false, Secure{}, ""))
	assert.NotNil(t, err, "should not accept non-pointer type in BindFlag")

	err = opts.BindFlag(&s, "test", NewArgument("t", "", Single, false, Secure{}, ""))
	assert.Nil(t, err, "should accept string pointer type in BindFlag")

	err = opts.BindFlag(&i, "test1", NewArgument("t1", "", Single, false, Secure{}, ""))
	assert.Nil(t, err, "should accept int pointer type in BindFlag")

	assert.True(t, opts.ParseString("--test \"hello world\" --test1 12334"),
		"should parse a command line argument when given a bound variable")
	assert.Equal(t, s, "hello world", "should set value of bound variable when parsing")
	assert.Equal(t, 12334, i, "should not fail to assign command line integer argument to variable")

	type tt struct {
		testStr string
		testInt int
	}

	err = opts.BindFlag(&tt{
		testStr: "",
		testInt: 0,
	}, "test1", NewArgument("t1", "", Single, false, Secure{}, ""))
	assert.NotNil(t, err, "should not attempt to bind unsupported struct")

	assert.True(t, opts.ParseString("--test1 2"), "should parse a command line argument when given a bound variable")

	opts = NewCmdLineOption()
	var boolBind bool
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.Nil(t, err, "should accept Standalone flags in BindFlag if the data type is boolean")

	opts = NewCmdLineOption()
	err = opts.BindFlag(&i, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.NotNil(t, err, "should not accept Standalone flags in BindFlag if the data type is not boolean")
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.Nil(t, err, "should allow adding field if not yet specified")
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.NotNil(t, err, "should error when adding duplicate field")
}

func TestCmdLineOption_FileFlag(t *testing.T) {
	var s string
	cmdLine, err := NewCmdLine(
		WithBindFlag("test", &s,
			NewArg(WithShortFlag("t"),
				WithType(File))))
	assert.Nil(t, err, "should not fail to bind pointer to file flag")
	tempDir, err := os.MkdirTemp("", "*")
	assert.Nil(t, err)
	defer os.RemoveAll(tempDir)
	temp, err := os.CreateTemp(tempDir, "*")
	assert.Nil(t, err)
	_, err = temp.WriteString("test_value_123")
	assert.Nil(t, err)
	name := temp.Name()
	err = temp.Sync()
	assert.Nil(t, err)
	err = temp.Close()
	assert.Nil(t, err)
	assert.NotEmpty(t, name)
	localArg := fmt.Sprintf(`--test "%s"`, name)
	result := cmdLine.ParseString(localArg)
	assert.True(t, result, "should be able to parse a File argument")
	assert.Equal(t, "test_value_123", s)
	assert.Nil(t, cmdLine.SetFlag("test", "one234"), "should be able to set the value of a File flag")
	assert.Equal(t, "one234", cmdLine.GetOrDefault("test", ""))
	fileVal, err := os.ReadFile(name)
	assert.Nil(t, err)
	assert.Equal(t, "one234", string(fileVal), "should correctly set the underlying value")
}

type TestOptOk struct {
	IsTest       bool   `long:"isTest" short:"t" description:"test bool option" required:"true" type:"standalone" path:"create user type,create group type"`
	IntOption    int    `short:"i" description:"test int option" default:"-20"`
	StringOption string `short:"so" description:"test string option" type:"single" default:"1"`
}

type TestOptNok struct {
	IsTest bool
}

func TestCmdLineOption_NewCmdLineFromStruct(t *testing.T) {
	testOpt := TestOptOk{}
	cmd, err := NewCmdLineFromStruct(&testOpt)
	assert.Nil(t, err)
	if err == nil {
		assert.False(t, cmd.ParseString("-t true --stringOption one"), "parse should fail when a command-specific flag is required but no associated command is specified")
		cmd.ClearAll()
		assert.True(t, cmd.ParseString("create user type -t --stringOption one"), "parse should success when a command-specific flag is given and the associated command is specified")
		assert.Equal(t, true, testOpt.IsTest, "test bool option should be true")
		assert.Equal(t, "one", testOpt.StringOption, "should set value of StringOption")
		assert.Equal(t, "one", cmd.GetOrDefault("stringOption", ""),
			"should be able to reference by long name when long name is not explicitly set")
	}
	_, err = NewCmdLineFromStruct(&TestOptNok{})
	assert.NotNil(t, err, "should error out on invalid struct")
	cmd, err = NewCmdLineFromStruct(&testOpt)
	assert.Nil(t, err)
	assert.True(t, cmd.ParseString("create user type create group type -t --stringOption"))
	assert.Equal(t, true, testOpt.IsTest, "test bool option should be true when multiple commands share same flag")
	assert.Equal(t, "1", testOpt.StringOption, "should use default value if flag is parsed without a default value")
	assert.Equal(t, -20, testOpt.IntOption, "should use default value if not required and not defined and not set on command line")
}

type Address struct {
	City    string `long:"city" description:"City name" typeOf:"Single"`
	ZipCode string `long:"zipcode" description:"ZIP code" typeOf:"Single"`
}

type UserProfile struct {
	Name      string    `long:"name" short:"n" description:"Full name" typeOf:"Single"`
	Age       int       `long:"age" short:"a" description:"Age of user" typeOf:"Single"`
	Addresses []Address `long:"address"`
}

func TestCmdLineOption_NewCmdLineRecursion(t *testing.T) {
	profile := &UserProfile{
		Addresses: make([]Address, 1),
	}

	cmd, err := NewCmdLineFromStruct(profile)
	assert.Nil(t, err, "should handle nested structs")
	assert.True(t, cmd.ParseString("--name Jack -a 10 --address.0.city 'New York'"), "should parse nested arguments")
	assert.Equal(t, "Jack", cmd.GetOrDefault("name", ""))
	assert.Equal(t, "New York", cmd.GetOrDefault("address.0.city", ""))
	assert.Equal(t, "10", cmd.GetOrDefault("age", ""))
}

func TestCmdLineOption_CommandSpecificFlags(t *testing.T) {
	opts := NewCmdLineOption()

	// Define commands and associated flags
	_ = opts.AddCommand(&Command{
		Name: "create",
		Subcommands: []Command{
			{
				Name: "user",
				Subcommands: []Command{
					{
						Name: "type",
					},
				},
			},
		},
	})

	err := opts.AddFlag("username", &Argument{
		Description: "Username for user creation",
		TypeOf:      Single,
		Short:       "u",
	}, "create user type")
	assert.Nil(t, err, "should properly associate flag with command Path")

	err = opts.AddFlag("email", &Argument{
		Description: "Email for user creation",
		TypeOf:      Single,
	}, "create user")
	assert.Nil(t, err, "should properly associate flag with user subcommand")

	// Test with valid command and flag
	assert.True(t, opts.ParseString("create user type --username john_doe"), "should parse flag associated with specific command Path")
	assert.Equal(t, "john_doe", opts.GetOrDefault("username", "", "create user type"), "flag should be parsed correctly for its command Path")

	// Test with missing command flag
	assert.False(t, opts.ParseString("create user --username john_doe"), "should not parse flag not associated with the command Path")
}

func TestCmdLineOption_GlobalFlags(t *testing.T) {
	opts := NewCmdLineOption()

	// Define commands
	_ = opts.AddCommand(&Command{
		Name: "create",
		Subcommands: []Command{
			{
				Name: "user",
				Subcommands: []Command{
					{
						Name: "type",
					},
				},
			},
		},
	})

	verbose := false
	// Add a global flag
	err := opts.BindFlag(&verbose, "verbose", &Argument{
		Description: "Enable verbose logging",
		TypeOf:      Standalone,
	})
	assert.Nil(t, err, "should properly add global flag")

	// Test global flag with command
	assert.True(t, opts.ParseString("create user type --verbose"), "should parse global flag with any command")
	assert.True(t, opts.HasFlag("verbose"), "global flag should be recognized")
}

func TestCmdLineOption_SharedFlags(t *testing.T) {
	opts := NewCmdLineOption()

	// Define commands
	_ = opts.AddCommand(&Command{
		Name: "create",
		Subcommands: []Command{
			{
				Name: "user",
				Subcommands: []Command{
					{
						Name: "type",
					},
				},
			},
			{
				Name: "group",
			},
		},
	})

	// Shared flag for both commands
	err := opts.AddFlag("sharedFlag", &Argument{
		Description: "Shared flag for user creation",
		TypeOf:      Single,
	}, "create user type")
	assert.Nil(t, err, "should properly add shared flag to user creation command")

	err = opts.AddFlag("sharedFlag", &Argument{
		Description: "Shared flag for group creation",
		TypeOf:      Single,
	}, "create group")
	assert.Nil(t, err, "should properly add shared flag to group creation command")

	// Test flag in both paths
	assert.True(t, opts.ParseString("create user type --sharedFlag user_value"), "should parse flag in user command")
	assert.Equal(t, "user_value", opts.GetOrDefault("sharedFlag", "", "create user type"), "flag should be parsed correctly in user command")

	assert.True(t, opts.ParseString("create group --sharedFlag group_value"), "should parse flag in group command")
	assert.Equal(t, "group_value", opts.GetOrDefault("sharedFlag", "", "create group"), "flag should be parsed correctly in group command")
}

func TestCmdLineOption_EnvToFlag(t *testing.T) {
	var s string
	cmdLine, err := NewCmdLine(
		WithCommand(NewCommand(WithName("command"), WithSubcommands(NewCommand(WithName("test"))))),
		WithBindFlag("testMe", &s,
			NewArg(WithShortFlag("t"),
				WithType(Single))))
	assert.Nil(t, err)
	flagFunc := cmdLine.SetEnvFilter(func(s string) string {
		return upperSnakeToCamelCase(s)
	})
	assert.Nil(t, flagFunc, "flagFunc should be nil when none is set")
	os.Setenv("TEST_ME", "test")

	assert.True(t, cmdLine.ParseString("command test --testMe 123"))
	assert.Equal(t, "123", s)
	assert.True(t, cmdLine.HasCommand("command test"))
	assert.True(t, cmdLine.ParseString("command test"))
	assert.Equal(t, "test", s)
	assert.True(t, cmdLine.HasCommand("command test"))

	cmdLine, err = NewCmdLine(
		WithCommand(NewCommand(WithName("command"), WithSubcommands(NewCommand(WithName("test"))))),
		WithBindFlag("testMe", &s,
			NewArg(WithShortFlag("t"),
				WithType(Single)), "command test"))
	assert.Nil(t, err)
	flagFunc = cmdLine.SetEnvFilter(func(s string) string {
		return upperSnakeToCamelCase(s)
	})

	assert.True(t, cmdLine.ParseString("command test --testMe 123"))

	assert.Equal(t, "123", s)
	assert.True(t, cmdLine.HasCommand("command test"))
	assert.True(t, cmdLine.ParseString("command test"))
	assert.Equal(t, "test", s)
	assert.True(t, cmdLine.HasCommand("command test"))

}

func TestCmdLineOption_GlobalAndCommandEnvVars(t *testing.T) {
	opts := NewCmdLineOption()

	// Define commands
	_ = opts.AddCommand(&Command{Name: "create"})
	_ = opts.AddCommand(&Command{Name: "update"})

	// Define flags for global and specific commands
	_ = opts.AddFlag("verboseTest", &Argument{Description: "Verbose output", TypeOf: Standalone})
	_ = opts.AddFlag("configTest", &Argument{Description: "Config file", TypeOf: Single}, "create")
	_ = opts.AddFlag("idTest", &Argument{Description: "User ID", TypeOf: Single}, "create")
	flagFunc := opts.SetEnvFilter(func(s string) string {
		return upperSnakeToCamelCase(s)
	})
	assert.Nil(t, flagFunc, "flagFunc should be nil when none is set")
	// Simulate environment variables
	os.Setenv("VERBOSE_TEST", "true")
	os.Setenv("CONFIG_TEST", "/path/to/config")
	os.Setenv("ID_TEST", "1234")

	assert.True(t, opts.ParseString("create"), "should parse command with global and env flags")

	// Check global flag
	assert.Equal(t, "true", opts.GetOrDefault("verboseTest", ""), "global verbose should be true")

	// Check command-specific flags
	assert.Equal(t, "/path/to/config", opts.GetOrDefault("configTest@create", ""), "config flag should be parsed from env var")
	assert.Equal(t, "1234", opts.GetOrDefault("idTest@create", ""), "ID should be parsed from env var")
}

func TestCmdLineOption_MixedFlagsWithEnvVars(t *testing.T) {
	opts := NewCmdLineOption()

	// Define commands
	_ = opts.AddCommand(&Command{Name: "delete"})
	_ = opts.AddCommand(&Command{Name: "create"})

	// Define flags for global and specific commands
	_ = opts.AddFlag("verbose", &Argument{Description: "Verbose output", TypeOf: Standalone})
	_ = opts.AddFlag("config", &Argument{Description: "Config file", TypeOf: Single}, "create")
	_ = opts.AddFlag("force", &Argument{Description: "Force deletion", TypeOf: Standalone}, "delete")
	flagFunc := opts.SetEnvFilter(func(s string) string {
		return upperSnakeToCamelCase(s)
	})
	assert.Nil(t, flagFunc, "flagFunc should be nil when none is set")
	// Simulate environment variables
	os.Setenv("VERBOSE", "true")
	os.Setenv("FORCE", "true")

	// Mixed commands with flags
	assert.True(t, opts.ParseString("delete create --config /my/config"), "should parse mixed commands with flags")

	// Check global flag
	assert.Equal(t, "true", opts.GetOrDefault("verbose", ""), "global verbose should be true")

	// Check command-specific flags
	assert.Equal(t, "true", opts.GetOrDefault("force@delete", ""), "force flag should be true from env var")
	assert.Equal(t, "/my/config", opts.GetOrDefault("config@create", ""), "config flag should be parsed from command")
}

func TestCmdLineOption_RepeatCommandWithDifferentContextWithCallbacks(t *testing.T) {
	opts := NewCmdLineOption()
	opts.SetExecOnParse(true)
	idx := 0

	_ = opts.AddCommand(&Command{Name: "create", Callback: func(cmdLine *CmdLineOption, command *Command) error {
		assert.True(t, cmdLine.HasFlag("id", command.Path))
		assert.True(t, cmdLine.HasFlag("group", command.Path))
		if idx == 0 {
			assert.Equal(t, "1", cmdLine.GetOrDefault("id", "", command.Path))
			assert.Equal(t, "3", cmdLine.GetOrDefault("group", "", command.Path))

		} else if idx == 1 {
			assert.Equal(t, "2", cmdLine.GetOrDefault("id", "", command.Path))
			assert.Equal(t, "4", cmdLine.GetOrDefault("group", "", command.Path))
			assert.Equal(t, "Mike", cmdLine.GetOrDefault("name", "", command.Path))
		}

		idx++

		return nil
	}})

	// Define flags for specific commands
	_ = opts.AddFlag("id", &Argument{Description: "User ID", TypeOf: Single}, "create")
	_ = opts.AddFlag("group", &Argument{Description: "Group ID", TypeOf: Single}, "create")
	_ = opts.AddFlag("name", &Argument{Description: "User Name", TypeOf: Single}, "create")

	// Simulate repeated commands with flags
	assert.True(t, opts.ParseString("create --id 1 --group 3 create --id 2 --group 4 --name Mike"), "should parse repeated commands with different contexts")

	assert.Equal(t, "Mike", opts.GetOrDefault("name@create", ""), "name flag should be 'Mike'")
}

func upperSnakeToCamelCase(s string) string {
	return strcase.ToLowerCamel(s)
}

func TestParse_PosixFlagsWithEnvVars(t *testing.T) {
	opts := NewCmdLineOption()
	opts.SetPosix(true)

	// Define commands and flags
	_ = opts.AddCommand(&Command{Name: "build"})
	_ = opts.AddFlag("output", &Argument{Description: "Output file", Short: "o", TypeOf: Single}, "build")
	_ = opts.AddFlag("opt", &Argument{Description: "Optimization level", Short: "p", TypeOf: Single}, "build")
	flagFunc := opts.SetEnvFilter(func(s string) string {
		return upperSnakeToCamelCase(s)
	})
	assert.Nil(t, flagFunc, "flagFunc should be nil when none is set")
	// Simulate environment variables
	os.Setenv("OUTPUT", "/build/output")

	assert.True(t, opts.ParseString("build -p2"), "should handle posix-style flags")
	assert.Equal(t, "/build/output", opts.GetOrDefault("output", "", "build"), "output flag should be 'mybuild'")
	assert.Equal(t, "2", opts.GetOrDefault("opt", "", "build"), "optimization should be '2' from env var")
}

func TestCmdLineOption_VarInFileFlag(t *testing.T) {
	uname := fmt.Sprintf("%x", md5.Sum([]byte(time.Now().Format(time.RFC3339Nano))))
	var s string
	cmdLine, err := NewCmdLine(
		WithBindFlag("test", &s,
			NewArg(WithShortFlag("t"),
				WithType(File),
				WithDefaultValue(filepath.Join("${EXEC_DIR}", uname, "test")))))
	assert.Nil(t, err, "should not fail to bind pointer to file flag")
	execPath, err := os.Executable()
	assert.Nil(t, err)
	execDir := filepath.Dir(execPath)

	fp := filepath.Join("${EXEC_DIR}", uname, "test")
	err = os.Mkdir(filepath.Join(execDir, uname), 0755)
	assert.Nil(t, err)
	err = os.WriteFile(filepath.Join(execDir, uname, "test"), []byte("test123"), 0755)
	assert.Nil(t, err)
	assert.True(t, cmdLine.ParseString(fmt.Sprintf("--test %s", fp)), "should parse file flag with var")
	assert.Equal(t, "test123", s)
	s = ""
	assert.True(t, cmdLine.ParseString("--test"), "should parse file flag with var with default value ")
	assert.Equal(t, "test123", s)
	os.RemoveAll(filepath.Join(execDir, uname))
}

func TestNewCmdLineOption_BindNil(t *testing.T) {
	opts := NewCmdLineOption()

	type tester struct {
		TestStr string
	}

	var test *tester
	err := opts.CustomBindFlag(test, func(flag, value string, customStruct interface{}) {

	}, "test1", NewArgument("t1", "", Single, false, Secure{}, ""))

	assert.NotNil(t, err, "should not be able to custom bind a nil pointer")
}

func TestCmdLineOption_CustomBindFlag(t *testing.T) {
	type tester struct {
		TestStr string
		testInt int
	}

	opts := NewCmdLineOption()
	customType := &tester{
		TestStr: "3",
		testInt: 0,
	}

	err := opts.CustomBindFlag(customType, func(flag, value string, customStruct interface{}) {
		assert.Equal(t, "test1", flag, "should receive the name of the parsed flag")
		assert.Equal(t, "2", value, "should receive the value of the parsed flag")
		assert.Equal(t, "3", customStruct.(*tester).TestStr, "customStruct should point to customType")
		customStruct.(*tester).TestStr = "2"
		assert.Equal(t, "2", customType.TestStr, "changing the field value of the reflected type should change the field in the underlying type")
	}, "test1", NewArgument("t1", "", Single, false, Secure{}, ""))
	assert.Nil(t, err, "should be able to bind custom flag")

	assert.True(t, opts.ParseString("--test1 2"), "should parse a command line argument when given a bound variable")
}

func TestCmdLineOption_WithCustomBindFlag(t *testing.T) {
	type tester struct {
		TestStr string
		testInt int
	}

	var test tester
	cmdLine, err := NewCmdLine(
		WithCustomBindFlag("test1", &tester{
			TestStr: "20",
			testInt: 123344,
		},
			func(flag, value string, customStruct interface{}) {
				assert.Equal(t, "test1", flag, "should receive the name of the parsed flag")
				assert.Equal(t, "22330", value, "should receive the value of the parsed flag")
				assert.Equal(t, "20", customStruct.(*tester).TestStr, "customStruct should point to anonymous type")
				customStruct.(*tester).TestStr = value
				test = *customStruct.(*tester)
			},
			NewArg(
				WithShortFlag("t"),
				WithType(Single))))
	assert.Nil(t, err, "should accept custom bind flag")

	assert.True(t, cmdLine.ParseString("--test1 22330"), "should parse a command line argument when given a bound variable")
	assert.Equal(t, "22330", test.TestStr, "should be able to reference anonymous type through interface assignment in callback")
	assert.Equal(t, 123344, test.testInt, "should be able to reference anonymous type through interface assignment in callback")
}

func TestCmdLineOption_Parsing(t *testing.T) {
	cmdLine, err := NewCmdLine(
		WithFlag("flagWithValue",
			NewArg(
				WithShortFlag("fw"),
				WithType(Single),
				WithDescription("this flag requires a value"),
				WithDependentFlags([]string{"flagA", "flagB"}),
				SetRequired(true))),
		WithFlag("flagA",
			NewArg(
				WithShortFlag("fa"),
				WithType(Standalone))),
		WithFlag("flagB",
			NewArg(
				WithShortFlag("fb"),
				WithDescription("This is flag B - flagWithValue depends on it"),
				WithDefaultValue("db"),
				WithType(Single))),
		WithFlag("flagC",
			NewArg(
				WithShortFlag("fc"),
				WithDescription("this is flag C - it's a chained flag which can return a list"),
				WithType(Chained))),
		WithCommand(&Command{
			Name: "create",
			Subcommands: []Command{
				{
					Name: "user",
					Subcommands: []Command{
						{
							Name: "type",
						},
					},
				},
				{
					Name: "group",
					Subcommands: []Command{
						{
							Name: "member",
						},
					},
				},
			},
		}),
	)
	assert.Nil(t, err, "fflag composition should work")

	assert.True(t, cmdLine.ParseString(`--flagWithValue 
		"test value" --fa --flagB 
--flagC "1|2|3" create user type
 create group member`), "command line options should be passed correctly")

	assert.True(t, cmdLine.HasCommand("create user type"), "should find command")
	assert.False(t, cmdLine.HasCommand("create nil type"), "should find not command with incorrect Path")
	assert.True(t, cmdLine.HasCommand("create group member"), "should find all subcommands")

	list, err := cmdLine.GetList("flagC")
	assert.Nil(t, err, "chained flag should return a list")
	assert.Equal(t, []string{"1", "2", "3"}, list)

	val, found := cmdLine.Get("flagB")
	assert.True(t, found, "flagB was supplied on command line we expect it to be err")
	assert.Equal(t, "db", val, "flagB was specified on command line but no value was given,"+
		" we expect it to have the default value")

	warnings := cmdLine.GetWarnings()
	assert.Len(t, warnings, 0, "no warnings were expected all options were supplied")

	allCommands := cmdLine.GetCommands()
	assert.Len(t, allCommands, 2)
	// reset parsed options and commands to parse again
	cmdLine.ClearAll()

	if !cmdLine.ParseString(`--flagWithValue 
		"test value" create user type
 author --fa
--flagC "1|2|3" `,
	) {
		t.Errorf("command line options should be passed correctly")
	}

	warnings = cmdLine.GetWarnings()
	assert.Len(t, warnings, 1, "we expect 1 warning: flagWithValue lists flagB as dependency"+
		" but flagB was not specified on command line")

	val, found = cmdLine.Get("flagWithValue")
	assert.True(t, found, "flagWithValue was supplied on command line we expect it to be err")
	assert.Equal(t, "test value", val, "flagWithValue was specified on command line and a value was given,"+
		" we expect it to have the given value")
}

func TestCmdLineOption_PrintUsage(t *testing.T) {
	opts := NewCmdLineOption()

	err := opts.AddCommand(&Command{
		Name: "create",
		Subcommands: []Command{
			{
				Name: "user",
				Subcommands: []Command{
					{
						Name: "type",
						Subcommands: []Command{{
							Name: "wacky1",
						}},
					},
					{
						Name: "alias",
						Subcommands: []Command{{
							Name: "wacky2",
							Subcommands: []Command{{
								Name: "wacky7",
								Subcommands: []Command{{
									Name:        "wacky8",
									Description: "wacky8 expects a user value on the command line",
								}, {
									Name: "wacky9",
									Subcommands: []Command{{
										Name: "wacky10",
									}},
								}},
							}},
						}},
					},
				},
			},
			{
				Name: "group",
				Subcommands: []Command{
					{
						Name: "type",
						Subcommands: []Command{{
							Name: "wacky3",
						}},
					},
					{
						Name: "alias",
						Subcommands: []Command{{
							Name: "wacky4",
						}},
					},
				},
			},
			{
				Name: "computer",
				Subcommands: []Command{
					{
						Name: "type",
						Subcommands: []Command{{
							Name: "wacky5",
						}},
					},
					{
						Name: "alias",
						Subcommands: []Command{{
							Name: "wacky6",
						}},
					},
				},
			},
		},
	})
	assert.Nil(t, err, "should be able to create command")

	err = opts.AddCommand(&Command{Name: "create1", Subcommands: []Command{{Name: "user1"}}})
	assert.Nil(t, err, "should be able to add command")

	writer := newArrayWriter()
	opts.PrintCommands(writer)

	assert.Len(t, *writer.data, 22, "PrintCommands in this test should return 22 elements")
	assert.Contains(t, *writer.data, " └───── wacky8 \"wacky8 expects a user value on the command line\"\n")
	assert.Contains(t, *writer.data, " │───── wacky9 \"\"\n")
	assert.Contains(t, *writer.data, " └────── wacky10 \"\"\n")

}

func TestPosixCompatibleFlags(t *testing.T) {
	opts := NewCmdLineOption()
	opts.SetPosix(true)
	err := opts.AddFlag("alongflag", &Argument{
		Short:       "a",
		Description: "short flag a",
		TypeOf:      Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("alsolong", &Argument{
		Short:       "b",
		Description: "short flag b",
		TypeOf:      Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("boolFlag", &Argument{
		Short:       "c",
		Description: "short flag c",
		TypeOf:      Standalone,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("anotherlong", &Argument{
		Short:       "d",
		Description: "short flag d",
		TypeOf:      Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("yetanotherlong", &Argument{
		Short:       "e",
		Description: "short flag e",
		TypeOf:      Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("badoption", &Argument{
		Short:       "ab",
		Description: "posix incompatible flag",
		TypeOf:      Single,
	})
	assert.True(t, errors.Is(err, ErrPosixIncompatible))
	err = opts.AddFlag("listFlag", &Argument{
		Short:       "f",
		Description: "list",
		TypeOf:      Chained,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("tee", &Argument{
		Short:       "t",
		Description: "tee for 2",
		TypeOf:      Single,
	})
	assert.Nil(t, err)

	assert.True(t, opts.ParseString("-t23 -a23cb1233 -d 3 -e2 -f\"1,2,3,on\""))
	assert.Len(t, opts.GetErrors(), 0)

	valA := opts.GetOrDefault("a", "")
	valB := opts.GetOrDefault("b", "")
	valC, _ := opts.GetBool("c")
	valD := opts.GetOrDefault("d", "")
	valE := opts.GetOrDefault("e", "")
	valF, _ := opts.GetList("f")
	valT := opts.GetOrDefault("t", "")

	assert.Equal(t, "23", opts.GetOrDefault("a", valA))
	assert.Equal(t, "1233", opts.GetOrDefault("b", valB))
	assert.Equal(t, true, valC)
	assert.Equal(t, "3", valD)
	assert.Equal(t, "2", valE)
	assert.Equal(t, []string{"1", "2", "3", "on"}, valF)
	assert.Equal(t, "23", valT)
}

func TestCmdLineOption_FunctionOptionConfigurationOfCommands(t *testing.T) {
	opts := NewCmdLineOption()

	var valueReceived string
	err := opts.AddCommand(
		NewCommand(WithName("create"),
			WithCommandDescription("create family of commands"),
			WithSubcommands(
				NewCommand(
					WithName("user"),
					WithCommandDescription("create user"),
					WithCallback(
						func(cmdLine *CmdLineOption, command *Command) error {
							valueReceived = command.Name

							return nil
						}),
				),
				NewCommand(
					WithName("group"),
					WithCommandDescription("create group")),
			)))
	assert.Nil(t, err, "should be able to add commands")

	assert.True(t, opts.ParseString("create user test create group test2"), "should be able to parse commands")

	assert.Equal(t, "", valueReceived, "command callback should not be called before execute")
	assert.Equal(t, 0, opts.ExecuteCommands(), "command callback error should be nil if no error occurred")
	assert.Equal(t, "user", valueReceived, "command callback should be called on execute")
	assert.Equal(t, 2, opts.GetPositionalArgCount(), "values which are neither commands nor flags should be assessed as positional arguments")
	posArgs := opts.GetPositionalArgs()
	assert.Equal(t, "test", posArgs[0].Value)
	// positional arguments are 0-based
	assert.Equal(t, 2, posArgs[0].Position)
	assert.Equal(t, "test2", posArgs[1].Value)
	// positional arguments are 0-based
	assert.Equal(t, 5, posArgs[1].Position)

}

func TestCmdLineOption_ParseWithDefaults(t *testing.T) {
	defaults := map[string]string{"flagWithValue": "valueA"}

	cmdLine, _ := NewCmdLine(
		WithFlag("flagWithValue",
			NewArg(
				WithShortFlag("fw"),
				WithType(Single),
				WithDescription("this flag requires a value"),
				WithDependentFlags([]string{"flagA", "flagB"}),
				SetRequired(true))),
		WithFlag("flagA",
			NewArg(
				WithShortFlag("fa"),
				WithType(Standalone))),
		WithFlag("flagB",
			NewArg(
				WithShortFlag("fb"),
				WithDescription("This is flag B - flagWithValue depends on it"),
				WithDefaultValue("db"),
				WithType(Single))),
		WithFlag("flagC",
			NewArg(
				WithShortFlag("fc"),
				WithDescription("this is flag C - it's a chained flag which can return a list"),
				WithType(Chained))))

	assert.True(t, cmdLine.ParseStringWithDefaults(defaults, "-fa -fb"), "required value should be set by default")
	assert.Equal(t, cmdLine.GetOrDefault("flagWithValue", ""), "valueA", "value should be supplied by default")

}

func TestCmdLineOption_StandaloneFlagWithExplicitValue(t *testing.T) {
	cmdLine, _ := NewCmdLine(
		WithFlag("flagA",
			NewArg(
				WithShortFlag("fa"),
				WithType(Standalone))),
		WithFlag("flagB",
			NewArg(
				WithShortFlag("fb"),
				WithType(Single))))

	assert.True(t, cmdLine.ParseString("-fa false -fb hello"), "should properly parse a command-line with explicitly "+
		"set boolean flag value among other values")
	boolValue, err := cmdLine.GetBool("fa")
	assert.Nil(t, err, "boolean conversion of 'false' string value should not result in error")
	assert.False(t, boolValue, "the user-supplied false value of a boolean flag should be respected")
	assert.Equal(t, cmdLine.GetOrDefault("fb", ""), "hello", "Single flag in command-line "+
		"with explicitly set boolean flag should have the correct value")
	cmdLine.ClearAll()
	assert.False(t, cmdLine.ParseString("-fa ouch -fb hello"), "should not properly parse a command-line with explicitly "+
		"set invalid boolean flag value among other values")
	_, err = cmdLine.GetBool("fa")
	assert.NotNil(t, err, "boolean conversion of non-boolean string value should result in error")
}

func TestCmdLineOption_PrintUsageWithGroups(t *testing.T) {
	opts := NewCmdLineOption()

	// Add global flags
	err := opts.AddFlag("help", &Argument{
		Description: "Display help",
		TypeOf:      Standalone,
	})
	assert.Nil(t, err, "should add global flag successfully")

	// Add commands
	cmd := &Command{
		Name:        "create",
		Description: "Create resources",
		Subcommands: []Command{
			{
				Name:        "user",
				Description: "Manage users",
				Subcommands: []Command{
					{
						Name:        "type",
						Description: "Specify user type",
					},
				},
			},
			{
				Name:        "group",
				Description: "Manage groups",
			},
		},
	}
	err = opts.AddCommand(cmd)
	assert.Nil(t, err, "should add commands successfully")

	// Add command-specific flags
	err = opts.AddFlag("username", &Argument{
		Description: "Username for user creation",
		TypeOf:      Single,
		Required:    true,
	}, "create user type")
	assert.Nil(t, err, "should add command-specific flag successfully")
	err = opts.AddFlag("firstName", &Argument{
		Description: "User first name",
		TypeOf:      Single,
	}, "create user type")
	assert.Nil(t, err, "should add command-specific flag successfully")

	err = opts.AddFlag("email", &Argument{
		Description: "Email for user creation",
		Short:       "e",
		TypeOf:      Single,
	}, "create user")
	assert.Nil(t, err, "should add command-specific flag successfully")

	// Capture the output of PrintUsageWithGroups
	writer := newArrayWriter()
	opts.PrintUsageWithGroups(writer)

	expectedOutput := `usage: ` + os.Args[0] + `

Global Flags:

 --help or - "Display help" (optional)

Commands:
 +  create "Create resources"
 │─  ** create user "Manage users"
 |   |  --email or -e "Email for user creation" (optional)
 └─  **  ** create user type "Specify user type"
 |   |   |  --username "Username for user creation" (required)
 |   |   |  --firstName "User first name" (optional)
 └─  ** create group "Manage groups"
`
	output := strings.Join(*writer.data, "")
	assert.Equal(t, expectedOutput, output, "usage output should be grouped and formatted correctly")
}

func TestCmdLineOption_PrintUsageWithCustomGroups(t *testing.T) {
	opts := NewCmdLineOption()

	// Add global flags
	err := opts.AddFlag("help", &Argument{
		Description: "Display help",
		TypeOf:      Standalone,
	})
	assert.Nil(t, err, "should add global flag successfully")

	// Add commands
	cmd := &Command{
		Name:        "create",
		Description: "Create resources",
		Subcommands: []Command{
			{
				Name:        "user",
				Description: "Manage users",
				Subcommands: []Command{
					{
						Name:        "type",
						Description: "Specify user type",
					},
				},
			},
			{
				Name:        "group",
				Description: "Manage groups",
			},
		},
	}
	err = opts.AddCommand(cmd)
	assert.Nil(t, err, "should add commands successfully")

	// Add command-specific flags
	err = opts.AddFlag("username", &Argument{
		Description: "Username for user creation",
		TypeOf:      Single,
		Required:    true,
	}, "create user type")
	assert.Nil(t, err, "should add command-specific flag successfully")

	err = opts.AddFlag("email", &Argument{
		Description: "Email for user creation",
		TypeOf:      Single,
	}, "create user")
	assert.Nil(t, err, "should add command-specific flag successfully")

	// Define custom print config
	printConfig := &PrettyPrintConfig{
		NewCommandPrefix:     " + ",
		DefaultPrefix:        " │ ",
		TerminalPrefix:       " └ ",
		OuterLevelBindPrefix: " └ ",
		InnerLevelBindPrefix: " * ",
	}

	// Capture the output of PrintUsageWithGroups
	writer := newArrayWriter()
	opts.PrintCommandsWithFlags(writer, printConfig)

	expectedOutput := ` + create "Create resources"
 │  * create user "Manage users"
 └  └ --email "Email for user creation" (optional)
 └  *  * create user type "Specify user type"
 └  └  └ --username "Username for user creation" (required)
 └  * create group "Manage groups"
`

	// Check that the printed output matches the expected structure
	output := strings.Join(*writer.data, "")
	assert.Equal(t, expectedOutput, output, "usage output should be grouped and formatted correctly with custom config")
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
