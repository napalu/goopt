package goopt

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/napalu/goopt/types"

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

	_ = opts.AddFlag("test2", NewArgument("t2", "", types.Single, false, types.Secure{}, ""))

	err := opts.AcceptPattern("test2", types.PatternValue{Pattern: `^[0-9]+$`, Description: "whole integers only"})
	assert.Nil(t, err, "constraint violation - 'Single' flags take values and therefore should PatternValue")
	assert.True(t, opts.Parse([]string{"--test2", "12344"}), "test2 should accept values which match whole integer patterns")
}

func TestParser_AcceptPatterns(t *testing.T) {
	opts := NewParser()

	_ = opts.AddFlag("test", NewArgument("t", "", types.Single, false, types.Secure{}, ""))

	err := opts.AcceptPatterns("test", []types.PatternValue{
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

	_ = opts.AddFlag("upper", NewArgument("t", "", types.Single, false, types.Secure{}, ""))
	err := opts.AddFlagPreValidationFilter("upper", strings.ToUpper)
	assert.Nil(t, err, "should be able to add a filter to a valid flag")

	_ = opts.AcceptPattern("upper", types.PatternValue{Pattern: "^[A-Z]+$", Description: "upper case only"})
	assert.True(t, opts.HasPreValidationFilter("upper"), "flag should have a filter defined")
	assert.True(t, opts.Parse([]string{"--upper", "lowercase"}), "parse should not fail and pass PatternValue properly")

	value, _ := opts.Get("upper")
	assert.Equal(t, "LOWERCASE", value, "the value of flag upper should be transformed to uppercase")
}

func TestParser_AddPostValidationFilter(t *testing.T) {
	opts := NewParser()

	_ = opts.AddFlag("status", NewArgument("t", "", types.Single, false, types.Secure{}, ""))
	err := opts.AddFlagPostValidationFilter("status", func(s string) string {
		if strings.EqualFold(s, "active") {
			return "-1"
		} else if strings.EqualFold(s, "inactive") {
			return "0"
		}

		return s
	})

	assert.Nil(t, err, "should be able to add a filter to a valid flag")

	_ = opts.AcceptPattern("status", types.PatternValue{Pattern: "^(?:active|inactive)$", Description: "set status to either 'active' or 'inactive'"})
	assert.True(t, opts.HasPostValidationFilter("status"), "flag should have a filter defined")
	assert.False(t, opts.Parse([]string{"--status", "invalid"}), "parse should fail on invalid input")
	opts.ClearAll()
	assert.True(t, opts.Parse([]string{"--status", "active"}), "parse should not fail and pass PatternValue properly")

	value, _ := opts.Get("status")
	assert.Equal(t, "-1", value, "the value of flag status should have been transformed to -1 after PatternValue validation")
}

func TestParser_DependsOnFlagValue(t *testing.T) {
	opts := NewParser()

	_ = opts.AddFlag("main", NewArgument("m", "", types.Single, false, types.Secure{}, ""))
	_ = opts.AddFlag("dependent", NewArgument("d", "", types.Single, false, types.Secure{}, ""))

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
		WithType(types.Single)))
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

	// current behavior: last command overwrites a previous one with the same path
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
				WithType(types.Single))))
	assert.Nil(t, err, "should not fail to bind pointer to supported slice variable to flag when using option functions")

	cmdLine, err := NewParserWith(
		WithBindFlag("test", &s,
			NewArg(WithShortFlag("t"),
				WithType(types.Single))),
		WithBindFlag("test1", &i,
			NewArg(WithShortFlag("i"),
				WithType(types.Single))))

	assert.Nil(t, err, "should not fail to bind multiple pointer variables to flag when using option functions")
	assert.True(t, cmdLine.ParseString("--test value --test1 12334"), "should be able to parse an argument configured via option function")
	assert.Equal(t, "value", s, "should not fail to assign command line string argument to variable")
	assert.Equal(t, 12334, i, "should not fail to assign command line integer argument to variable")
}

func TestParser_BindFlag(t *testing.T) {
	var s string
	var i int

	opts := NewParser()
	err := opts.BindFlag(s, "test", NewArgument("t", "", types.Single, false, types.Secure{}, ""))
	assert.NotNil(t, err, "should not accept non-pointer type in BindFlag")

	err = opts.BindFlag(&s, "test", NewArgument("t", "", types.Single, false, types.Secure{}, ""))
	assert.Nil(t, err, "should accept string pointer type in BindFlag")

	err = opts.BindFlag(&i, "test1", NewArgument("t1", "", types.Single, false, types.Secure{}, ""))
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
	}, "test1", NewArgument("t1", "", types.Single, false, types.Secure{}, ""))
	assert.NotNil(t, err, "should not attempt to bind unsupported struct")

	assert.True(t, opts.ParseString("--test1 2"), "should parse a command line argument when given a bound variable")

	opts = NewParser()
	var boolBind bool
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", types.Standalone, false, types.Secure{}, ""))
	assert.Nil(t, err, "should accept Standalone flags in BindFlag if the data type is boolean")

	opts = NewParser()
	err = opts.BindFlag(&i, "test", NewArgument("t", "", types.Standalone, false, types.Secure{}, ""))
	assert.NotNil(t, err, "should not accept Standalone flags in BindFlag if the data type is not boolean")
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", types.Standalone, false, types.Secure{}, ""))
	assert.Nil(t, err, "should allow adding field if not yet specified")
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", types.Standalone, false, types.Secure{}, ""))
	assert.NotNil(t, err, "should error when adding duplicate field")
}

func TestParser_FileFlag(t *testing.T) {
	var s string
	cmdLine, err := NewParserWith(
		WithBindFlag("test", &s,
			NewArg(WithShortFlag("t"),
				WithType(types.File))))
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

type TestOptDefault struct {
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
	_, err = NewParserFromStruct(&TestOptDefault{})
	assert.Nil(t, err, "should not error out on default struct")
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
		TypeOf:      types.Single,
		Short:       "u",
	}, "create user type")
	assert.Nil(t, err, "should properly associate flag with command Path")

	err = opts.AddFlag("email", &Argument{
		Description: "Email for user creation",
		TypeOf:      types.Single,
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
		TypeOf:      types.Standalone,
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
		TypeOf:      types.Single,
	}, "create user type")
	assert.Nil(t, err, "should properly add shared flag to user creation command")

	err = opts.AddFlag("sharedFlag", &Argument{
		Description: "Shared flag for group creation",
		TypeOf:      types.Single,
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
				WithType(types.Single))))
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
				WithType(types.Single)), "command test"))
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
	_ = opts.AddFlag("verboseTest", &Argument{Description: "Verbose output", TypeOf: types.Standalone})
	_ = opts.AddFlag("configTest", &Argument{Description: "Config file", TypeOf: types.Single}, "create")
	_ = opts.AddFlag("idTest", &Argument{Description: "User ID", TypeOf: types.Single}, "create")
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
	_ = opts.AddFlag("verbose", &Argument{Description: "Verbose output", TypeOf: types.Standalone})
	_ = opts.AddFlag("config", &Argument{Description: "Config file", TypeOf: types.Single}, "create")
	_ = opts.AddFlag("force", &Argument{Description: "Force deletion", TypeOf: types.Standalone}, "delete")
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
		assert.True(t, cmdLine.HasFlag("id", command.path))
		assert.True(t, cmdLine.HasFlag("group", command.path))
		if idx == 0 {
			assert.Equal(t, "1", cmdLine.GetOrDefault("id", "", command.path))
			assert.Equal(t, "3", cmdLine.GetOrDefault("group", "", command.path))

		} else if idx == 1 {
			assert.Equal(t, "2", cmdLine.GetOrDefault("id", "", command.path))
			assert.Equal(t, "4", cmdLine.GetOrDefault("group", "", command.path))
			assert.Equal(t, "Mike", cmdLine.GetOrDefault("name", "", command.path))
		}

		idx++

		return nil
	}})

	// Define flags for specific commands
	_ = opts.AddFlag("id", &Argument{Description: "User ID", TypeOf: types.Single}, "create")
	_ = opts.AddFlag("group", &Argument{Description: "Group ID", TypeOf: types.Single}, "create")
	_ = opts.AddFlag("name", &Argument{Description: "User Name", TypeOf: types.Single}, "create")

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
	_ = opts.AddFlag("output", &Argument{Description: "Output file", Short: "o", TypeOf: types.Single}, "build")
	_ = opts.AddFlag("opt", &Argument{Description: "Optimization level", Short: "p", TypeOf: types.Single}, "build")
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
				WithType(types.File),
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

	}, "test1", NewArgument("t1", "", types.Single, false, types.Secure{}, ""))

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
	}, "test1", NewArgument("t1", "", types.Single, false, types.Secure{}, ""))
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
				WithType(types.Single))))
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
				WithType(types.Single),
				WithDescription("this flag requires a value"),
				WithDependentFlags([]string{"flagA", "flagB"}),
				SetRequired(true))),
		WithFlag("flagA",
			NewArg(
				WithShortFlag("fa"),
				WithType(types.Standalone))),
		WithFlag("flagB",
			NewArg(
				WithShortFlag("fb"),
				WithDescription("This is flag B - flagWithValue depends on it"),
				WithDefaultValue("db"),
				WithType(types.Single))),
		WithFlag("flagC",
			NewArg(
				WithShortFlag("fc"),
				WithDescription("this is flag C - it's a chained flag which can return a list"),
				WithType(types.Chained))),
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
		TypeOf:      types.Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("alsolong", &Argument{
		Short:       "b",
		Description: "short flag b",
		TypeOf:      types.Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("boolFlag", &Argument{
		Short:       "c",
		Description: "short flag c",
		TypeOf:      types.Standalone,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("anotherlong", &Argument{
		Short:       "d",
		Description: "short flag d",
		TypeOf:      types.Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("yetanotherlong", &Argument{
		Short:       "e",
		Description: "short flag e",
		TypeOf:      types.Single,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("badoption", &Argument{
		Short:       "ab",
		Description: "posix incompatible flag",
		TypeOf:      types.Single,
	})
	assert.True(t, errors.Is(err, types.ErrPosixIncompatible))
	err = opts.AddFlag("listFlag", &Argument{
		Short:       "f",
		Description: "list",
		TypeOf:      types.Chained,
	})
	assert.Nil(t, err)
	err = opts.AddFlag("tee", &Argument{
		Short:       "t",
		Description: "tee for 2",
		TypeOf:      types.Single,
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
				WithType(types.Single),
				WithDescription("this flag requires a value"),
				WithDependentFlags([]string{"flagA", "flagB"}),
				SetRequired(true))),
		WithFlag("flagA",
			NewArg(
				WithShortFlag("fa"),
				WithType(types.Standalone))),
		WithFlag("flagB",
			NewArg(
				WithShortFlag("fb"),
				WithDescription("This is flag B - flagWithValue depends on it"),
				WithDefaultValue("db"),
				WithType(types.Single))),
		WithFlag("flagC",
			NewArg(
				WithShortFlag("fc"),
				WithDescription("this is flag C - it's a chained flag which can return a list"),
				WithType(types.Chained))))

	assert.True(t, cmdLine.ParseStringWithDefaults(defaults, "-fa -fb"), "required value should be set by default")
	assert.Equal(t, cmdLine.GetOrDefault("flagWithValue", ""), "valueA", "value should be supplied by default")

}

func TestParser_StandaloneFlagWithExplicitValue(t *testing.T) {
	cmdLine, _ := NewParserWith(
		WithFlag("flagA",
			NewArg(
				WithShortFlag("fa"),
				WithType(types.Standalone))),
		WithFlag("flagB",
			NewArg(
				WithShortFlag("fb"),
				WithType(types.Single))))

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
		TypeOf:      types.Standalone,
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
		TypeOf:      types.Single,
		Required:    true,
	}, "create user type")
	assert.Nil(t, err, "should add command-specific flag successfully")
	err = opts.AddFlag("firstName", &Argument{
		Description: "User first name",
		TypeOf:      types.Single,
	}, "create user type")
	assert.Nil(t, err, "should add command-specific flag successfully")

	err = opts.AddFlag("email", &Argument{
		Description: "Email for user creation",
		Short:       "e",
		TypeOf:      types.Single,
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
		TypeOf:      types.Standalone,
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
		TypeOf:      types.Single,
		Required:    true,
	}, "create user type")
	assert.Nil(t, err, "should add command-specific flag successfully")

	err = opts.AddFlag("email", &Argument{
		Description: "Email for user creation",
		TypeOf:      types.Single,
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
	_ = p.AddFlag("global-flag", NewArgument("g", "A global flag", types.Single, false, types.Secure{}, ""))
	_ = p.AddFlag("verbose", NewArgument("v", "Verbose output", types.Standalone, false, types.Secure{}, ""))

	// Add command with flags
	cmd := &Command{
		Name:        "test",
		Description: "Test command",
	}
	_ = p.AddCommand(cmd)
	_ = p.AddFlag("test-flag", NewArgument("t", "A test flag", types.Single, false, types.Secure{}, ""), "test")

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
			err := opts.AddFlag("number", NewArgument("n", "", types.Single, false, types.Secure{}, ""))
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
			err = opts.AddFlag("number", NewArgument("n", "", types.Single, false, types.Secure{}, ""), "create user")
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
		flagType   types.OptionType
		setupValue string

		wantErr bool
	}{
		{
			name:       "non-chained flag",
			flagType:   types.Single,
			setupValue: "item1,item2",
			wantErr:    true,
		},
		{
			name:       "non-existent flag",
			flagType:   types.Chained,
			setupValue: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()

			if tt.flagType != types.Chained {
				// Add a non-chained flag
				err := opts.AddFlag("list", NewArgument("l", "", tt.flagType, false, types.Secure{}, ""))
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
	err = opts.AddFlag("list", NewArgument("l", "", types.Chained, false, types.Secure{}, ""))
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

			err := opts.AddFlag("list", NewArgument("l", "", types.Chained, false, types.Secure{}, ""))
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

			err = opts.AddFlag("list", NewArgument("l", "", types.Chained, false, types.Secure{}, ""), tt.cmdPath...)
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
			err := opts.AddFlag("test", NewArgument("t", "", types.Single, false, types.Secure{}, ""))
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
		acceptedValues []types.PatternValue
		input          string
		wantOk         bool
		wantParse      bool
		want           string
	}{
		{
			name:           "valid value",
			acceptedValues: []types.PatternValue{{Pattern: `^one$`, Description: "one"}, {Pattern: `^two$`, Description: "two"}, {Pattern: `^three$`, Description: "three"}},
			input:          "two",
			wantOk:         true,
			wantParse:      true,
			want:           "two",
		},
		{
			name:           "invalid value",
			acceptedValues: []types.PatternValue{{Pattern: `^one$`, Description: "one"}, {Pattern: `^two$`, Description: "two"}, {Pattern: `^three$`, Description: "three"}},
			input:          "four",
			wantOk:         true,
			wantParse:      false,
			want:           "four", // parse should be false but the value should still be set
		},
		{
			name:           "empty accepted values",
			acceptedValues: []types.PatternValue{},
			input:          "anything",
			wantOk:         true,
			wantParse:      true,
			want:           "anything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()
			err := opts.AddFlag("test", NewArgument("t", "", types.Single, false, types.Secure{}, ""))
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
				err := p.AddFlag("flag1", NewArgument("f1", "", types.Single, false, types.Secure{}, ""))
				if err != nil {
					return err
				}
				err = p.AddFlag("flag2", NewArgument("f2", "", types.Single, false, types.Secure{}, ""))
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
				_ = p.AddFlag("main", NewArgument("m", "", types.Single, false, types.Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", types.Single, false, types.Secure{}, ""))
				return p.AddDependency("dependent", "main")
			},
			input:     "--dependent test",
			wantParse: true,
			wantWarns: []string{"Flag 'dependent' depends on 'main' which was not specified."},
		},
		{
			name: "value dependency - single value",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("mode", NewArgument("m", "", types.Single, false, types.Secure{}, ""))
				_ = p.AddFlag("debug", NewArgument("d", "", types.Single, false, types.Secure{}, ""))
				return p.AddDependencyValue("debug", "mode", []string{"development"})
			},
			input:     "--mode production --debug true",
			wantParse: true,
			wantWarns: []string{"Flag 'debug' depends on 'mode' with value 'development' which was not specified. (got 'production')"},
		},
		{
			name: "value dependency - multiple values",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("mode", NewArgument("m", "", types.Single, false, types.Secure{}, ""))
				_ = p.AddFlag("debug", NewArgument("d", "", types.Single, false, types.Secure{}, ""))
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
				_ = p.AddFlag("cmdMain", NewArgument("m", "", types.Single, false, types.Secure{}, "cmd"))
				_ = p.AddFlag("cmdDependent", NewArgument("d", "", types.Single, false, types.Secure{}, "cmd"))
				return p.AddDependency("cmdDependent", "cmdMain", "cmd")
			},
			input:     "cmd --cmdDependent test",
			wantParse: true,
			wantWarns: []string{"Flag 'cmdDependent' depends on 'cmdMain' which was not specified."},
		},
		{
			name: "dependency using short form",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArgument("m", "", types.Single, false, types.Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", types.Single, false, types.Secure{}, ""))
				return p.AddDependency("d", "m") // using short forms
			},
			input:     "-d test",
			wantParse: true,
			wantWarns: []string{"Flag 'dependent' depends on 'main' which was not specified."},
		},
		{
			name: "mixed form dependencies",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArgument("m", "", types.Single, false, types.Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", types.Single, false, types.Secure{}, ""))
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
				_ = p.AddFlag("main", NewArgument("m", "", types.Single, false, types.Secure{}, ""))
				_ = p.AddFlag("dependent", NewArgument("d", "", types.Single, false, types.Secure{}, ""))
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
				_ = p.AddFlag("cmdMain", NewArgument("m", "", types.Single, false, types.Secure{}, "cmd"))
				_ = p.AddFlag("cmdDependent", NewArgument("d", "", types.Single, false, types.Secure{}, "cmd"))
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
		field struct {
			Name     string
			Tag      string
			WantName string
			WantPath string
			WantArg  Argument
			WantErr  bool
		}
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
					TypeOf:       types.Single,
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
					TypeOf:       types.Single,
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
					Secure:      types.Secure{IsSecure: true},
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
					Secure:      types.Secure{IsSecure: true},
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
					TypeOf: types.Empty,
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
			name: "unspecified type defaults to empty if type is not supported or cannot be inferred",
			field: testField{
				Name:     "TestField",
				Tag:      `goopt:"name:test"`,
				WantName: "test",
				WantPath: "",
				WantArg: Argument{
					TypeOf: types.Empty,
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
					TypeOf:      types.File,
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

type PrecedenceTestOptions struct {
	Value string `goopt:"name:value;desc:Test value;default:struct_default"`
}

func TestParser_ConfigurationPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() (*Parser, error)
		cliArgs       []string
		envVar        string
		externalValue string
		defaultValue  string
		expectedValue string
		description   string
		flagName      string
	}{
		// Struct-based parser tests
		{
			name: "struct: CLI has highest precedence",
			setupFunc: func() (*Parser, error) {
				opts := &PrecedenceTestOptions{}
				return NewParserFromStruct(opts)
			},
			cliArgs:       []string{"--value", "cli_value"},
			envVar:        "VALUE=env_value",
			externalValue: "external_value",
			expectedValue: "cli_value",
			flagName:      "value",
			description:   "CLI value should override all others",
		},
		{
			name: "struct: External config has second highest precedence",
			setupFunc: func() (*Parser, error) {
				opts := &PrecedenceTestOptions{}
				return NewParserFromStruct(opts)
			},
			envVar:        "VALUE=env_value",
			externalValue: "external_value",
			expectedValue: "external_value",
			description:   "External value should override env and default",
			flagName:      "value",
		},
		{
			name: "struct: ENV has third highest precedence",
			setupFunc: func() (*Parser, error) {
				opts := &PrecedenceTestOptions{}
				return NewParserFromStruct(opts)
			},
			envVar:        "VALUE=env_value",
			expectedValue: "env_value",
			description:   "ENV value should override default",
			flagName:      "value",
		},
		{
			name: "struct: Default has lowest precedence",
			setupFunc: func() (*Parser, error) {
				opts := &PrecedenceTestOptions{}
				return NewParserFromStruct(opts)
			},
			expectedValue: "struct_default",
			description:   "Default value should be used when no others present",
			flagName:      "value",
		},
		{
			name: "env: snake_case to camelCase",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("myTestValue", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("default"),
				))
			},
			envVar:        "my_test_value=env_value",
			expectedValue: "env_value",
			flagName:      "myTestValue",
			description:   "snake_case env var should map to camelCase flag",
		},
		{
			name: "env: SCREAMING_SNAKE to camelCase",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("myTestValue", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("default"),
				))
			},
			envVar:        "MY_TEST_VALUE=env_value",
			expectedValue: "env_value",
			description:   "SCREAMING_SNAKE env var should map to camelCase flag",
			flagName:      "myTestValue",
		},
		{
			name: "env: kebab-case to camelCase",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("myTestValue", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("default"),
				))
			},
			envVar:        "my-test-value=env_value",
			expectedValue: "env_value",
			description:   "kebab-case env var should map to camelCase flag",
			flagName:      "myTestValue",
		},
		{
			name: "env: PascalCase to camelCase",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("myTestValue", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("default"),
				))
			},
			envVar:        "MyTestValue=env_value",
			expectedValue: "env_value",
			description:   "PascalCase env var should map to camelCase flag",
			flagName:      "myTestValue",
		},
		{
			name: "env: dotted.case to camelCase",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("myTestValue", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("default"),
				))
			},
			envVar:        "my.test.value=env_value",
			expectedValue: "env_value",
			description:   "dotted.case env var should map to camelCase flag",
			flagName:      "myTestValue",
		},
		{
			name: "env: mixed_Case.formats-HERE to camelCase",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("myTestValueHere", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("default"),
				))
			},
			envVar:        "my_Test.value-HERE=env_value",
			expectedValue: "env_value",
			description:   "mixed case formats should map to camelCase flag",
			flagName:      "myTestValueHere",
		},
		{
			name: "builder: CLI has highest precedence",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("value", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("builder_default"),
				))
			},
			cliArgs:       []string{"--value", "cli_value"},
			envVar:        "VALUE=env_value",
			externalValue: "external_value",
			expectedValue: "cli_value",
			flagName:      "value",
		},
		{
			name: "imperative: CLI has highest precedence",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				err := p.AddFlag("value", NewArg(
					WithDescription("Test value"),
				))
				if err != nil {
					return nil, err
				}
				err = p.SetArgument("value", nil, WithDefaultValue("imperative_default"))
				if err != nil {
					return nil, err
				}
				return p, nil
			},
			cliArgs:       []string{"--value", "cli_value"},
			envVar:        "VALUE=env_value",
			externalValue: "external_value",
			expectedValue: "cli_value",
			flagName:      "value",
		},
		{
			name: "struct: default from tag should not override ENV",
			setupFunc: func() (*Parser, error) {
				opts := &struct {
					Value string `goopt:"name:value;default:struct_default"`
				}{}
				return NewParserFromStruct(opts)
			},
			envVar:        "VALUE=env_value",
			expectedValue: "env_value",
			description:   "ENV value should have precedence over struct tag default",
			flagName:      "value",
		},
		{
			name: "builder: default from NewArg should not override ENV",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				return p, p.AddFlag("value", NewArg(
					WithDescription("Test value"),
					WithDefaultValue("builder_default"),
				))
			},
			envVar:        "VALUE=env_value",
			expectedValue: "env_value",
			description:   "ENV value should have precedence over NewArg default",
			flagName:      "value",
		},
		{
			name: "imperative: default from SetArgument should not override ENV",
			setupFunc: func() (*Parser, error) {
				p := NewParser()
				err := p.AddFlag("value", NewArg(
					WithDescription("Test value"),
				))
				if err != nil {
					return nil, err
				}
				err = p.SetArgument("value", nil, WithDefaultValue("imperative_default"))
				if err != nil {
					return nil, err
				}
				return p, nil
			},
			envVar:        "VALUE=env_value",
			expectedValue: "env_value",
			description:   "ENV value should have precedence over SetArgument default",
			flagName:      "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup parser
			parser, err := tt.setupFunc()
			assert.NoError(t, err)

			// Setup environment
			if tt.envVar != "" {
				key, value, _ := strings.Cut(tt.envVar, "=")
				os.Setenv(key, value)
				defer os.Unsetenv(key)
				parser.SetEnvNameConverter(func(s string) string {
					return DefaultFlagNameConverter(s)
				})
			}

			// Parse with or without defaults
			var success bool
			if tt.externalValue != "" {
				success = parser.ParseWithDefaults(
					map[string]string{"value": tt.externalValue},
					tt.cliArgs,
				)
			} else {
				success = parser.Parse(tt.cliArgs)
			}

			assert.True(t, success, "Parse should succeed")

			// Verify value
			value, found := parser.Get(tt.flagName)
			assert.True(t, found, "Value should be found")
			assert.Equal(t, tt.expectedValue, value, tt.description)
		})
	}
}

func TestParser_NestedConfigurationPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() (*Parser, error)
		cliArgs       []string
		envVar        string
		flagName      string
		externalValue string
		expectedValue string
		description   string
	}{
		{
			name: "nested struct: CLI has highest precedence",
			setupFunc: func() (*Parser, error) {
				opts := &struct {
					Database struct {
						Connection struct {
							Host string `goopt:"name:host;default:localhost"`
						} `goopt:"name:connection"`
					} `goopt:"name:database"`
				}{}
				return NewParserFromStruct(opts)
			},
			cliArgs:       []string{"--database.connection.host", "cli_host"},
			envVar:        "DATABASE_CONNECTION_HOST=env_host",
			flagName:      "database.connection.host",
			externalValue: "external_host",
			expectedValue: "cli_host",
			description:   "CLI value should override all others in nested structure",
		},
		{
			name: "nested struct: external has second highest precedence",
			setupFunc: func() (*Parser, error) {
				opts := &struct {
					Database struct {
						Connection struct {
							Host string `goopt:"name:host;default:localhost"`
						} `goopt:"name:connection"`
					} `goopt:"name:database"`
				}{}
				return NewParserFromStruct(opts)
			},
			envVar:        "DATABASE_CONNECTION_HOST=env_host",
			flagName:      "database.connection.host",
			externalValue: "external_host",
			expectedValue: "external_host",
			description:   "External value should override env and default in nested structure",
		},
		{
			name: "nested struct: ENV has third highest precedence",
			setupFunc: func() (*Parser, error) {
				opts := &struct {
					Database struct {
						Connection struct {
							Host string `goopt:"name:host;default:localhost"`
						} `goopt:"name:connection"`
					} `goopt:"name:database"`
				}{}
				return NewParserFromStruct(opts)
			},
			envVar:        "DATABASE_CONNECTION_HOST=env_host",
			flagName:      "database.connection.host",
			expectedValue: "env_host",
			description:   "ENV value should override default in nested structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup parser
			parser, err := tt.setupFunc()
			assert.NoError(t, err)

			// Setup environment with custom converter for nested flags
			if tt.envVar != "" {
				key, value, _ := strings.Cut(tt.envVar, "=")
				os.Setenv(key, value)
				defer os.Unsetenv(key)
				parser.SetEnvNameConverter(func(s string) string {
					parts := strings.Split(s, "_")
					for i, part := range parts {
						parts[i] = DefaultFlagNameConverter(part)
					}
					return strings.Join(parts, ".")
				})
			}

			// Parse with or without defaults
			var success bool
			if tt.externalValue != "" {
				success = parser.ParseWithDefaults(
					map[string]string{tt.flagName: tt.externalValue},
					tt.cliArgs,
				)
			} else {
				success = parser.Parse(tt.cliArgs)
			}

			assert.True(t, success, "Parse should succeed")

			// Verify value
			value, found := parser.Get(tt.flagName)
			assert.True(t, found, "Value should be found")
			assert.Equal(t, tt.expectedValue, value, tt.description)
		})
	}
}

func TestParser_SliceFieldProcessing(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() (*Parser, error)
		cliArgs       []string
		flagName      string
		expectedValue string
		description   string
	}{
		{
			name: "slice of structs",
			setupFunc: func() (*Parser, error) {
				opts := &struct {
					Servers []struct {
						Host string `goopt:"name:host"`
						Port int    `goopt:"name:port"`
					} `goopt:"name:servers"`
				}{
					// Pre-initialize slice
					Servers: make([]struct {
						Host string `goopt:"name:host"`
						Port int    `goopt:"name:port"`
					}, 1),
				}
				return NewParserFromStruct(opts)
			},
			cliArgs:       []string{"--servers.0.host", "localhost", "--servers.0.port", "8080"},
			flagName:      "servers.0.host",
			expectedValue: "localhost",
			description:   "Should handle slice of structs with pre-initialized slice",
		},
		{
			name: "nested slice of structs",
			setupFunc: func() (*Parser, error) {
				opts := &struct {
					Database struct {
						Shards []struct {
							Host     string `goopt:"name:host"`
							Replicas int    `goopt:"name:replicas"`
						} `goopt:"name:shards"`
					} `goopt:"name:database"`
				}{
					Database: struct {
						Shards []struct {
							Host     string `goopt:"name:host"`
							Replicas int    `goopt:"name:replicas"`
						} `goopt:"name:shards"`
					}{
						// Pre-initialize nested slice
						Shards: make([]struct {
							Host     string `goopt:"name:host"`
							Replicas int    `goopt:"name:replicas"`
						}, 1),
					},
				}
				return NewParserFromStruct(opts)
			},
			cliArgs:       []string{"--database.shards.0.host", "shard1", "--database.shards.0.replicas", "3"},
			flagName:      "database.shards.0.host",
			expectedValue: "shard1",
			description:   "Should handle nested slice of structs with pre-initialized slice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := tt.setupFunc()
			assert.NoError(t, err)

			success := parser.Parse(tt.cliArgs)
			assert.True(t, success, "Parse should succeed")

			value, found := parser.Get(tt.flagName)
			assert.True(t, found, "Value should be found")
			assert.Equal(t, tt.expectedValue, value, tt.description)
		})
	}
}

func TestParser_ProcessStructCommands(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (*Parser, interface{})
		wantCmds    []string
		description string
	}{
		{
			name: "simple command",
			setupFunc: func() (*Parser, interface{}) {
				opts := &struct {
					Serve struct {
						Host string `goopt:"name:host"`
					} `goopt:"name:serve;kind:command"`
				}{}
				p := NewParser()
				return p, opts
			},
			wantCmds:    []string{"serve"},
			description: "Should register single command without parent",
		},
		{
			name: "nested commands with parent",
			setupFunc: func() (*Parser, interface{}) {
				opts := &struct {
					Server struct {
						Start struct {
							Host string `goopt:"name:host"`
						} `goopt:"name:start;kind:command"`
					} `goopt:"name:server;kind:command"`
				}{}
				p := NewParser()
				return p, opts
			},
			wantCmds:    []string{"server", "server start"},
			description: "Should use existing parent command",
		},
		{
			name: "multiple nested commands",
			setupFunc: func() (*Parser, interface{}) {
				opts := &struct {
					Create struct {
						User struct {
							Type struct {
								Name string `goopt:"name:name"`
							} `goopt:"name:type;kind:command"`
						} `goopt:"name:user;kind:command"`
					} `goopt:"name:create;kind:command"`
				}{}
				p := NewParser()
				return p, opts
			},
			wantCmds:    []string{"create", "create user", "create user type"},
			description: "Should build command hierarchy without pre-registered parents",
		},
		{
			name: "explicit command type",
			setupFunc: func() (*Parser, interface{}) {
				opts := Command{
					path:        "server",
					Description: "Server management",
					Name:        "server",
					Subcommands: []Command{
						{
							path:        "start",
							Description: "Start the server",
							Name:        "start",
						},
					},
				}
				p := NewParser()
				return p, opts
			},
			wantCmds:    []string{"server", "server start"},
			description: "Should handle explicit Command{} fields",
		},
		{
			name: "mixed command types",
			setupFunc: func() (*Parser, interface{}) {
				opts := &struct {
					Create struct {
						User  Command
						Group struct {
							Add Command
						} `goopt:"name:group;kind:command;path:create group"`
					} `goopt:"name:create;kind:command"`
				}{
					Create: struct {
						User  Command
						Group struct {
							Add Command
						} `goopt:"name:group;kind:command;path:create group"`
					}{
						User: Command{
							Name:        "user",
							Description: "User management",
						},
						Group: struct {
							Add Command
						}{
							Add: Command{
								Name:        "add",
								Description: "Add a group",
							},
						},
					},
				}
				p := NewParser()
				return p, opts
			},
			wantCmds:    []string{"create", "create group", "create group add", "create user"},
			description: "Should handle mix of tagged structs and Command{} fields",
		},
		{
			name: "command with properties",
			setupFunc: func() (*Parser, interface{}) {
				opts := &struct {
					ServerCmd Command
				}{
					ServerCmd: Command{
						Name:        "server",
						Description: "Server management",
						path:        "server",
					},
				}
				p := NewParser()
				return p, opts
			},
			wantCmds:    []string{"server"},
			description: "Should preserve Command{} properties",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, opts := tt.setupFunc()

			// Handle both pointer and non-pointer types
			val := reflect.ValueOf(opts)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}

			err := parser.processStructCommands(val, "", 0, 10)
			assert.NoError(t, err, "processStructCommands should not error")

			var gotCmds []string
			for kv := parser.registeredCommands.Front(); kv != nil; kv = kv.Next() {
				gotCmds = append(gotCmds, kv.Value.path)

			}

			sort.Strings(gotCmds)
			sort.Strings(tt.wantCmds)
			assert.Equal(t, tt.wantCmds, gotCmds, tt.description)
		})
	}
}

func TestParser_ValidateDependencies(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(p *Parser) *FlagInfo
		mainKey     string
		wantErrs    []string
		description string
	}{
		{
			name: "circular dependency",
			setupFunc: func(p *Parser) *FlagInfo {
				flag1 := NewArgument("", "", types.Single, false, types.Secure{}, "")
				flag1.DependencyMap = map[string][]string{"flag2": {""}}
				_ = p.AddFlag("flag1", flag1)

				flag2 := NewArgument("", "", types.Single, false, types.Secure{}, "")
				flag2.DependencyMap = map[string][]string{"flag1": {""}}
				_ = p.AddFlag("flag2", flag2)

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []string{"circular dependency detected"},
			description: "Should detect direct circular dependencies",
		},
		{
			name: "indirect circular dependency",
			setupFunc: func(p *Parser) *FlagInfo {
				flag1 := NewArgument("", "", types.Single, false, types.Secure{}, "")
				flag1.DependencyMap = map[string][]string{"flag2": {""}}
				_ = p.AddFlag("flag1", flag1)

				flag2 := NewArgument("", "", types.Single, false, types.Secure{}, "")
				flag2.DependencyMap = map[string][]string{"flag3": {""}}
				_ = p.AddFlag("flag2", flag2)

				flag3 := NewArgument("", "", types.Single, false, types.Secure{}, "")
				flag3.DependencyMap = map[string][]string{"flag1": {""}}
				_ = p.AddFlag("flag3", flag3)

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []string{"circular dependency detected"},
			description: "Should detect indirect circular dependencies",
		},
		{
			name: "max depth exceeded",
			setupFunc: func(p *Parser) *FlagInfo {
				// Create chain of MaxDependencyDepth+2 flags where each depends on the next
				maxDepth := p.GetMaxDependencyDepth() + 2
				for i := 1; i <= maxDepth; i++ {
					flag := NewArgument("", "", types.Single, false, types.Secure{}, "")
					if i < maxDepth {
						flag.DependencyMap = map[string][]string{fmt.Sprintf("flag%d", i+1): {""}}
					}
					_ = p.AddFlag(fmt.Sprintf("flag%d", i), flag)
				}

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []string{"maximum dependency depth exceeded for flag"},
			description: "Should detect when dependency chain exceeds max depth",
		},
		{
			name: "missing dependent flag",
			setupFunc: func(p *Parser) *FlagInfo {
				flag := NewArgument("", "", types.Single, false, types.Secure{}, "")
				flag.DependencyMap = map[string][]string{"nonexistent": {""}}
				_ = p.AddFlag("flag1", flag)

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []string{"flag 'flag1' depends on 'nonexistent', but it is missing from command group"},
			description: "Should detect when dependent flag doesn't exist",
		},
		{
			name: "valid simple dependency",
			setupFunc: func(p *Parser) *FlagInfo {
				flag1 := NewArgument("", "", types.Single, false, types.Secure{}, "")
				flag1.DependencyMap = map[string][]string{"flag2": {""}}
				_ = p.AddFlag("flag1", flag1)

				flag2 := NewArgument("", "", types.Single, false, types.Secure{}, "")
				_ = p.AddFlag("flag2", flag2)

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    nil,
			description: "Should accept valid dependency chain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			flagInfo := tt.setupFunc(p)

			visited := make(map[string]bool)
			p.validateDependencies(flagInfo, tt.mainKey, visited, 0)

			errs := p.GetErrors()
			if len(tt.wantErrs) == 0 {
				assert.Empty(t, errs)
			} else {
				assert.NotEmpty(t, errs, "Expected errors but got none")
				if len(errs) > 0 {
					for i, wantErr := range tt.wantErrs {
						assert.Contains(t, errs[i].Error(), wantErr)
					}
				}
			}
		})
	}
}

func TestParser_SetMaxDependencyDepth(t *testing.T) {
	tests := []struct {
		name     string
		setDepth int
		want     int
	}{
		{
			name:     "default value",
			setDepth: 0,
			want:     DefaultMaxDependencyDepth,
		},
		{
			name:     "custom valid value",
			setDepth: 15,
			want:     15,
		},
		{
			name:     "negative value should use default",
			setDepth: -1,
			want:     DefaultMaxDependencyDepth,
		},
		{
			name:     "zero should use default",
			setDepth: 0,
			want:     DefaultMaxDependencyDepth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()

			if tt.setDepth != 0 {
				p.SetMaxDependencyDepth(tt.setDepth)
			}

			got := p.GetMaxDependencyDepth()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParser_CommandExecution(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(p *Parser) error
		args        []string
		execMethod  string            // "single", "all", or "onParse"
		wantErrs    map[string]string // map[commandName]expectedError
		description string
	}{
		{
			name: "execute single command",
			setupFunc: func(p *Parser) error {
				cmd := &Command{
					Name: "test",
					Callback: func(cmdLine *Parser, command *Command) error {
						return fmt.Errorf("test failed")
					},
				}
				return p.AddCommand(cmd)
			},
			args:        []string{"test"},
			execMethod:  "single",
			wantErrs:    map[string]string{"test": "test failed"},
			description: "Should execute single command via ExecuteCommand",
		},
		{
			name: "execute all commands",
			setupFunc: func(p *Parser) error {
				cmd1 := &Command{
					Name: "cmd1",
					Callback: func(cmdLine *Parser, command *Command) error {
						return fmt.Errorf("cmd1 failed")
					},
				}
				cmd2 := &Command{
					Name: "cmd2",
					Callback: func(cmdLine *Parser, command *Command) error {
						return fmt.Errorf("cmd2 failed")
					},
				}
				_ = p.AddCommand(cmd1)
				return p.AddCommand(cmd2)
			},
			args:       []string{"cmd1", "cmd2"},
			execMethod: "all",
			wantErrs: map[string]string{
				"cmd1": "cmd1 failed",
				"cmd2": "cmd2 failed",
			},
			description: "Should execute all commands via ExecuteCommands",
		},
		{
			name: "execute on parse",
			setupFunc: func(p *Parser) error {
				p.SetExecOnParse(true)
				cmd := &Command{
					Name: "test",
					Callback: func(cmdLine *Parser, command *Command) error {
						return fmt.Errorf("test failed")
					},
				}
				return p.AddCommand(cmd)
			},
			args:        []string{"test"},
			execMethod:  "onParse",
			wantErrs:    map[string]string{"test": "test failed"},
			description: "Should execute commands during Parse when ExecOnParse is true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			err := tt.setupFunc(p)
			assert.NoError(t, err)

			success := p.Parse(tt.args)

			// Execute commands based on method
			switch tt.execMethod {
			case "single":
				err = p.ExecuteCommand()
				assert.Error(t, err)
				// Verify errors via GetCommandExecutionErrors
				cmdErrs := p.GetCommandExecutionErrors()
				assert.Equal(t, len(tt.wantErrs), len(cmdErrs))
				for _, kv := range cmdErrs {
					expectedErr, exists := tt.wantErrs[kv.Key]
					assert.True(t, exists, "Unexpected command error for %s", kv.Key)
					assert.Contains(t, kv.Value.Error(), expectedErr)
				}
			case "all":
				errCount := p.ExecuteCommands()
				assert.Equal(t, len(tt.wantErrs), errCount)
				// Verify errors via GetCommandExecutionErrors
				cmdErrs := p.GetCommandExecutionErrors()
				assert.Equal(t, len(tt.wantErrs), len(cmdErrs))
				for _, kv := range cmdErrs {
					expectedErr, exists := tt.wantErrs[kv.Key]
					assert.True(t, exists, "Unexpected command error for %s", kv.Key)
					assert.Contains(t, kv.Value.Error(), expectedErr)
				}
			case "onParse":
				// For ExecOnParse, check parser errors instead
				assert.False(t, success, "Parse should fail due to command error")
				parserErrs := p.GetErrors()
				assert.Equal(t, len(tt.wantErrs), len(parserErrs))
				for cmdName, expectedErr := range tt.wantErrs {
					found := false
					for _, err := range parserErrs {
						if strings.Contains(err.Error(), expectedErr) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected error for command %s not found", cmdName)
				}
			}
		})
	}
}

func TestParser_NestedCommandFlags(t *testing.T) {
	type CommandWithFlag struct {
		Command struct {
			Flag string `goopt:"name:flag"`
		} `goopt:"kind:command;name:command"`
	}

	type NestedCommands struct {
		Parent struct {
			Child struct {
				Flag string `goopt:"name:flag"`
			} `goopt:"kind:command;name:child"`
		} `goopt:"kind:command;name:parent"`
	}

	type CommandWithPath struct {
		Command struct {
			Flag string `goopt:"name:flag;path:other"`
		} `goopt:"kind:command;name:command"`
	}

	type CommandWithNested struct {
		Command struct {
			Nested struct {
				Flag string `goopt:"name:flag"`
			}
		} `goopt:"kind:command;name:command"`
	}

	type CommandWithSlice struct {
		Command struct {
			Items []struct {
				Flag string `goopt:"name:flag"`
			}
		} `goopt:"kind:command;name:command"`
	}

	t.Run("flag nested under command", func(t *testing.T) {
		config := &CommandWithFlag{}
		parser, err := NewParserFromStruct(config)
		assert.NoError(t, err)

		success := parser.Parse([]string{"command", "--flag", "value"})
		assert.True(t, success)

		value, found := parser.Get("flag", "command")
		assert.True(t, found, "Value should be found")
		assert.Equal(t, "value", value, "Flag should be scoped to command")

		_, found = parser.Get("flag")
		assert.False(t, found, "Flag should not be accessible outside command context")
	})

	t.Run("deeply nested flag under commands", func(t *testing.T) {
		config := &NestedCommands{}
		parser, err := NewParserFromStruct(config)
		assert.NoError(t, err)

		success := parser.Parse([]string{"parent", "child", "--flag", "value"})
		assert.True(t, success)

		value, found := parser.Get("flag", "parent child")
		assert.True(t, found, "Value should be found")
		assert.Equal(t, "value", value, "Flag should be scoped to parent child command path")

		_, found = parser.Get("flag")
		assert.False(t, found, "Flag should not be accessible outside command context")
	})

	t.Run("nested flag with explicit path overrides command nesting", func(t *testing.T) {
		config := &CommandWithPath{}
		parser, err := NewParserFromStruct(config)
		assert.NoError(t, err)

		success := parser.Parse([]string{"other", "--flag", "value"})
		assert.True(t, success)

		value, found := parser.Get("flag@other")
		assert.True(t, found, "Value should be found")
		assert.Equal(t, "value", value, "Explicit path should override structural nesting")

		_, found = parser.Get("flag")
		assert.False(t, found, "Flag should not be accessible outside command context")
	})

	t.Run("nested flag structure under command", func(t *testing.T) {
		config := &CommandWithNested{}
		parser, err := NewParserFromStruct(config)
		assert.NoError(t, err)

		success := parser.Parse([]string{"command", "--nested.flag", "value"})
		assert.True(t, success)

		value, found := parser.Get("nested.flag", "command")
		assert.True(t, found, "Value should be found")
		assert.Equal(t, "value", value, "Nested flag structure should maintain dot notation and command scope")

		_, found = parser.Get("nested.flag")
		assert.False(t, found, "Flag should not be accessible outside command context")
	})

	t.Run("slice of structs under command", func(t *testing.T) {
		config := &CommandWithSlice{}
		// Initialize the slice with one element
		config.Command.Items = make([]struct {
			Flag string `goopt:"name:flag"`
		}, 1)

		parser, err := NewParserFromStruct(config)
		assert.NoError(t, err)

		success := parser.Parse([]string{"command", "--items.0.flag", "value"})
		assert.True(t, success, "Parse should succeed")

		value, found := parser.Get("items.0.flag", "command")
		assert.True(t, found, "Value should be found")
		assert.Equal(t, "value", value, "Slice elements should maintain index notation and command scope")
	})
}
func TestParser_NestedSlicePathRegex(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"test.0.inner", true},           // Basic slice -> struct
		{"test.0.inner.1.more", true},    // Two levels: slice -> struct -> slice -> struct
		{"test.0.inner.1.more.2", false}, // Invalid: ends with index
		{"simple", false},                // Not a slice path
		{"test.notanumber.inner", false}, // Invalid: not a number
		{"test.0", false},                // Invalid: ends with index
		{"test.0.", false},               // Invalid: incomplete
		{".0.test", false},               // Invalid: starts with index
		{"test.0.inner.1", false},        // Invalid: ends with index
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isNestedSlicePath(tt.path)
			assert.Equal(t, tt.expected, result, "Path: %s", tt.path)
		})
	}
}

func TestParser_ValidateSlicePath(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Parser)
		path      string
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid single level slice access",
			setup: func(p *Parser) {
				p.acceptedFlags.Set("items", &FlagInfo{
					Argument: &Argument{Capacity: 3},
				})
				p.acceptedFlags.Set("items.0", &FlagInfo{})
				p.acceptedFlags.Set("items.1", &FlagInfo{})
				p.acceptedFlags.Set("items.2", &FlagInfo{})
			},
			path: "items.1",
		},
		{
			name: "index out of bounds",
			setup: func(p *Parser) {
				p.acceptedFlags.Set("items", &FlagInfo{
					Argument: &Argument{Capacity: 3},
				})
			},
			path:      "items.3",
			wantError: true,
			errorMsg:  "index 3 out of bounds at 'items.3': valid range is 0-2",
		},
		{
			name: "nested valid path",
			setup: func(p *Parser) {
				p.acceptedFlags.Set("outer", &FlagInfo{
					Argument: &Argument{Capacity: 2},
				})
				p.acceptedFlags.Set("outer.0", &FlagInfo{})
				p.acceptedFlags.Set("outer.0.inner", &FlagInfo{
					Argument: &Argument{Capacity: 3},
				})
				p.acceptedFlags.Set("outer.0.inner.1", &FlagInfo{})
			},
			path: "outer.0.inner.1",
		},
		{
			name: "nested out of bounds",
			setup: func(p *Parser) {
				p.acceptedFlags.Set("outer", &FlagInfo{
					Argument: &Argument{Capacity: 2},
				})
				p.acceptedFlags.Set("outer.0", &FlagInfo{})
				p.acceptedFlags.Set("outer.0.inner", &FlagInfo{
					Argument: &Argument{Capacity: 3},
				})
			},
			path:      "outer.0.inner.3",
			wantError: true,
			errorMsg:  "index 3 out of bounds at 'outer.0.inner.3': valid range is 0-2",
		},
		{
			name: "negative index",
			setup: func(p *Parser) {
				p.acceptedFlags.Set("items", &FlagInfo{
					Argument: &Argument{Capacity: 3},
				})
			},
			path:      "items.-1",
			wantError: true,
			errorMsg:  "index -1 out of bounds",
		},
		{
			name: "missing capacity",
			setup: func(p *Parser) {
				p.acceptedFlags.Set("items", &FlagInfo{
					Argument: &Argument{}, // No capacity set
				})
			},
			path:      "items.0",
			wantError: true,
			errorMsg:  "has no capacity set",
		},
		{
			name: "unknown path",
			setup: func(p *Parser) {
				// No flags registered
			},
			path:      "unknown.0",
			wantError: true,
			errorMsg:  "unknown flag: unknown.0", // Updated to match actual error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			tt.setup(p)

			err := p.validateSlicePath(tt.path)

			if tt.wantError {
				if err == nil {
					t.Errorf("validateSlicePath() error = nil, want error")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("validateSlicePath() error = %v, want error containing %q", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("validateSlicePath() unexpected error = %v", err)
			}
		})
	}
}

// Alternative non-regex implementation
func isNestedSlicePathSimple(path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return false
	}

	// Must start with field name
	if _, err := strconv.Atoi(parts[0]); err == nil {
		return false
	}

	// Check alternating pattern: field.number.field...
	for i := 1; i < len(parts); i += 2 {
		// Even positions (1,3,5...) must be numbers
		if _, err := strconv.Atoi(parts[i]); err != nil {
			return false
		}
		// Odd positions after must be field names
		if i+1 < len(parts) {
			if _, err := strconv.Atoi(parts[i+1]); err == nil {
				return false
			}
		}
	}

	// Must end with field name
	return len(parts)%2 == 1
}

func BenchmarkPathValidation(b *testing.B) {
	paths := []string{
		"test.0.inner",
		"test.0.inner.1.more",
		"test.0.inner.1.more.field",
		"simple",
		"test.notanumber.inner",
		"test.0",
		"test.0.",
		".0.test",
		"really.0.deep.1.nested.2.path.3.structure.4.test",
	}

	b.Run("Regex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, path := range paths {
				isNestedSlicePath(path)
			}
		}
	})

	b.Run("Simple", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, path := range paths {
				isNestedSlicePathSimple(path)
			}
		}
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
