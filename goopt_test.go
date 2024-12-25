package goopt

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/stretchr/testify/assert"
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

func TestParser_AcceptPattern(t *testing.T) {
	opts := NewParser()

	_ = opts.AddFlag("test2", NewArgument("t2", "", Single, false, Secure{}, ""))

	err := opts.AcceptPattern("test2", PatternValue{Pattern: `^[0-9]+$`, Description: "whole integers only"})
	assert.Nil(t, err, "constraint violation - 'Single' flags take values and therefore should PatternValue")
	assert.True(t, opts.Parse([]string{"--test2", "12344"}), "test2 should accept values which match whole integer patterns")
}

func TestParser_AcceptPatterns(t *testing.T) {
	opts := NewParser()

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

func TestParser_AddPreValidationFilter(t *testing.T) {
	opts := NewParser()

	_ = opts.AddFlag("upper", NewArgument("t", "", Single, false, Secure{}, ""))
	err := opts.AddFlagPreValidationFilter("upper", strings.ToUpper)
	assert.Nil(t, err, "should be able to add a filter to a valid flag")

	_ = opts.AcceptPattern("upper", PatternValue{Pattern: "^[A-Z]+$", Description: "upper case only"})
	assert.True(t, opts.HasPreValidationFilter("upper"), "flag should have a filter defined")
	assert.True(t, opts.Parse([]string{"--upper", "lowercase"}), "parse should not fail and pass PatternValue properly")

	value, _ := opts.Get("upper")
	assert.Equal(t, "LOWERCASE", value, "the value of flag upper should be transformed to uppercase")
}

func TestParser_AddPostValidationFilter(t *testing.T) {
	opts := NewParser()

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

func TestParser_DependsOnFlagValue(t *testing.T) {
	opts := NewParser()

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

func TestParser_AddCommand(t *testing.T) {
	opts := NewParser()

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

func TestParser_RegisterCommand(t *testing.T) {
	opts := NewParser()

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

func TestParser_GetCommandValues(t *testing.T) {
	opts, _ := NewParserWith(
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

func TestParser_ValueCallback(t *testing.T) {
	opts := NewParser()

	shouldBeEqualToOneAfterExecute := 0
	cmd := &Command{
		Name: "create",
		Subcommands: []Command{{
			Name: "user",
			Subcommands: []Command{{
				Name: "type",
				Callback: func(cmdLine *Parser, command *Command) error {
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

func TestParser_WithBindFlag(t *testing.T) {
	var s string
	var i int
	var ts []string

	_, err := NewParserWith(
		WithBindFlag("test", &ts,
			NewArg(
				WithShortFlag("t"),
				WithType(Single))))
	assert.Nil(t, err, "should not fail to bind pointer to supported slice variable to flag when using option functions")

	cmdLine, err := NewParserWith(
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

func TestParser_BindFlag(t *testing.T) {
	var s string
	var i int

	opts := NewParser()
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

	opts = NewParser()
	var boolBind bool
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.Nil(t, err, "should accept Standalone flags in BindFlag if the data type is boolean")

	opts = NewParser()
	err = opts.BindFlag(&i, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.NotNil(t, err, "should not accept Standalone flags in BindFlag if the data type is not boolean")
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.Nil(t, err, "should allow adding field if not yet specified")
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.NotNil(t, err, "should error when adding duplicate field")
}

func TestParser_FileFlag(t *testing.T) {
	var s string
	cmdLine, err := NewParserWith(
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

func TestParser_NewCmdLineFromStruct(t *testing.T) {
	testOpt := TestOptOk{}
	cmd, err := NewParserFromStruct(&testOpt)
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
	_, err = NewParserFromStruct(&TestOptNok{})
	assert.NotNil(t, err, "should error out on invalid struct")
	cmd, err = NewParserFromStruct(&testOpt)
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

func TestParser_NewCmdLineRecursion(t *testing.T) {
	profile := &UserProfile{
		Addresses: make([]Address, 1),
	}

	cmd, err := NewParserFromStruct(profile)
	assert.Nil(t, err, "should handle nested structs")
	assert.True(t, cmd.ParseString("--name Jack -a 10 --address.0.city 'New York'"), "should parse nested arguments")
	assert.Equal(t, "Jack", cmd.GetOrDefault("name", ""))
	assert.Equal(t, "New York", cmd.GetOrDefault("address.0.city", ""))
	assert.Equal(t, "10", cmd.GetOrDefault("age", ""))
}

func TestParser_CommandSpecificFlags(t *testing.T) {
	opts := NewParser()

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

func TestParser_GlobalFlags(t *testing.T) {
	opts := NewParser()

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

func TestParser_SharedFlags(t *testing.T) {
	opts := NewParser()

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

func TestParser_EnvToFlag(t *testing.T) {
	var s string
	cmdLine, err := NewParserWith(
		WithCommand(NewCommand(WithName("command"), WithSubcommands(NewCommand(WithName("test"))))),
		WithBindFlag("testMe", &s,
			NewArg(WithShortFlag("t"),
				WithType(Single))))
	assert.Nil(t, err)
	flagFunc := cmdLine.SetEnvNameConverter(func(s string) string {
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

	cmdLine, err = NewParserWith(
		WithCommand(NewCommand(WithName("command"), WithSubcommands(NewCommand(WithName("test"))))),
		WithBindFlag("testMe", &s,
			NewArg(WithShortFlag("t"),
				WithType(Single)), "command test"))
	assert.Nil(t, err)
	_ = cmdLine.SetEnvNameConverter(func(s string) string {
		return upperSnakeToCamelCase(s)
	})

	assert.True(t, cmdLine.ParseString("command test --testMe 123"))

	assert.Equal(t, "123", s)
	assert.True(t, cmdLine.HasCommand("command test"))
	assert.True(t, cmdLine.ParseString("command test"))
	assert.Equal(t, "test", s)
	assert.True(t, cmdLine.HasCommand("command test"))

}

func TestParser_GlobalAndCommandEnvVars(t *testing.T) {
	opts := NewParser()

	// Define commands
	_ = opts.AddCommand(&Command{Name: "create"})
	_ = opts.AddCommand(&Command{Name: "update"})

	// Define flags for global and specific commands
	_ = opts.AddFlag("verboseTest", &Argument{Description: "Verbose output", TypeOf: Standalone})
	_ = opts.AddFlag("configTest", &Argument{Description: "Config file", TypeOf: Single}, "create")
	_ = opts.AddFlag("idTest", &Argument{Description: "User ID", TypeOf: Single}, "create")
	flagFunc := opts.SetEnvNameConverter(func(s string) string {
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

func TestParser_MixedFlagsWithEnvVars(t *testing.T) {
	opts := NewParser()

	// Define commands
	_ = opts.AddCommand(&Command{Name: "delete"})
	_ = opts.AddCommand(&Command{Name: "create"})

	// Define flags for global and specific commands
	_ = opts.AddFlag("verbose", &Argument{Description: "Verbose output", TypeOf: Standalone})
	_ = opts.AddFlag("config", &Argument{Description: "Config file", TypeOf: Single}, "create")
	_ = opts.AddFlag("force", &Argument{Description: "Force deletion", TypeOf: Standalone}, "delete")
	flagFunc := opts.SetEnvNameConverter(func(s string) string {
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

func TestParser_RepeatCommandWithDifferentContextWithCallbacks(t *testing.T) {
	opts := NewParser()
	opts.SetExecOnParse(true)
	idx := 0

	_ = opts.AddCommand(&Command{Name: "create", Callback: func(cmdLine *Parser, command *Command) error {
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

func TestParser_PosixFlagsWithEnvVars(t *testing.T) {
	opts := NewParser()
	opts.SetPosix(true)

	// Define commands and flags
	_ = opts.AddCommand(&Command{Name: "build"})
	_ = opts.AddFlag("output", &Argument{Description: "Output file", Short: "o", TypeOf: Single}, "build")
	_ = opts.AddFlag("opt", &Argument{Description: "Optimization level", Short: "p", TypeOf: Single}, "build")
	flagFunc := opts.SetEnvNameConverter(func(s string) string {
		return upperSnakeToCamelCase(s)
	})
	assert.Nil(t, flagFunc, "flagFunc should be nil when none is set")
	// Simulate environment variables
	os.Setenv("OUTPUT", "/build/output")

	assert.True(t, opts.ParseString("build -p2"), "should handle posix-style flags")
	assert.Equal(t, "/build/output", opts.GetOrDefault("output", "", "build"), "output flag should be 'mybuild'")
	assert.Equal(t, "2", opts.GetOrDefault("opt", "", "build"), "optimization should be '2' from env var")
}

func TestParser_VarInFileFlag(t *testing.T) {
	uname := fmt.Sprintf("%x", md5.Sum([]byte(time.Now().Format(time.RFC3339Nano))))
	var s string
	cmdLine, err := NewParserWith(
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

func TestParser_BindNil(t *testing.T) {
	opts := NewParser()

	type tester struct {
		TestStr string
	}

	var test *tester
	err := opts.CustomBindFlag(test, func(flag, value string, customStruct interface{}) {

	}, "test1", NewArgument("t1", "", Single, false, Secure{}, ""))

	assert.NotNil(t, err, "should not be able to custom bind a nil pointer")
}

func TestParser_CustomBindFlag(t *testing.T) {
	type tester struct {
		TestStr string
		testInt int
	}

	opts := NewParser()
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

func TestParser_WithCustomBindFlag(t *testing.T) {
	type tester struct {
		TestStr string
		testInt int
	}

	var test tester
	cmdLine, err := NewParserWith(
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

func TestParser_Parsing(t *testing.T) {
	cmdLine, err := NewParserWith(
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

func TestParser_PrintUsage(t *testing.T) {
	opts := NewParser()

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

func TestParser_PosixCompatibleFlags(t *testing.T) {
	opts := NewParser()
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

func TestParser_CommandCallbacks(t *testing.T) {
	opts := NewParser()

	var valueReceived string
	err := opts.AddCommand(
		NewCommand(WithName("create"),
			WithCommandDescription("create family of commands"),
			WithSubcommands(
				NewCommand(
					WithName("user"),
					WithCommandDescription("create user"),
					WithCallback(
						func(cmdLine *Parser, command *Command) error {
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

func TestParser_ParseWithDefaults(t *testing.T) {
	defaults := map[string]string{"flagWithValue": "valueA"}

	cmdLine, _ := NewParserWith(
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

func TestParser_StandaloneFlagWithExplicitValue(t *testing.T) {
	cmdLine, _ := NewParserWith(
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

func TestParser_PrintUsageWithGroups(t *testing.T) {
	opts := NewParser()

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

func TestParser_PrintUsageWithCustomGroups(t *testing.T) {
	opts := NewParser()

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

func TestParser_GenerateCompletion(t *testing.T) {
	p := NewParser()

	// Add global flags
	p.AddFlag("global-flag", NewArgument("g", "A global flag", Single, false, Secure{}, ""))
	p.AddFlag("verbose", NewArgument("v", "Verbose output", Standalone, false, Secure{}, ""))

	// Add command with flags
	cmd := &Command{
		Name:        "test",
		Description: "Test command",
	}
	p.AddCommand(cmd)
	p.AddFlag("test-flag", NewArgument("t", "A test flag", Single, false, Secure{}, ""), "test")

	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			result := p.GenerateCompletion(shell, "testapp")
			if result == "" {
				t.Errorf("GenerateCompletion(%q) returned empty string", shell)
			}

			// Basic validation that the output contains shell-specific content
			switch shell {
			case "bash":
				if !strings.Contains(result, "function __testapp_completion") {
					t.Error("Bash completion missing expected content")
				}
			case "zsh":
				if !strings.Contains(result, "#compdef testapp") {
					t.Error("Zsh completion missing expected content")
				}
			case "fish":
				if !strings.Contains(result, "complete -c testapp") {
					t.Error("Fish completion missing expected content")
				}
			case "powershell":
				if !strings.Contains(result, "Register-ArgumentCompleter") {
					t.Error("PowerShell completion missing expected content")
				}
			}
		})
	}
}

func TestParser_GetNumericTypes(t *testing.T) {
	tests := []struct {
		name         string
		setupValue   string
		wantInt      int
		wantFloat    float64
		wantIntErr   bool
		wantFloatErr bool
	}{
		{
			name:       "valid integer",
			setupValue: "42",
			wantInt:    42,
			wantFloat:  42.0,
		},
		{
			name:       "valid float",
			setupValue: "42.5",
			wantInt:    0,
			wantFloat:  42.5,
			wantIntErr: true,
		},
		{
			name:         "invalid number",
			setupValue:   "not_a_number",
			wantIntErr:   true,
			wantFloatErr: true,
		},
		{
			name:       "zero value",
			setupValue: "0",
			wantInt:    0,
			wantFloat:  0.0,
		},
		{
			name:       "negative integer",
			setupValue: "-123",
			wantInt:    -123,
			wantFloat:  -123.0,
		},
		{
			name:       "negative float",
			setupValue: "-123.45",
			wantInt:    0,
			wantFloat:  -123.45,
			wantIntErr: true,
		},
		{
			name:       "large number",
			setupValue: "9999999",
			wantInt:    9999999,
			wantFloat:  9999999.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()
			err := opts.AddFlag("number", NewArgument("n", "", Single, false, Secure{}, ""))
			assert.Nil(t, err, "should add flag without error")

			assert.True(t, opts.ParseString(fmt.Sprintf("--number %s", tt.setupValue)))

			// Test GetInt
			gotInt, err := opts.GetInt("number", 64)
			if tt.wantIntErr {
				assert.NotNil(t, err, "GetInt should return error for %s", tt.setupValue)
			} else {
				assert.Nil(t, err, "GetInt should not return error for %s", tt.setupValue)
				assert.Equal(t, tt.wantInt, int(gotInt), "GetInt value mismatch")
			}

			// Test GetFloat
			gotFloat, err := opts.GetFloat("number", 64)
			if tt.wantFloatErr {
				assert.NotNil(t, err, "GetFloat should return error for %s", tt.setupValue)
			} else {
				assert.Nil(t, err, "GetFloat should not return error for %s", tt.setupValue)
				assert.Equal(t, tt.wantFloat, gotFloat, "GetFloat value mismatch")
			}
		})
	}
}

func TestParser_GetNumericTypesWithPath(t *testing.T) {
	tests := []struct {
		name         string
		setupValue   string
		cmdPath      string
		wantInt      int
		wantFloat    float64
		wantIntErr   bool
		wantFloatErr bool
	}{
		{
			name:       "valid integer with path",
			setupValue: "42",
			cmdPath:    "create user",
			wantInt:    42,
			wantFloat:  42.0,
		},
		{
			name:         "invalid path",
			setupValue:   "42",
			cmdPath:      "invalid path",
			wantIntErr:   true,
			wantFloatErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()

			// Add command
			cmd := &Command{
				Name: "create",
				Subcommands: []Command{{
					Name: "user",
				}},
			}
			err := opts.AddCommand(cmd)
			assert.Nil(t, err, "should add command without error")

			// Add flag to specific command path
			err = opts.AddFlag("number", NewArgument("n", "", Single, false, Secure{}, ""), "create user")
			assert.Nil(t, err, "should add flag without error")

			assert.True(t, opts.ParseString(fmt.Sprintf("create user --number %s", tt.setupValue)))

			// Test GetInt with path
			gotInt, err := opts.GetInt("number", 64, tt.cmdPath)
			if tt.wantIntErr {
				assert.NotNil(t, err, "GetInt should return error for path %s", tt.cmdPath)
			} else {
				assert.Nil(t, err, "GetInt should not return error for path %s", tt.cmdPath)
				assert.Equal(t, tt.wantInt, int(gotInt), "GetInt value mismatch")
			}

			// Test GetFloat with path
			gotFloat, err := opts.GetFloat("number", 64, tt.cmdPath)
			if tt.wantFloatErr {
				assert.NotNil(t, err, "GetFloat should return error for path %s", tt.cmdPath)
			} else {
				assert.Nil(t, err, "GetFloat should not return error for path %s", tt.cmdPath)
				assert.Equal(t, tt.wantFloat, gotFloat, "GetFloat value mismatch")
			}
		})
	}
}

func TestParser_GetListErrors(t *testing.T) {
	tests := []struct {
		name       string
		flagType   OptionType
		setupValue string

		wantErr bool
	}{
		{
			name:       "non-chained flag",
			flagType:   Single,
			setupValue: "item1,item2",
			wantErr:    true,
		},
		{
			name:       "non-existent flag",
			flagType:   Chained,
			setupValue: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()

			if tt.flagType != Chained {
				// Add a non-chained flag
				err := opts.AddFlag("list", NewArgument("l", "", tt.flagType, false, Secure{}, ""))
				assert.Nil(t, err, "should add flag without error")
				assert.True(t, opts.ParseString("--list "+tt.setupValue))
			}

			// Try to get list from non-existent or non-chained flag
			got, err := opts.GetList("list")
			assert.NotNil(t, err)
			assert.Empty(t, got)
		})
	}
}

func TestParser_GetListWithCustomDelimiter(t *testing.T) {
	opts := NewParser()

	// Set custom delimiter function
	customDelim := func(r rune) bool {
		return r == ';' || r == '#'
	}
	err := opts.SetListDelimiterFunc(customDelim)
	assert.Nil(t, err)

	// Add chained flag
	err = opts.AddFlag("list", NewArgument("l", "", Chained, false, Secure{}, ""))
	assert.Nil(t, err)

	// Test with custom delimiters
	assert.True(t, opts.ParseString("--list \"item1;item2#item3\""))
	got, err := opts.GetList("list")
	assert.Nil(t, err)
	assert.Equal(t, []string{"item1", "item2", "item3"}, got)
}

func TestParser_GetListDefaultDelimiters(t *testing.T) {
	tests := []struct {
		name       string
		setupValue string
		want       []string
	}{
		{
			name:       "all default delimiters",
			setupValue: "\"item1,item2|item3 item4|\"",
			want:       []string{"item1", "item2", "item3", "item4"},
		},
		{
			name:       "consecutive delimiters",
			setupValue: "\"item1,,item2||item3  item4\"",
			want:       []string{"item1", "item2", "item3", "item4"},
		},
		{
			name:       "mixed delimiter patterns",
			setupValue: "\"item1, item2|item3,item4 item5\"",
			want:       []string{"item1", "item2", "item3", "item4", "item5"},
		},
		{
			name:       "single value with delimiters",
			setupValue: "item1",
			want:       []string{"item1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()

			err := opts.AddFlag("list", NewArgument("l", "", Chained, false, Secure{}, ""))
			assert.Nil(t, err, "should add flag without error")

			assert.True(t, opts.ParseString("--list "+tt.setupValue))

			got, err := opts.GetList("list")
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParser_GetListWithCommandPath(t *testing.T) {
	tests := []struct {
		name          string
		setupValue    string
		cmdPath       []string
		want          []string
		wantErr       bool
		shouldNotFail bool
	}{
		{
			name:          "valid command path",
			setupValue:    "item1,item2",
			cmdPath:       []string{"cmd", "subcmd"},
			want:          []string{"item1", "item2"},
			shouldNotFail: true,
		},
		{
			name:          "invalid command path",
			setupValue:    "item1,item2",
			cmdPath:       []string{"invalid", "path"},
			wantErr:       true,
			shouldNotFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()

			cmd := &Command{
				Name: "cmd",
				Subcommands: []Command{{
					Name: "subcmd",
				}},
			}
			err := opts.AddCommand(cmd)
			assert.Nil(t, err, "should add command without error")

			err = opts.AddFlag("list", NewArgument("l", "", Chained, false, Secure{}, ""), tt.cmdPath...)
			assert.Nil(t, err, "should add flag without error")

			cmdLine := strings.Join(tt.cmdPath, " ") + " --list " + tt.setupValue
			assert.Equal(t, tt.shouldNotFail, opts.ParseString(cmdLine))

			got, err := opts.GetList("list", tt.cmdPath...)
			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParser_ValidationFilters(t *testing.T) {
	tests := []struct {
		name       string
		preFilter  FilterFunc
		postFilter FilterFunc
		input      string
		want       string
	}{
		{
			name: "pre-validation filter",
			preFilter: func(s string) string {
				return strings.ToUpper(s)
			},
			input: "test",
			want:  "TEST",
		},
		{
			name: "post-validation filter",
			postFilter: func(s string) string {
				return strings.TrimSpace(s)
			},
			input: " test ",
			want:  "test",
		},
		{
			name: "both filters",
			preFilter: func(s string) string {
				return strings.ToUpper(s)
			},
			postFilter: func(s string) string {
				return strings.TrimSpace(s)
			},
			input: " test ",
			want:  "TEST",
		},
		{
			name: "filter returning empty string",
			preFilter: func(s string) string {
				return ""
			},
			input: "test",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()
			err := opts.AddFlag("test", NewArgument("t", "", Single, false, Secure{}, ""))
			assert.Nil(t, err)

			if tt.preFilter != nil {
				err = opts.AddFlagPreValidationFilter("test", tt.preFilter)
				assert.Nil(t, err)
			}

			if tt.postFilter != nil {
				err = opts.AddFlagPostValidationFilter("test", tt.postFilter)
				assert.Nil(t, err)
			}

			assert.True(t, opts.ParseString(fmt.Sprintf("--test %s", tt.input)))

			got, ok := opts.Get("test")
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParser_AcceptedValues(t *testing.T) {
	tests := []struct {
		name           string
		acceptedValues []PatternValue
		input          string
		wantOk         bool
		wantParse      bool
		want           string
	}{
		{
			name:           "valid value",
			acceptedValues: []PatternValue{{Pattern: `^one$`, Description: "one"}, {Pattern: `^two$`, Description: "two"}, {Pattern: `^three$`, Description: "three"}},
			input:          "two",
			wantOk:         true,
			wantParse:      true,
			want:           "two",
		},
		{
			name:           "invalid value",
			acceptedValues: []PatternValue{{Pattern: `^one$`, Description: "one"}, {Pattern: `^two$`, Description: "two"}, {Pattern: `^three$`, Description: "three"}},
			input:          "four",
			wantOk:         true,
			wantParse:      false,
			want:           "four", // parse should be false but the value should still be set
		},
		{
			name:           "empty accepted values",
			acceptedValues: []PatternValue{},
			input:          "anything",
			wantOk:         true,
			wantParse:      true,
			want:           "anything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()
			err := opts.AddFlag("test", NewArgument("t", "", Single, false, Secure{}, ""))
			assert.Nil(t, err)

			if len(tt.acceptedValues) > 0 {
				err = opts.AcceptPatterns("test", tt.acceptedValues)
				assert.Nil(t, err)
			}

			assert.Equal(t, tt.wantParse, opts.ParseString(fmt.Sprintf("--test %s", tt.input)))

			got, ok := opts.Get("test")
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParser_RequiredIfFlags(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*Parser) error
		input     string
		wantParse bool
	}{
		{
			name: "required if other flag present",
			setupFunc: func(p *Parser) error {
				err := p.AddFlag("flag1", NewArgument("f1", "", Single, false, Secure{}, ""))
				if err != nil {
					return err
				}
				err = p.AddFlag("flag2", NewArgument("f2", "", Single, false, Secure{}, ""))
				if err != nil {
					return err
				}

				return p.SetArgument("flag2", nil, SetRequiredIf(func(cmdLine *Parser, optionName string) (bool, string) {
					return cmdLine.HasFlag("flag1") && optionName == "flag2", "flag2 is required when flag1 is present"
				}))
			},
			input:     "--flag1 value1", // should fail because flag2 is required when flag1 is present
			wantParse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()
			err := tt.setupFunc(opts)
			assert.Nil(t, err)

			assert.Equal(t, tt.wantParse, opts.ParseString(tt.input))
		})
	}
}

func TestParser_Dependencies(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*Parser) error
		input     string
		wantParse bool
		wantWarns []string
	}{
		{
			name: "simple dependency",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArgument("m", "", Single, false, Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", Single, false, Secure{}, ""))
				return p.AddDependency("dependent", "main")
			},
			input:     "--dependent test",
			wantParse: true,
			wantWarns: []string{"Flag 'dependent' depends on 'main' which was not specified."},
		},
		{
			name: "value dependency - single value",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("mode", NewArgument("m", "", Single, false, Secure{}, ""))
				_ = p.AddFlag("debug", NewArgument("d", "", Single, false, Secure{}, ""))
				return p.AddDependencyValue("debug", "mode", []string{"development"})
			},
			input:     "--mode production --debug true",
			wantParse: true,
			wantWarns: []string{"Flag 'debug' depends on 'mode' with value 'development' which was not specified. (got 'production')"},
		},
		{
			name: "value dependency - multiple values",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("mode", NewArgument("m", "", Single, false, Secure{}, ""))
				_ = p.AddFlag("debug", NewArgument("d", "", Single, false, Secure{}, ""))
				return p.AddDependencyValue("debug", "mode", []string{"development", "testing"})
			},
			input:     "--mode development --debug true",
			wantParse: true,
			wantWarns: nil,
		},
		{
			name: "command-specific dependency",
			setupFunc: func(p *Parser) error {
				_ = p.AddCommand(NewCommand(WithName("cmd"), WithCommandDescription("test command"), WithSubcommands(NewCommand(WithName("subcmd")))))
				_ = p.AddFlag("cmdMain", NewArgument("m", "", Single, false, Secure{}, "cmd"))
				_ = p.AddFlag("cmdDependent", NewArgument("d", "", Single, false, Secure{}, "cmd"))
				return p.AddDependency("cmdDependent", "cmdMain", "cmd")
			},
			input:     "cmd --cmdDependent test",
			wantParse: true,
			wantWarns: []string{"Flag 'cmdDependent' depends on 'cmdMain' which was not specified."},
		},
		{
			name: "dependency using short form",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArgument("m", "", Single, false, Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", Single, false, Secure{}, ""))
				return p.AddDependency("d", "m") // using short forms
			},
			input:     "-d test",
			wantParse: true,
			wantWarns: []string{"Flag 'dependent' depends on 'main' which was not specified."},
		},
		{
			name: "mixed form dependencies",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArgument("m", "", Single, false, Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", Single, false, Secure{}, ""))
				err := p.AddDependency("dependent", "m") // long depends on short
				if err != nil {
					return err
				}
				return p.AddDependencyValue("d", "main", []string{"value"}) // short depends on long
			},
			input:     "--dependent test",
			wantParse: true,
			wantWarns: []string{"Flag 'dependent' depends on 'main' which was not specified."},
		},
		{
			name: "remove dependency - short form",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArgument("m", "", Single, false, Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", Single, false, Secure{}, ""))
				err := p.AddDependency("dependent", "main")
				if err != nil {
					return err
				}
				return p.RemoveDependency("d", "m") // remove using short forms
			},
			input:     "--dependent test",
			wantParse: true,
			wantWarns: nil, // dependency was removed
		},
		{
			name: "command dependency with short forms",
			setupFunc: func(p *Parser) error {
				_ = p.AddCommand(NewCommand(
					WithName("cmd"),
					WithCommandDescription("test command"),
					WithSubcommands(NewCommand(WithName("subcmd"))),
				))
				_ = p.AddFlag("cmdMain", NewArgument("m", "", Single, false, Secure{}, "cmd"))
				_ = p.AddFlag("cmdDependent", NewArgument("d", "", Single, false, Secure{}, "cmd"))
				return p.AddDependency("d", "m", "cmd") // using short forms with command
			},
			input:     "cmd -d test",
			wantParse: true,
			wantWarns: []string{"Flag 'cmdDependent' depends on 'cmdMain' which was not specified."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()
			err := tt.setupFunc(opts)
			assert.Nil(t, err)

			assert.Equal(t, tt.wantParse, opts.ParseString(tt.input))

			warnings := opts.GetWarnings()
			if len(tt.wantWarns) == 0 {
				assert.Empty(t, warnings)
			} else {
				assert.Equal(t, tt.wantWarns, warnings)
			}
		})
	}
}

type testField struct {
	Name     string
	Tag      string
	WantName string
	WantPath string
	WantArg  Argument
	WantErr  bool
}

func TestParser_UnmarshalTagsToArgument(t *testing.T) {
	tests := []struct {
		name  string
		field testField
	}{
		{
			name: "legacy format",
			field: testField{
				Name:     "TestField",
				Tag:      `long:"test" short:"t" description:"test desc" path:"cmd subcmd"`,
				WantName: "test",
				WantPath: "cmd subcmd",
				WantArg: Argument{
					Short:       "t",
					Description: "test desc",
				},
			},
		},
		{
			name: "new format",
			field: testField{
				Name:     "TestField",
				Tag:      `goopt:"name:test;short:t;desc:test desc;path:cmd subcmd"`,
				WantName: "test",
				WantPath: "cmd subcmd",
				WantArg: Argument{
					Short:       "t",
					Description: "test desc",
				},
			},
		},
		{
			name: "fallback to field name",
			field: testField{
				Name:     "TestField",
				Tag:      `goopt:"short:t;desc:test desc"`,
				WantName: "",
				WantPath: "",
				WantArg: Argument{
					Short:       "t",
					Description: "test desc",
				},
			},
		},
		{
			name: "multiple paths",
			field: testField{
				Name:     "TestField",
				Tag:      `goopt:"name:test;short:t;desc:test desc;path:cmd1 subcmd1,cmd2 subcmd2"`,
				WantName: "test",
				WantPath: "cmd1 subcmd1,cmd2 subcmd2",
				WantArg: Argument{
					Short:       "t",
					Description: "test desc",
				},
			},
		},
		{
			name: "legacy format with all options",
			field: testField{
				Name:     "TestField",
				Tag:      `long:"test" short:"t" description:"test desc" path:"cmd subcmd" required:"true" type:"single" default:"defaultValue"`,
				WantName: "test",
				WantPath: "cmd subcmd",
				WantArg: Argument{
					Short:        "t",
					Description:  "test desc",
					Required:     true,
					TypeOf:       Single,
					DefaultValue: "defaultValue",
				},
			},
		},
		{
			name: "new format with all options",
			field: testField{
				Name:     "TestField",
				Tag:      `goopt:"name:test;short:t;desc:test desc;path:cmd subcmd;required:true;type:single;default:defaultValue"`,
				WantName: "test",
				WantPath: "cmd subcmd",
				WantArg: Argument{
					Short:        "t",
					Description:  "test desc",
					Required:     true,
					TypeOf:       Single,
					DefaultValue: "defaultValue",
				},
			},
		},
		{
			name: "secure flag legacy",
			field: testField{
				Name:     "Password",
				Tag:      `long:"password" short:"p" description:"secure input" secure:"true"`,
				WantName: "password",
				WantPath: "",
				WantArg: Argument{
					Short:       "p",
					Description: "secure input",
					Secure:      Secure{IsSecure: true},
				},
			},
		},
		{
			name: "secure flag new format",
			field: testField{
				Name:     "Password",
				Tag:      `goopt:"name:password;short:p;desc:secure input;secure:true"`,
				WantName: "password",
				WantPath: "",
				WantArg: Argument{
					Short:       "p",
					Description: "secure input",
					Secure:      Secure{IsSecure: true},
				},
			},
		},
		{
			name: "invalid type value",
			field: testField{
				Name:     "TestField",
				Tag:      `goopt:"name:test;type:invalid"`,
				WantName: "test",
				WantPath: "",
				WantArg: Argument{
					TypeOf: Empty,
				},
			},
		},
		{
			name: "empty path segments",
			field: testField{
				Name:     "TestField",
				Tag:      `goopt:"name:test;path:cmd  subcmd"`, // double space
				WantName: "test",
				WantPath: "cmd  subcmd", // preserved as-is for later processing
				WantArg:  Argument{},
			},
		},
		{
			name: "mixed format (should use goopt)",
			field: testField{
				Name:     "TestField",
				Tag:      `long:"ignored" goopt:"name:test;short:t" description:"ignored"`,
				WantName: "test",
				WantPath: "",
				WantArg: Argument{
					Short: "t",
				},
			},
		},
		{
			name: "file type",
			field: testField{
				Name:     "ConfigFile",
				Tag:      `goopt:"name:config;type:file;desc:configuration file"`,
				WantName: "config",
				WantPath: "",
				WantArg: Argument{
					Description: "configuration file",
					TypeOf:      File,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to reflect.StructField for testing
			structField := reflect.StructField{
				Name: tt.field.Name,
				Tag:  reflect.StructTag(tt.field.Tag),
			}

			arg := &Argument{}
			gotName, gotPath, err := unmarshalTagsToArgument(structField, arg)
			if (err != nil) != tt.field.WantErr {
				t.Errorf("unmarshalTagsToArgument() error = %v, wantErr %v", err, tt.field.WantErr)
				return
			}
			if gotName != tt.field.WantName {
				t.Errorf("unmarshalTagsToArgument() name = %v, want %v", gotName, tt.field.WantName)
			}
			if gotPath != tt.field.WantPath {
				t.Errorf("unmarshalTagsToArgument() path = %v, want %v", gotPath, tt.field.WantPath)
			}
			if !reflect.DeepEqual(*arg, tt.field.WantArg) {
				t.Errorf("unmarshalTagsToArgument() arg = %v, want %v", *arg, tt.field.WantArg)
			}
		})
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
