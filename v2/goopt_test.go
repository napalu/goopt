package goopt

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/internal/testutil"
	"github.com/napalu/goopt/v2/internal/util"
	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"

	"golang.org/x/text/language"
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

	_ = opts.AddFlag("test2", NewArg(WithShortFlag("t2")))

	err := opts.AcceptPattern("test2", types.PatternValue{Pattern: `^[0-9]+$`, Description: "whole integers only"})
	assert.Nil(t, err, "constraint violation - 'Single' flags take values and therefore should PatternValue")
	assert.True(t, opts.Parse([]string{"--test2", "12344"}), "test2 should accept values which match whole integer patterns")
}

func TestParser_AcceptPatterns(t *testing.T) {
	opts := NewParser()

	_ = opts.AddFlag("test", NewArg(WithShortFlag("t")))

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

	_ = opts.AddFlag("upper", NewArg(WithShortFlag("t")))
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

	_ = opts.AddFlag("status", NewArg(WithShortFlag("t")))
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
	opts.ClearErrors()
	assert.True(t, opts.Parse([]string{"--status", "active"}), "parse should not fail and pass PatternValue properly")

	value, _ := opts.Get("status")
	assert.Equal(t, "-1", value, "the value of flag status should have been transformed to -1 after PatternValue validation")
}

func TestParser_DependsOnFlagValue(t *testing.T) {
	opts := NewParser()

	_ = opts.AddFlag("main", NewArg(WithShortFlag("m")))
	_ = opts.AddFlag("dependent", NewArg(WithShortFlag("d")))

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
	err := opts.BindFlag(s, "test", NewArg(WithShortFlag("t")))
	assert.NotNil(t, err, "should not accept non-pointer type in BindFlag")

	err = opts.BindFlag(&s, "test", NewArg(WithShortFlag("t")))
	assert.Nil(t, err, "should accept string pointer type in BindFlag")

	err = opts.BindFlag(&i, "test1", NewArg(WithShortFlag("t1")))
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
	}, "test1", NewArg(WithShortFlag("t1")))
	assert.NotNil(t, err, "should not attempt to bind unsupported struct")

	assert.True(t, opts.ParseString("--test1 2"), "should parse a command line argument when given a bound variable")

	opts = NewParser()
	var boolBind bool
	err = opts.BindFlag(&boolBind, "test", NewArg(WithShortFlag("t"), WithType(types.Standalone)))
	assert.Nil(t, err, "should accept Standalone flags in BindFlag if the data type is boolean")

	opts = NewParser()
	err = opts.BindFlag(&i, "test", NewArg(WithShortFlag("t"), WithType(types.Standalone)))
	assert.NotNil(t, err, "should not accept Standalone flags in BindFlag if the data type is not boolean")
	err = opts.BindFlag(&boolBind, "test", NewArg(WithShortFlag("t"), WithType(types.Standalone)))
	assert.Nil(t, err, "should allow adding field if not yet specified")
	err = opts.BindFlag(&boolBind, "test", NewArg(WithShortFlag("t"), WithType(types.Standalone)))
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
	IsTest       bool   `goopt:"short:t;desc:test bool option;required:true;path:create user type,create group type"`
	IntOption    int    `goopt:"short:i;desc:test int option;default:-20"`
	StringOption string `goopt:"short:so;desc:test string option;type:single;default:1"`
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
		cmd, err := NewParserFromStruct(&testOpt)
		assert.Nil(t, err)
		assert.True(t, cmd.ParseString("create user type -t --stringOption one"), "parse should succeed when a command-specific flag is given and the associated command is specified")
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
	City    string `typeOf:"Single" goopt:"name:city;desc:City name"`
	ZipCode string `typeOf:"Single" goopt:"name:zipcode;desc:ZIP code"`
}

type UserProfile struct {
	Name      string    `typeOf:"Single" goopt:"name:name;short:n;desc:Full name"`
	Age       int       `typeOf:"Single" goopt:"name:age;short:a;desc:Age of user"`
	Addresses []Address `goopt:"name:address"`
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
	flagFunc := cmdLine.SetEnvNameConverter(upperSnakeToCamelCase)
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
	_ = cmdLine.SetEnvNameConverter(upperSnakeToCamelCase)

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
	flagFunc := opts.SetEnvNameConverter(upperSnakeToCamelCase)
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
	flagFunc := opts.SetEnvNameConverter(upperSnakeToCamelCase)

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
	flagFunc := opts.SetEnvNameConverter(upperSnakeToCamelCase)
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

	}, "test1", NewArg(WithShortFlag("t1")))

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
	}, "test1", NewArg(WithShortFlag("t1")))
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
	var (
		cmdLine *Parser
		err     error
	)
	setup := func() {
		cmdLine, err = NewParserWith(
			WithFlag("flagWithValue",
				NewArg(
					WithShortFlag("fw"),
					WithType(types.Single),
					WithDescription("this flag requires a value"),
					WithDependentFlags([]string{"flagA", "flagB"}),
					WithRequired(true))),
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
	}

	setup()
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
	setup()
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
	assert.True(t, errors.Is(err, errs.ErrPosixShortForm))
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
				WithRequired(true))),
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
	var cmdLine *Parser
	setup := func() {
		cmdLine, _ = NewParserWith(
			WithFlag("flagA",
				NewArg(
					WithShortFlag("fa"),
					WithType(types.Standalone))),
			WithFlag("flagB",
				NewArg(
					WithShortFlag("fb"),
					WithType(types.Single))))
	}

	setup()
	// Test valid boolean flag value
	assert.True(t, cmdLine.ParseString("-fa false -fb hello"), "should properly parse a command-line with explicitly "+
		"set boolean flag value among other values")
	boolValue, err := cmdLine.GetBool("fa")
	assert.Nil(t, err, "boolean conversion of 'false' string value should not result in error")
	assert.False(t, boolValue, "the user-supplied false value of a boolean flag should be respected")
	assert.Equal(t, cmdLine.GetOrDefault("fb", ""), "hello", "Single flag in command-line "+
		"with explicitly set boolean flag should have the correct value")

	// Test invalid boolean flag value
	setup()
	assert.True(t, cmdLine.ParseString("-fa ouch -fb hello"), "should parse a command-line with a valid boolean flag value")
	_, err = cmdLine.GetBool("fa")
	assert.Nil(t, err)
	assert.Equal(t, 1, cmdLine.GetPositionalArgCount(), "should have 1 positional argument")
	assert.Equal(t, cmdLine.GetPositionalArgs()[0].Value, "ouch", "should have registered invalid boolean flag value as positional argument")

	setup()
	assert.True(t, cmdLine.ParseString("-fa false -fb hello"), "should parse a command-line with a valid boolean flag value")
	assert.Equal(t, 0, cmdLine.GetPositionalArgCount(), "should have 1 positional argument")
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

 --help "Display help" (optional)

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
	_ = p.AddFlag("global-flag", NewArg(WithShortFlag("g"), WithDescription("A global flag")))
	_ = p.AddFlag("verbose", NewArg(WithShortFlag("v"), WithDescription("Verbose output"), WithType(types.Standalone)))

	// Add command with flags
	cmd := &Command{
		Name:        "test",
		Description: "Test command",
	}
	_ = p.AddCommand(cmd)
	_ = p.AddFlag("test-flag", NewArg(WithShortFlag("t"), WithDescription("A test flag")), "test")

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
			err := opts.AddFlag("number", NewArg(WithShortFlag("n")))
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
			err = opts.AddFlag("number", NewArg(WithShortFlag("n")), "create user")
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
				err := opts.AddFlag("list", NewArg(WithShortFlag("l"), WithType(tt.flagType)))
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
	err = opts.AddFlag("list", NewArg(WithShortFlag("l"), WithType(types.Chained)))
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

			err := opts.AddFlag("list", NewArg(WithShortFlag("l"), WithType(types.Chained)))
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

			err = opts.AddFlag("list", NewArg(WithShortFlag("l"), WithType(types.Chained)), tt.cmdPath...)
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
			name:      "pre-validation filter",
			preFilter: strings.ToUpper,
			input:     "test",
			want:      "TEST",
		},
		{
			name:       "post-validation filter",
			postFilter: strings.TrimSpace,
			input:      " test ",
			want:       "test",
		},
		{
			name:       "both filters",
			preFilter:  strings.ToUpper,
			postFilter: strings.TrimSpace,
			input:      " test ",
			want:       "TEST",
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
			err := opts.AddFlag("test", NewArg(WithShortFlag("t")))
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
			err := opts.AddFlag("test", NewArg(WithShortFlag("t")))
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
				err := p.AddFlag("flag1", NewArg(WithShortFlag("f1")))
				if err != nil {
					return err
				}
				err = p.AddFlag("flag2", NewArg(WithShortFlag("f2")))
				if err != nil {
					return err
				}

				return p.SetArgument("flag2", nil, WithRequiredIf(func(cmdLine *Parser, optionName string) (bool, string) {
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
				_ = p.AddFlag("main", NewArg(WithShortFlag("m")))
				_ = p.AddFlag("dependent", NewArg(WithShortFlag("d")))
				return p.AddDependency("dependent", "main")
			},
			input:     "--dependent test",
			wantParse: true,
			wantWarns: []string{"Flag 'dependent' depends on 'main' which was not specified."},
		},
		{
			name: "value dependency - single value",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("mode", NewArg(WithShortFlag("m")))
				_ = p.AddFlag("debug", NewArg(WithShortFlag("d")))
				return p.AddDependencyValue("debug", "mode", []string{"development"})
			},
			input:     "--mode production --debug true",
			wantParse: true,
			wantWarns: []string{"Flag 'debug' depends on 'mode' with value 'development' which was not specified. (got 'production')"},
		},
		{
			name: "value dependency - multiple values",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("mode", NewArg(WithShortFlag("m")))
				_ = p.AddFlag("debug", NewArg(WithShortFlag("d")))
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
				_ = p.AddFlag("cmdMain", NewArg(WithShortFlag("m")))
				_ = p.AddFlag("cmdDependent", NewArg(WithShortFlag("d")))
				return p.AddDependency("cmdDependent", "cmdMain", "cmd")
			},
			input:     "cmd --cmdDependent test",
			wantParse: true,
			wantWarns: []string{"Flag 'cmdDependent' depends on 'cmdMain' which was not specified."},
		},
		{
			name: "dependency using short form",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArg(WithShortFlag("m")))
				_ = p.AddFlag("dependent", NewArg(WithShortFlag("d")))
				return p.AddDependency("d", "m") // using short forms
			},
			input:     "-d test",
			wantParse: true,
			wantWarns: []string{"Flag 'dependent' depends on 'main' which was not specified."},
		},
		{
			name: "mixed form dependencies",
			setupFunc: func(p *Parser) error {
				_ = p.AddFlag("main", NewArg(WithShortFlag("m")))
				_ = p.AddFlag("dependent", NewArg(WithShortFlag("d")))
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
				_ = p.AddFlag("main", NewArg(WithShortFlag("m")))
				_ = p.AddFlag("dependent", NewArg(WithShortFlag("d")))
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
				_ = p.AddFlag("cmdMain", NewArg(WithShortFlag("m")), "cmd subcmd")
				_ = p.AddFlag("cmdDependent", NewArg(WithShortFlag("d")), "cmd subcmd")
				return p.AddDependency("d", "m", "cmd subcmd") // using short forms with command
			},
			input:     "cmd subcmd -d test",
			wantParse: true,
			wantWarns: []string{"Flag 'cmdDependent@cmd subcmd' depends on 'cmdMain@cmd subcmd' which was not specified."},
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
			gotName, gotPath, err := unmarshalTagsToArgument(nil, structField, arg)
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

			if !arg.Equal(&tt.field.WantArg) {
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
		wantErrs    []error
		description string
	}{
		{
			name: "circular dependency",
			setupFunc: func(p *Parser) *FlagInfo {
				flag1 := NewArg()
				flag1.DependencyMap = map[string][]string{"flag2": {""}}
				_ = p.AddFlag("flag1", flag1)

				flag2 := NewArg()
				flag2.DependencyMap = map[string][]string{"flag1": {""}}
				_ = p.AddFlag("flag2", flag2)

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []error{errs.ErrCircularDependency.WithArgs("flag1", "flag2")},
			description: "Should detect direct circular dependencies",
		},
		{
			name: "indirect circular dependency",
			setupFunc: func(p *Parser) *FlagInfo {
				flag1 := NewArg()
				flag1.DependencyMap = map[string][]string{"flag2": {""}}
				_ = p.AddFlag("flag1", flag1)

				flag2 := NewArg()
				flag2.DependencyMap = map[string][]string{"flag3": {""}}
				_ = p.AddFlag("flag2", flag2)

				flag3 := NewArg()
				flag3.DependencyMap = map[string][]string{"flag1": {""}}
				_ = p.AddFlag("flag3", flag3)

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []error{errs.ErrCircularDependency.WithArgs("flag1", "flag2", "flag3")},
			description: "Should detect indirect circular dependencies",
		},
		{
			name: "max depth exceeded",
			setupFunc: func(p *Parser) *FlagInfo {
				// Create chain of MaxDependencyDepth+2 flags where each depends on the next
				maxDepth := p.GetMaxDependencyDepth() + 2
				for i := 1; i <= maxDepth; i++ {
					flag := NewArg()
					if i < maxDepth {
						flag.DependencyMap = map[string][]string{fmt.Sprintf("flag%d", i+1): {""}}
					}
					_ = p.AddFlag(fmt.Sprintf("flag%d", i), flag)
				}

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []error{errs.ErrRecursionDepthExceeded.WithArgs("flag1")},
			description: "Should detect when dependency chain exceeds max depth",
		},
		{
			name: "missing dependent flag",
			setupFunc: func(p *Parser) *FlagInfo {
				flag := NewArg()
				flag.DependencyMap = map[string][]string{"nonexistent": {""}}
				_ = p.AddFlag("flag1", flag)

				flagInfo, _ := p.acceptedFlags.Get("flag1")
				return flagInfo
			},
			mainKey:     "flag1",
			wantErrs:    []error{errs.ErrDependencyNotFound.WithArgs("flag1", "nonexistent")},
			description: "Should detect when dependent flag doesn't exist",
		},
		{
			name: "valid simple dependency",
			setupFunc: func(p *Parser) *FlagInfo {
				flag1 := NewArg()
				flag1.DependencyMap = map[string][]string{"flag2": {""}}
				_ = p.AddFlag("flag1", flag1)

				flag2 := NewArg()
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
						testutil.AssertError(t, errs[i], wantErr)
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
	testBundle := i18n.Default()
	sentinel := i18n.NewError("err.test_failed")
	err := testBundle.AddLanguage(language.English, map[string]string{
		"goopt.error.command_callback_error": "command failed '%[1]s'",
		"err.test_failed":                    "test failed '%[1]s'",
		"err.cmd1_failed":                    "cmd1 failed '%[1]s'",
		"err.cmd2_failed":                    "cmd2 failed '%[1]s'",
	})
	if err != nil {
		t.Fatalf("failed to add language: %v", err)
	}
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
						return sentinel.WithArgs(command.Name)
					},
				}
				return p.AddCommand(cmd)
			},
			args:        []string{"test"},
			execMethod:  "single",
			wantErrs:    map[string]string{"test": "test failed 'test'"},
			description: "Should execute single command via ExecuteCommand",
		},
		{
			name: "execute all commands",
			setupFunc: func(p *Parser) error {
				cmd1 := &Command{
					Name: "cmd1",
					Callback: func(cmdLine *Parser, command *Command) error {
						return errs.ErrCommandCallbackError.WithArgs(command.Name)
					},
				}
				cmd2 := &Command{
					Name: "cmd2",
					Callback: func(cmdLine *Parser, command *Command) error {
						return errs.ErrCommandCallbackError.WithArgs(command.Name)
					},
				}
				_ = p.AddCommand(cmd1)
				return p.AddCommand(cmd2)
			},
			args:       []string{"cmd1", "cmd2"},
			execMethod: "all",
			wantErrs: map[string]string{
				"cmd1": "command failed 'cmd1'",
				"cmd2": "command failed 'cmd2'",
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
						return errs.ErrCommandCallbackError.WithArgs(command.Name).Wrap(errors.New("custom error"))
					},
				}
				return p.AddCommand(cmd)
			},
			args:       []string{"test"},
			execMethod: "onParse",
			wantErrs: map[string]string{
				"test": "command failed 'test': custom error",
			},
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
					expectedKey := tt.wantErrs[kv.Key]
					expectedMsg := testBundle.T(expectedKey)
					renderedMsg := kv.Value.Error()
					assert.Equal(t, expectedMsg, renderedMsg)
				}
			case "all":
				errCount := p.ExecuteCommands()
				assert.Equal(t, len(tt.wantErrs), errCount)
				// Verify errors via GetCommandExecutionErrors
				cmdErrs := p.GetCommandExecutionErrors()
				assert.Equal(t, len(tt.wantErrs), len(cmdErrs))
				for _, kv := range cmdErrs {
					expectedKey := tt.wantErrs[kv.Key]
					expectedMsg := testBundle.T(expectedKey)
					renderedMsg := kv.Value.Error()
					assert.Equal(t, expectedMsg, renderedMsg)
				}
			case "onParse":
				// For ExecOnParse, check parser errors instead
				assert.False(t, success, "Parse should fail due to command error")
				parserErrs := p.GetErrors()
				assert.Equal(t, len(tt.wantErrs), len(parserErrs))
				for cmdName, expectedErr := range tt.wantErrs {
					found := false
					for _, err := range parserErrs {
						renderedMsg := err.Error()
						if renderedMsg == expectedErr {
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
		err       error
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
			err:       errs.ErrIndexOutOfBounds.WithArgs("items.3", 3, 2),
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
			err:       errs.ErrIndexOutOfBounds.WithArgs("outer.0.inner.3", 3, 2),
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
			err:       errs.ErrIndexOutOfBounds.WithArgs("items.-1", 3, 2),
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
			err:       errs.ErrNegativeCapacity.WithArgs("items.0", 0),
		},
		{
			name: "unknown path",
			setup: func(p *Parser) {
				// No flags registered
			},
			path:      "unknown.0",
			wantError: true,
			err:       errs.ErrUnknownFlag.WithArgs("unknown.0"),
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
				} else if !errors.Is(err, tt.err) {
					t.Errorf("validateSlicePath() error = %v, want error containing %q", err, tt.err)
				}
				return
			}

			if err != nil {
				t.Errorf("validateSlicePath() unexpected error = %v", err)
			}
		})
	}
}

func TestParser_PositionalArgumentsWithFlags(t *testing.T) {
	opts := NewParser()

	// Add a standalone flag
	_ = opts.AddFlag("verbose", &Argument{Description: "Verbose output", TypeOf: types.Standalone})

	// Parse a command line with flags and positional arguments
	assert.True(t, opts.ParseString("--verbose posArg1 posArg2"), "should parse flags and positional arguments")

	// Check positional arguments
	posArgs := opts.GetPositionalArgs()
	assert.Equal(t, 2, len(posArgs), "should have two positional arguments")
	assert.Equal(t, "posArg1", posArgs[0].Value)
	assert.Equal(t, "posArg2", posArgs[1].Value)
}

func TestParser_PositionalArgumentsWithCommands(t *testing.T) {
	opts := NewParser()

	// Add a command
	_ = opts.AddCommand(&Command{Name: "create"})

	// Parse a command line with commands and positional arguments
	assert.True(t, opts.ParseString("create posArg1 posArg2"), "should parse commands and positional arguments")

	// Check positional arguments
	posArgs := opts.GetPositionalArgs()
	assert.Equal(t, 2, len(posArgs), "should have two positional arguments")
	assert.Equal(t, "posArg1", posArgs[0].Value)
	assert.Equal(t, "posArg2", posArgs[1].Value)
}

func TestParser_PositionalArgumentsWithCommandSpecificFlags(t *testing.T) {
	opts := NewParser()

	// Add a command with a specific flag
	_ = opts.AddCommand(&Command{Name: "create"})
	_ = opts.AddFlag("name", &Argument{Description: "Name of the entity", TypeOf: types.Single}, "create")

	// Parse a command line with command-specific flags and positional arguments
	assert.True(t, opts.ParseString("create --name entityName posArg1 posArg2"), "should parse command-specific flags and positional arguments")

	// Check positional arguments
	posArgs := opts.GetPositionalArgs()
	assert.Equal(t, 2, len(posArgs), "should have two positional arguments")
	assert.Equal(t, "posArg1", posArgs[0].Value)
	assert.Equal(t, "posArg2", posArgs[1].Value)
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

func TestParser_PositionalArguments(t *testing.T) {
	tests := []struct {
		name          string
		args          string
		setupParser   func(*Parser)
		wantPositions []PositionalArgument
		wantErr       bool
		wantedErr     error
	}{
		{
			name: "basic positional",
			args: "source.txt --verbose dest.txt",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("source", NewArg(
					WithPosition(0),
				))
				_ = p.AddFlag("verbose", NewArg(WithType(types.Standalone)))
				_ = p.AddFlag("dest", NewArg(WithPosition(1)))
			},
			wantPositions: []PositionalArgument{
				{Position: 0, Value: "source.txt"},
				{Position: 2, Value: "dest.txt"}, // Unbound positional
			},
		},
		{
			name: "sequential positions",
			args: "--verbose source.txt dest.txt",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("dest", NewArg(
					WithPosition(1),
				))
				_ = p.AddFlag("source", NewArg(WithPosition(0)))
				_ = p.AddFlag("verbose", NewArg(WithType(types.Standalone)))
			},
			wantPositions: []PositionalArgument{
				{Position: 1, Value: "source.txt"},
				{Position: 2, Value: "dest.txt"},
			},
		},
		{
			name: "flag override of positional",
			args: "--valueFlag value --source override.txt dest.txt",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("valueFlag", NewArg())
				_ = p.AddFlag("source", NewArg(
					WithPosition(0),
				))
				_ = p.AddFlag("dest", NewArg(WithPosition(1)))
			},
			wantPositions: []PositionalArgument{
				{Position: 4, Value: "dest.txt"},
			},
		},
		{
			name: "missing required positional",
			args: "--verbose dest.txt",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("source", NewArg(
					WithPosition(0),
					WithRequired(true),
				))
				_ = p.AddFlag("dest", NewArg(WithPosition(1), WithRequired(true)))
				_ = p.AddFlag("verbose", NewArg(WithType(types.Standalone)))
			},
			wantErr:   true,
			wantedErr: errs.ErrRequiredPositionalFlag.WithArgs("dest", 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			tt.setupParser(p)

			ok := p.ParseString(tt.args)
			if tt.wantErr {
				assert.False(t, ok)
				errs := p.GetErrors()
				assert.NotEmpty(t, errs)
				if len(errs) > 0 {
					testutil.AssertError(t, errs[0], tt.wantedErr)
				}
				return
			}

			assert.True(t, ok)
			pos := p.GetPositionalArgs()

			assert.Equal(t, len(tt.wantPositions), len(pos))

			for i := 0; i < util.Min(len(tt.wantPositions), len(pos)); i++ {
				assert.Equal(t, tt.wantPositions[i].Position, pos[i].Position,
					"Position mismatch at index %d", i)
				assert.Equal(t, tt.wantPositions[i].Value, pos[i].Value,
					"Value mismatch at index %d", i)
			}
		})
	}
}

func TestNewParserFromStruct_NestedPointerStructs(t *testing.T) {
	type Inner struct {
		InnerField string `goopt:"name:inner-field;short:i;desc:inner field description"`
	}

	type Middle struct {
		MiddleField string  `goopt:"name:middle-field;short:m;desc:middle field description"`
		Inner       *Inner  `goopt:"name:inner"`
		InnerPtr    *string `goopt:"name:inner-ptr;desc:pointer to string"`
	}

	type Config struct {
		TopField string  `goopt:"name:top-field;short:t;desc:top field description"`
		Middle   *Middle `goopt:"name:middle"`
		NilPtr   *Inner  `goopt:"name:nil-ptr"`
	}

	str := "test"
	cfg := &Config{
		Middle: &Middle{
			Inner:    &Inner{},
			InnerPtr: &str,
		},
	}

	parser, err := NewParserFromStruct(cfg)
	assert.NoError(t, err)

	// Check that flags were properly registered
	arg1, err := parser.GetArgument("top-field")
	assert.NoError(t, err)
	assert.NotNil(t, arg1)

	arg2, err := parser.GetArgument("middle.middle-field")
	assert.NoError(t, err)
	assert.NotNil(t, arg2)

	arg3, err := parser.GetArgument("middle.inner.inner-field")
	assert.NoError(t, err)
	assert.NotNil(t, arg3)

	arg4, err := parser.GetArgument("middle.inner-ptr")
	assert.NoError(t, err)
	assert.NotNil(t, arg4)

	// Check that nil pointers are handled gracefully
	_, err = parser.GetArgument("nil-ptr.inner-field")
	assert.Error(t, err) // Should error because nil-ptr is nil
}

func TestParser_NestedCommandRegistration(t *testing.T) {
	type Config struct {
		// ... Config fields if needed
	}

	type Options struct {
		Verbose bool `goopt:"name:verbose;short:v;desc:be loud when working;type:standalone;default:false"`
		Csv     struct {
			In          string `goopt:"name:in;short:i;desc:csv input file"`
			Out         string `goopt:"name:out;short:o;desc:csv output;default:stdout"`
			Headers     bool   `goopt:"name:headers;short:h;desc:defaults to true - set to false to indicate csv file has no headers;type:standalone"`
			SearchByPos int    `goopt:"name:searchByPos;short:U;desc:position of field in csv file containing the user id;default:0"`
			GroupPos    int    `goopt:"name:groupPos;short:g;desc:position of field in csv file containing the user groups;default:0"`
			OverridePos int    `goopt:"name:overridePos;short:O;desc:position of field in csv file containing the user id to use if present"`
		}
		Config *Config
		Users  struct {
			Upsert struct {
				Config *Config
			} `goopt:"kind:command"`
			Update     struct{} `goopt:"kind:command"`
			Diff       struct{} `goopt:"kind:command"`
			Deactivate struct {
				Delete bool `goopt:"name:delete;short:dlt;desc:delete users when deactivating;default:false"`
			} `goopt:"kind:command"`
			Search struct {
				WithAttributes bool   `goopt:"name:withAttributes;short:wa;desc:ask for additional attributes;type:standalone"`
				CrowdQuery     string `goopt:"name:crowdQuery;short:q;desc:crowd search query string;required:true"`
			} `goopt:"kind:command"`
		} `goopt:"kind:command"`
		Group struct {
			Members struct{} `goopt:"kind:command"`
			Add     struct{} `goopt:"kind:command"`
			Sync    struct{} `goopt:"kind:command"`
		} `goopt:"kind:command"`
	}

	opts := &Options{
		Config: &Config{},
	}
	parser, err := NewParserFromStruct(opts)
	assert.NoError(t, err)

	// Test global flags
	verboseArg, err := parser.GetArgument("verbose")
	assert.NoError(t, err)
	assert.Equal(t, "v", verboseArg.Short)
	assert.True(t, verboseArg.TypeOf == types.Standalone)

	// Test nested non-command struct flags
	csvInArg, err := parser.GetArgument("csv.in")
	assert.NoError(t, err)
	assert.Equal(t, "i", csvInArg.Short)

	// Test command registration
	expectedCommands := []struct {
		path   string
		fields []string // expected flag names for this command
	}{
		{
			path:   "users upsert",
			fields: nil, // Add fields if any
		},
		{
			path:   "users update",
			fields: nil,
		},
		{
			path:   "users diff",
			fields: nil,
		},
		{
			path: "users deactivate",
			fields: []string{
				"delete",
			},
		},
		{
			path: "users search",
			fields: []string{
				"withAttributes",
				"crowdQuery",
			},
		},
		{
			path:   "group members",
			fields: nil,
		},
		{
			path:   "group add",
			fields: nil,
		},
		{
			path:   "group sync",
			fields: nil,
		},
	}

	for _, cmd := range expectedCommands {
		// Verify command exists
		command, ok := parser.getCommand(cmd.path)

		assert.NotNil(t, command, "Command %s should exist", cmd.path)
		assert.True(t, ok)

		// Verify command flags
		if cmd.fields != nil {
			for _, field := range cmd.fields {
				flagPath := buildPathFlag(field, cmd.path)
				arg, err := parser.GetArgument(flagPath)
				assert.NoError(t, err, "Flag %s should exist", flagPath)
				assert.NotNil(t, arg)
			}
		}
	}

	// Test that non-existent commands return nil
	command, ok := parser.getCommand("nonexistent")
	assert.Nil(t, command)
	assert.False(t, ok)

	command, ok = parser.getCommand("users nonexistent")
	assert.Nil(t, command)
	assert.False(t, ok)
}

func TestParser_ReusableAndMixedFlagPatterns(t *testing.T) {
	// Common reusable structures
	type LogConfig struct {
		Level   string `goopt:"name:level;default:info"`
		Format  string `goopt:"name:format;default:json"`
		Path    string `goopt:"name:path"`
		Verbose bool   `goopt:"name:verbose;type:standalone"`
	}

	type DatabaseConfig struct {
		Host     string `goopt:"name:host;default:localhost"`
		Port     int    `goopt:"name:port;default:5432"`
		User     string `goopt:"name:user;required:true"`
		Password string `goopt:"name:password;required:true"`
	}

	type testConfig struct {
		Primary DatabaseConfig `goopt:"name:primary-db"`
		Replica DatabaseConfig `goopt:"name:replica-db"`
		Logs    LogConfig      `goopt:"name:log"`
	}

	// For the mixed approach test
	type mixedConfig struct {
		Server struct {
			Host string     `goopt:"name:host;default:localhost"`
			Port int        `goopt:"name:port;default:8080"`
			Logs *LogConfig `goopt:"name:logs"`
		}
		Client struct {
			Endpoint string     `goopt:"name:endpoint;required:true"`
			Logs     *LogConfig `goopt:"name:logs"`
		}
	}

	// For the nil pointer test
	type nilPointerConfig struct {
		Server struct {
			Host string     `goopt:"name:host"`
			Logs *LogConfig `goopt:"name:logs"`
		}
	}

	tests := []struct {
		name          string
		setupConfig   func() interface{} // Change back to interface{} to handle different types
		args          []string
		envVars       map[string]string
		validateFunc  func(*testing.T, *Parser)
		expectSuccess bool
	}{
		{
			name: "reusable flag groups with prefixes",
			setupConfig: func() interface{} {
				return &testConfig{}
			},
			args: []string{
				"--primary-db.host", "primary.example.com",
				"--primary-db.user", "admin",
				"--primary-db.password", "secret",
				"--replica-db.host", "replica.example.com",
				"--replica-db.user", "reader",
				"--replica-db.password", "secret2",
				"--log.level", "debug",
			},
			validateFunc: func(t *testing.T, p *Parser) {
				// Check primary DB flags
				_, err := p.GetArgument("primary-db.host")
				assert.NoError(t, err)
				assert.Equal(t, "primary.example.com", p.GetOrDefault("primary-db.host", ""))

				// Check replica DB flags
				_, err = p.GetArgument("replica-db.host")
				assert.NoError(t, err)
				assert.Equal(t, "replica.example.com", p.GetOrDefault("replica-db.host", ""))

				// Check log flags
				_, err = p.GetArgument("log.level")
				assert.NoError(t, err)
				assert.Equal(t, "debug", p.GetOrDefault("log.level", ""))
			},
			expectSuccess: true,
		},
		{
			name: "mixed approach with pointers",
			setupConfig: func() interface{} {
				return &mixedConfig{
					Server: struct {
						Host string     `goopt:"name:host;default:localhost"`
						Port int        `goopt:"name:port;default:8080"`
						Logs *LogConfig `goopt:"name:logs"`
					}{
						Logs: &LogConfig{},
					},
					Client: struct {
						Endpoint string     `goopt:"name:endpoint;required:true"`
						Logs     *LogConfig `goopt:"name:logs"`
					}{
						Logs: &LogConfig{},
					},
				}
			},
			args: []string{
				"--server.host", "api.example.com",
				"--server.logs.level", "debug",
				"--client.endpoint", "https://api.example.com",
				"--client.logs.level", "info",
			},
			validateFunc: func(t *testing.T, p *Parser) {
				// Check server flags
				_, err := p.GetArgument("server.host")
				assert.NoError(t, err)
				assert.Equal(t, "api.example.com", p.GetOrDefault("server.host", ""))

				// Check server logs
				_, err = p.GetArgument("server.logs.level")
				assert.NoError(t, err)
				assert.Equal(t, "debug", p.GetOrDefault("server.logs.level", ""))

				// Check client flags
				_, err = p.GetArgument("client.endpoint")
				assert.NoError(t, err)
				assert.Equal(t, "https://api.example.com", p.GetOrDefault("client.endpoint", ""))

				// Check client logs
				_, err = p.GetArgument("client.logs.level")
				assert.NoError(t, err)
				assert.Equal(t, "info", p.GetOrDefault("client.logs.level", ""))
			},
			expectSuccess: true,
		},
		{
			name: "nil pointer handling",
			setupConfig: func() interface{} {
				return &nilPointerConfig{}
			},
			args: []string{"--server.host", "localhost"},
			validateFunc: func(t *testing.T, p *Parser) {
				// Should handle nil LogConfig gracefully
				_, err := p.GetArgument("server.host")
				assert.NoError(t, err)
				assert.Equal(t, "localhost", p.GetOrDefault("server.host", ""))

				// Logs flags should not be registered
				_, err = p.GetArgument("server.logs.level")
				assert.Error(t, err)
			},
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables if any
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Create parser directly from the config pointer
			config := tt.setupConfig()
			parser, err := NewParserFromInterface(config)
			if !assert.NoError(t, err) {
				return
			}

			// Parse arguments
			success := parser.Parse(tt.args)
			assert.Equal(t, tt.expectSuccess, success)

			// Run validation
			if tt.validateFunc != nil {
				tt.validateFunc(t, parser)
			}
		})
	}
}

func TestParser_NestedCommandFlagsWithSlices(t *testing.T) {
	type CommonConfig struct {
		Host string `goopt:"name:host;short:h"`
	}

	type TestStruct struct {
		Group struct {
			Add struct {
				Config CommonConfig
				Groups []string `goopt:"name:groups;short:g"`
			} `goopt:"kind:command"`
			Remove struct {
				Config CommonConfig
				Groups []string `goopt:"name:groups;short:g"`
			} `goopt:"kind:command"`
		} `goopt:"kind:command"`
	}

	opts := &TestStruct{}
	p, err := NewParserFromStruct(opts)
	assert.NoError(t, err)
	if p.GetErrorCount() > 0 {
		t.Logf("Parse errors: %v", p.GetErrors())
	}
	assert.Empty(t, p.GetErrors())

	tests := []struct {
		args     []string
		wantErr  bool
		expected []string
		host     string
	}{
		{
			args:     []string{"group", "add", "--groups", "group1,group2", "--config.host", "localhost"},
			expected: []string{"group1", "group2"},
			host:     "localhost",
		},
		{
			args:     []string{"group", "remove", "--groups", "group3", "--config.host", "otherhost"},
			expected: []string{"group3"},
			host:     "otherhost",
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			if !p.Parse(tt.args) {
				t.Logf("Parse errors: %v", p.GetErrors())
				assert.Contains(t, p.GetErrors(), tt.wantErr)
			}

			var got []string
			var gotHost string
			switch tt.args[1] {
			case "add":
				got = opts.Group.Add.Groups
				gotHost = opts.Group.Add.Config.Host
			case "remove":
				got = opts.Group.Remove.Groups
				gotHost = opts.Group.Remove.Config.Host
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Groups = %v, want %v", got, tt.expected)
			}
			if gotHost != tt.host {
				t.Errorf("Host = %v, want %v", gotHost, tt.host)
			}
		})
	}
}

func TestParser_PointerToSliceOfStructs(t *testing.T) {
	type Item struct {
		Name  string `goopt:"name:name"`
		Value int    `goopt:"name:value"`
	}

	type TestStruct struct {
		Command struct {
			Items       *[]*Item `goopt:"name:items"`        // Pointer to slice of pointers to structs
			SimpleItems *[]Item  `goopt:"name:simple-items"` // Pointer to slice of structs
		} `goopt:"kind:command"`
	}

	opts := &TestStruct{}

	// Initialize the slices and their elements
	items := make([]*Item, 2)
	items[0] = &Item{} // Initialize each pointer.go
	items[1] = &Item{}

	simpleItems := make([]Item, 2) // Simple items don't need pointer.go initialization

	opts.Command.Items = &items
	opts.Command.SimpleItems = &simpleItems

	p, err := NewParserFromStruct(opts)
	assert.NoError(t, err)
	assert.Empty(t, p.GetErrors())

	assert.True(t, p.Parse([]string{
		"command",
		"--items.0.name", "item1",
		"--items.0.value", "1",
		"--items.1.name", "item2",
		"--items.1.value", "2",
		"--simple-items.0.name", "item3",
		"--simple-items.0.value", "3",
		"--simple-items.1.name", "item4",
		"--simple-items.1.value", "4",
	}))

	assert.Empty(t, p.GetErrors())

	// Verify the values
	assert.Equal(t, "item1", (*opts.Command.Items)[0].Name)
	assert.Equal(t, 1, (*opts.Command.Items)[0].Value)
	assert.Equal(t, "item2", (*opts.Command.Items)[1].Name)
	assert.Equal(t, 2, (*opts.Command.Items)[1].Value)

	assert.Equal(t, "item3", (*opts.Command.SimpleItems)[0].Name)
	assert.Equal(t, 3, (*opts.Command.SimpleItems)[0].Value)
	assert.Equal(t, "item4", (*opts.Command.SimpleItems)[1].Name)
	assert.Equal(t, 4, (*opts.Command.SimpleItems)[1].Value)
}

func TestParser_CheckMultiple(t *testing.T) {
	type Config struct {
		LogLevel string   `goopt:"name:log-level;accepted:{pattern:(?i)^(?:ALL|INFO|ERROR|WARN|DEBUG|NONE)$,desc:Log level}"`
		Format   string   `goopt:"name:format;accepted:{pattern:json|yaml|toml,desc:Log format}"`
		Tags     []string `goopt:"name:tags;accepted:{pattern:^[a-zA-Z0-9_-]+$,desc:Log tags}"` // Fixed pattern to be more strict
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name:    "valid log level",
			args:    []string{"--log-level", "INFO"},
			wantErr: false,
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, "INFO", c.LogLevel)
			},
		},
		{
			name:    "invalid log level",
			args:    []string{"--log-level", "INVALID"},
			wantErr: true,
		},
		{
			name:    "valid format",
			args:    []string{"--format", "json"},
			wantErr: false,
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, "json", c.Format)
			},
		},
		{
			name:    "valid tags",
			args:    []string{"--tags", "tag1,tag2,tag3"},
			wantErr: false,
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, []string{"tag1", "tag2", "tag3"}, c.Tags)
			},
		},
		{
			name:    "invalid tags",
			args:    []string{"--tags", "tag1,@invalid,tag3"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg)
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			if tt.wantErr {
				assert.False(t, ok)
				assert.NotEmpty(t, p.GetErrors())
			} else {
				assert.True(t, ok)
				assert.Empty(t, p.GetErrors())
				if tt.check != nil {
					tt.check(t, cfg)
				}
			}
		})
	}
}

func TestParser_GetCommandExecutionError(t *testing.T) {
	tests := []struct {
		name          string
		setupCmd      *Command
		args          string
		cmdPath       string
		expectedError string
	}{
		{
			name: "no error on successful execution",
			setupCmd: &Command{
				Name: "command",
				Callback: func(cmdLine *Parser, command *Command) error {
					return nil
				},
			},
			args:    "command",
			cmdPath: "command",
		},
		{
			name: "error on command execution",
			setupCmd: &Command{
				Name: "failing",
				Callback: func(cmdLine *Parser, command *Command) error {
					return fmt.Errorf("command failed")
				},
			},
			args:          "failing",
			cmdPath:       "failing",
			expectedError: "command failed",
		},
		{
			name: "error in subcommand",
			setupCmd: &Command{
				Name: "parent",
				Subcommands: []Command{
					{
						Name: "child",
						Callback: func(cmdLine *Parser, command *Command) error {
							return fmt.Errorf("child command failed")
						},
					},
				},
			},
			args:          "parent child",
			cmdPath:       "parent child",
			expectedError: "child command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			err := p.AddCommand(tt.setupCmd)
			assert.NoError(t, err)

			_ = p.ParseString(tt.args)
			_ = p.ExecuteCommands()
			err = p.GetCommandExecutionError(tt.cmdPath)

			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

func TestParser_HasAcceptedValues(t *testing.T) {
	type Config struct {
		LogLevel string   `goopt:"name:log-level;accepted:{pattern:(?i)^(?:INFO|ERROR|WARN)$,desc:Log level}"`
		Format   string   `goopt:"name:format"`
		Tags     []string `goopt:"name:tags;accepted:{pattern:[a-z]+,desc:Tag format}"`
		Command  struct {
			SubFlag string `goopt:"name:sub-flag;accepted:{pattern:value1|value2,desc:Allowed values}"`
		} `goopt:"kind:command"`
	}

	tests := []struct {
		name     string
		flagPath string
		expected bool
	}{
		{
			name:     "flag with accepted values",
			flagPath: "log-level",
			expected: true,
		},
		{
			name:     "flag without accepted values",
			flagPath: "format",
			expected: false,
		},
		{
			name:     "slice flag with accepted values",
			flagPath: "tags",
			expected: true,
		},
		{
			name:     "nested flag with accepted values",
			flagPath: "sub-flag@command",
			expected: true,
		},
		{
			name:     "non-existent flag",
			flagPath: "nonexistent",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg)
			assert.NoError(t, err)
			assert.Empty(t, p.GetErrors(), "Parser should have no errors after initialization")

			hasAccepted := p.HasAcceptedValues(tt.flagPath)
			assert.Equal(t, tt.expected, hasAccepted)
		})
	}
}

func TestParser_PrintFlags(t *testing.T) {
	type Config struct {
		LogLevel string   `goopt:"name:log-level;short:l;desc:Set logging level;required:true"`
		Debug    bool     `goopt:"name:debug;short:d;desc:Enable debug mode"`
		Tags     []string `goopt:"name:tags;desc:List of tags"`
		Hidden   string   `goopt:"name:hidden;desc:Hidden option;ignore"`
	}

	tests := []struct {
		name             string
		expectedOutput   []string
		unexpectedOutput []string
	}{
		{
			name: "all flags",
			expectedOutput: []string{
				"\n --log-level or -l \"Set logging level\" (required)",
				"\n --debug or -d \"Enable debug mode\" (optional)",
				"\n --tags \"List of tags\" (optional)",
			},
			unexpectedOutput: []string{
				"--hidden", // Should not show ignored flags
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg)
			assert.NoError(t, err)

			// Capture output
			var buf bytes.Buffer
			p.PrintFlags(&buf)
			output := buf.String()

			// Check expected output
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected)
			}

			// Check unexpected output
			for _, unexpected := range tt.unexpectedOutput {
				assert.NotContains(t, output, unexpected)
			}
		})
	}
}

func TestParser_Path(t *testing.T) {
	tests := []struct {
		name     string
		setupCmd *Command
		cmdPath  string
	}{
		{
			name: "top level command",
			setupCmd: &Command{
				Name: "create",
			},
			cmdPath: "create",
		},
		{
			name: "nested command",
			setupCmd: &Command{
				Name: "create",
				Subcommands: []Command{
					{
						Name: "user",
					},
				},
			},
			cmdPath: "create user",
		},
		{
			name: "deeply nested command",
			setupCmd: &Command{
				Name: "create",
				Subcommands: []Command{
					{
						Name: "user",
						Subcommands: []Command{
							{
								Name: "admin",
							},
						},
					},
				},
			},
			cmdPath: "create user admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			err := p.AddCommand(tt.setupCmd)
			assert.NoError(t, err)

			args := strings.Split(tt.cmdPath, " ")
			ok := p.Parse(args)
			assert.True(t, ok)

			cmd, found := p.getCommand(tt.cmdPath)
			assert.True(t, found)
			assert.NotNil(t, cmd)
			assert.Equal(t, tt.cmdPath, cmd.Path())
		})
	}
}

func TestParser_SetArgumentPrefixes(t *testing.T) {
	type Config struct {
		Debug bool   `goopt:"kind:flag"`
		Level string `goopt:"kind:flag"`
	}

	tests := []struct {
		name     string
		args     []string
		want     map[string]string
		wantErr  bool
		wantBool bool // expected Parse() result
		prefixes []rune
	}{
		{
			name: "custom prefix flags",
			args: []string{"+debug", "/level", "info"},
			want: map[string]string{
				"debug": "true",
				"level": "info",
			},
			prefixes: []rune{'+', '/'},
			wantBool: true,
		},
		{
			name: "mixed prefix usage",
			args: []string{"+debug", "--level", "info"},
			want: map[string]string{
				"debug": "true",
				"level": "info",
			},
			prefixes: []rune{'-', '+'},
			wantBool: true,
		},
		{
			name: "wrong prefix",
			args: []string{"--debug", "--level", "info"},
			want: map[string]string{
				"debug": "false",
				"level": "info",
			},
			prefixes: nil, // pass nil prefixes
			wantBool: false,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg, WithArgumentPrefixes(tt.prefixes))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.Equal(t, tt.wantBool, ok, "Parse() result mismatch")

			for flag, expectedValue := range tt.want {
				value := p.GetOrDefault(flag, "")
				assert.Equal(t, expectedValue, value, "Value mismatch for flag %s", flag)
			}
		})
	}
}

func TestParser_GetOptions(t *testing.T) {
	type Config struct {
		Debug   bool     `goopt:"name:debug;short:d"`
		Level   string   `goopt:"name:level;short:l"`
		Tags    []string `goopt:"name:tags"`
		Command struct {
			SubFlag string `goopt:"name:sub-flag"`
		} `goopt:"kind:command"`
	}

	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name: "global flags",
			args: []string{"--debug", "--level", "info", "--tags", "tag1,tag2"},
			expected: map[string]string{
				"debug": "true",
				"level": "info",
				"tags":  "tag1,tag2",
			},
		},
		{
			name: "short flags resolved to long form",
			args: []string{"-d", "-l", "debug"},
			expected: map[string]string{
				"debug": "true",
				"level": "debug",
			},
		},
		{
			name: "command flags",
			args: []string{"command", "--sub-flag", "value"},
			expected: map[string]string{
				"sub-flag@command": "value",
			},
		},
		{
			name: "mixed flags with short form resolved",
			args: []string{"-d", "command", "--sub-flag", "value"},
			expected: map[string]string{
				"debug":            "true",
				"sub-flag@command": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg)
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.True(t, ok)

			options := p.GetOptions()
			assert.NotNil(t, options)
			assert.Equal(t, len(tt.expected), len(options))

			// Convert options to map for easier comparison
			optMap := make(map[string]string)
			for _, kv := range options {
				optMap[kv.Key] = kv.Value
			}

			assert.Equal(t, tt.expected, optMap)
		})
	}
}

func TestParser_GetFlagPath(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want string
	}{
		{
			name: "no path",
			flag: "flag",
			want: "",
		},
		{
			name: "simple path",
			flag: "flag@cmd",
			want: "cmd",
		},
		{
			name: "nested path",
			flag: "flag@cmd subcmd",
			want: "cmd subcmd",
		},
		{
			name: "empty string",
			flag: "",
			want: "",
		},
		{
			name: "only at",
			flag: "@",
			want: "",
		},
		{
			name: "multiple at",
			flag: "flag@@",
			want: "",
		},
		{
			name: "trailing at",
			flag: "flag@cmd@",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFlagPath(tt.flag); got != tt.want {
				t.Errorf("getFlagPath() = %v, want %v", got, tt.want)
			}

		})
	}
}

func TestParser_IsTopLevel(t *testing.T) {
	p := NewParser()
	_ = p.AddFlag("top", NewArg())
	_ = p.AddCommand(NewCommand(
		WithName("cmd"),
		WithSubcommands(NewCommand(WithName("sub"))),
	))

	cmd, _ := p.getCommand("cmd")
	assert.NotNil(t, cmd)
	sub, _ := p.getCommand("cmd sub")
	assert.NotNil(t, sub)

	assert.True(t, cmd.IsTopLevel(), "cmd should be top-level command")
	assert.False(t, sub.IsTopLevel(), "sub should not be top-level command")
}

func TestSplitPathFlag(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want []string
	}{
		{
			name: "no path",
			flag: "flag",
			want: []string{"flag"},
		},
		{
			name: "simple path",
			flag: "flag@cmd",
			want: []string{"flag", "cmd"},
		},
		{
			name: "nested path",
			flag: "flag@cmd subcmd",
			want: []string{"flag", "cmd subcmd"},
		},
		{
			name: "empty string",
			flag: "",
			want: []string{""},
		},
		{
			name: "only at",
			flag: "@",
			want: []string{"", ""},
		},
		{
			name: "multiple at",
			flag: "flag@@cmd",
			want: []string{"flag", "", "cmd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPathFlag(tt.flag)
			if !stringSliceEqual(got, tt.want) {
				t.Errorf("splitPathFlag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNestedSlicePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "empty path",
			path: "",
			want: false,
		},
		{
			name: "simple path",
			path: "users",
			want: false,
		},
		{
			name: "nested path",
			path: "field.0.inner",
			want: true,
		},
		{
			name: "deeply nested path",
			path: "field.0.inner.1.more",
			want: true,
		},
		{
			name: "trailing dot",
			path: "users.addresses.",
			want: false,
		},
		{
			name: "multiple dots",
			path: "users..addresses",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNestedSlicePath(tt.path); got != tt.want {
				t.Errorf("isNestedSlicePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParser_SecureFlagsInCommandContext(t *testing.T) {
	type Config struct {
		Nested struct {
			Secret string `goopt:"name:secret;secure:true;prompt:Enter secret;path:command"`
		}
	}

	type Options struct {
		Verbose          bool   `goopt:"name:verbose"`
		Password         string `goopt:"name:password;secure:true;prompt:Enter password"`
		ShouldNotBeAsked string `goopt:"secure:true;prompt:Should not be shown;path:not-passed-command;required:true"`
		Command          struct {
			Config Config
		} `goopt:"kind:command"`
	}

	opts := &Options{}
	parser, err := NewParserFromStruct(opts)
	assert.NoError(t, err)

	originalReader := parser.SetTerminalReader(nil)
	originalStderr := parser.SetStderr(&bytes.Buffer{})
	originalStdout := parser.SetStdout(&bytes.Buffer{})
	defer func() {
		parser.SetTerminalReader(originalReader)
		parser.SetStderr(originalStderr)
		parser.SetStdout(originalStdout)
	}()
	parser.SetTerminalReader(&MockTerminal{
		Password:         []byte("test-password"),
		IsTerminalResult: true,
		Err:              nil,
	})

	// Test parsing with command context
	args := []string{"command", "--password", "--verbose", "--config.nested.secret"}

	assert.True(t, parser.Parse(args))

	// Verify the password was set correctly
	assert.Equal(t, "test-password", opts.Password)
	assert.Equal(t, "test-password", opts.Command.Config.Nested.Secret)

	// Test that secure flags are properly registered in command context
	passwordArg, err := parser.GetArgument("password")
	assert.NoError(t, err)
	assert.True(t, passwordArg.Secure.IsSecure)
	assert.Equal(t, "Enter password", passwordArg.Secure.Prompt)

	secretArg, err := parser.GetArgument("config.nested.secret", "command")
	assert.NoError(t, err)
	assert.True(t, secretArg.Secure.IsSecure)
	assert.Equal(t, "Enter secret", secretArg.Secure.Prompt)

	assert.Empty(t, opts.ShouldNotBeAsked)
}

func TestParser_ComplexPositionalArguments(t *testing.T) {
	tests := []struct {
		name          string
		args          string
		setupParser   func(*Parser)
		wantPositions []PositionalArgument
		wantFlags     map[string]string
		wantErr       bool
		errContains   string
	}{
		{
			name: "three positionals with mixed flags",
			args: "input.txt --verbose output.txt --format json config.yaml",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("input", NewArg(
					WithPosition(0),
					WithDescription("Input file"),
					WithRequired((true)),
				))
				_ = p.AddFlag("output", NewArg(
					WithPosition(1),
					WithDescription("Output file"),
					WithRequired(true),
				))
				_ = p.AddFlag("config", NewArg(
					WithPosition(2),
					WithDescription("Config file"),
					WithRequired(true),
				))
				_ = p.AddFlag("verbose", NewArg(
					WithType(types.Standalone),
					WithDescription("Verbose output"),
				))
				_ = p.AddFlag("format", NewArg(
					WithType(types.Single),
					WithDescription("Output format"),
					WithAcceptedValues([]types.PatternValue{
						{Pattern: "json|yaml|text", Description: "Output format", Compiled: regexp.MustCompile("json|yaml|text")},
					},
					)))
			},
			wantPositions: []PositionalArgument{
				{Position: 0, Value: "input.txt"},
				{Position: 2, Value: "output.txt"},
				{Position: 5, Value: "config.yaml"},
			},
			wantFlags: map[string]string{
				"verbose": "true",
				"format":  "json",
			},
		},
		{
			name: "missing middle positional",
			args: "input.txt --verbose --format json config.yaml",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("input", NewArg(
					WithPosition(0),
					WithRequired(true),
				))
				_ = p.AddFlag("output", NewArg(
					WithPosition(1),
					WithRequired(true),
				))
				_ = p.AddFlag("config", NewArg(
					WithPosition(2),
					WithRequired(true),
				))
				_ = p.AddFlag("verbose", NewArg(WithType(types.Standalone)))
				_ = p.AddFlag("format", NewArg(WithType(types.Single)))
			},
			wantErr:     true,
			errContains: "missing required positional argument",
		},
		{
			name: "unbound positional between required ones",
			args: "input.txt extra.dat output.txt --verbose config.yaml",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("input", NewArg(
					WithPosition(0),
					WithRequired(true),
				))
				_ = p.AddFlag("output", NewArg(
					WithPosition(1),
					WithRequired(true),
				))
				_ = p.AddFlag("config", NewArg(
					WithPosition(2),
					WithRequired(true),
				))
				_ = p.AddFlag("verbose", NewArg(WithType(types.Standalone)))
			},
			wantPositions: []PositionalArgument{
				{Position: 0, Value: "input.txt"},
				{Position: 1, Value: "extra.dat"}, // Unbound positional
				{Position: 2, Value: "output.txt"},
				{Position: 4, Value: "config.yaml"},
			},
			wantFlags: map[string]string{
				"verbose": "true",
			},
		},
		{
			name: "flag value mistaken as positional",
			args: "input.txt --format json output.txt config.yaml",
			setupParser: func(p *Parser) {
				_ = p.AddFlag("input", NewArg(
					WithPosition(0),
					WithRequired(true),
				))
				_ = p.AddFlag("output", NewArg(
					WithPosition(1),
					WithRequired(true),
				))
				_ = p.AddFlag("config", NewArg(
					WithPosition(2),
					WithRequired(true),
				))
				_ = p.AddFlag("format", NewArg(WithType(types.Single)))
			},
			wantPositions: []PositionalArgument{
				{Position: 0, Value: "input.txt"},
				{Position: 3, Value: "output.txt"},
				{Position: 4, Value: "config.yaml"},
			},
			wantFlags: map[string]string{
				"format": "json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			tt.setupParser(p)

			ok := p.ParseString(tt.args)
			if tt.wantErr {
				assert.False(t, ok)
				errs := p.GetErrors()
				assert.NotEmpty(t, errs)
				assert.Contains(t, errs[0].Error(), tt.errContains)
				return
			}

			assert.True(t, ok)
			pos := p.GetPositionalArgs()
			assert.Equal(t, len(tt.wantPositions), len(pos))

			for i := 0; i < util.Min(len(tt.wantPositions), len(pos)); i++ {
				assert.Equal(t, tt.wantPositions[i].Position, pos[i].Position,
					"Position mismatch at index %d", i)
				assert.Equal(t, tt.wantPositions[i].Value, pos[i].Value,
					"Value mismatch at index %d", i)
			}

			// Verify flag values
			for flag, expectedValue := range tt.wantFlags {
				assert.Equal(t, expectedValue, p.GetOrDefault(flag, ""),
					"Flag value mismatch for %s", flag)
			}
		})
	}
}

func TestParser_ComplexPositionalArgumentsWithGapsAndDefaults(t *testing.T) {
	type Config struct {
		Source      string `goopt:"pos:0;required:true"`
		Destination string `goopt:"pos:1"`
		Optional    string `goopt:"pos:2;default:default_value"`
	}

	var cfg Config
	p, err := NewParserFromStruct(&cfg)
	assert.NoError(t, err)

	// Test with all positionals provided
	assert.True(t, p.ParseString("src dest opt"), "should parse all positional arguments")
	assert.Empty(t, p.GetErrors())
	assert.Equal(t, "src", cfg.Source, "should bind first positional")
	assert.Equal(t, "dest", cfg.Destination, "should bind second positional")
	assert.Equal(t, "opt", cfg.Optional, "should bind third positional")

	// Test with required and default
	cfg = Config{}
	p, err = NewParserFromStruct(&cfg)
	assert.NoError(t, err)
	assert.True(t, p.ParseString("src dest"), "should parse with missing optional argument")
	assert.Equal(t, "src", cfg.Source, "should bind first positional")
	assert.Equal(t, "dest", cfg.Destination, "should bind second positional")
	assert.Equal(t, "default_value", cfg.Optional, "should use default value for missing optional")

	// Test missing required
	cfg = Config{}
	p, err = NewParserFromStruct(&cfg)
	assert.NoError(t, err)
	assert.False(t, p.ParseString(""), "should not parse empty string when flags are required")
	assert.NotEmpty(t, p.GetErrors(), "should have error for missing required positional")

	// Test positions are preserved
	cfg = Config{}
	p, err = NewParserFromStruct(&cfg)
	assert.NoError(t, err)
	assert.True(t, p.ParseString("src dest"))
	posArgs := p.GetPositionalArgs()
	assert.Equal(t, 3, len(posArgs), "should have three positional arguments")
	assert.Equal(t, 0, posArgs[0].Position, "first positional should have position 0")
	assert.Equal(t, 1, posArgs[1].Position, "second positional should have position 1")
	assert.Equal(t, 2, posArgs[2].Position, "third positional should have position 2")
}

func TestParser_MoreStructTagPositionalArguments(t *testing.T) {
	type ComplexConfig struct {
		First    string `goopt:"pos:0;required:true"`
		Second   string `goopt:"pos:2;default:second_default"` // Note gap at pos:1
		Third    string `goopt:"pos:5;default:third_default"`  // Larger gap
		Fourth   string `goopt:"pos:1"`                        // Fill the gap
		Optional string `goopt:"pos:10;default:way_back"`      // Far gap
	}

	var cfg ComplexConfig
	p, err := NewParserFromStruct(&cfg)
	assert.NoError(t, err)

	// Test 1: Minimal input (only required)
	assert.True(t, p.ParseString("first"), "should parse with only required arg")
	assert.Empty(t, p.GetErrors())
	assert.Equal(t, "first", cfg.First)
	assert.Equal(t, "second_default", cfg.Second)
	assert.Equal(t, "third_default", cfg.Third)
	assert.Equal(t, "", cfg.Fourth)
	assert.Equal(t, "way_back", cfg.Optional)

	// Test 2: Fill some gaps
	cfg = ComplexConfig{}
	p, err = NewParserFromStruct(&cfg)
	assert.NoError(t, err)
	assert.True(t, p.ParseString("first middle second"), "should parse with some gaps filled")
	assert.Empty(t, p.GetErrors())
	assert.Equal(t, "first", cfg.First)
	assert.Equal(t, "middle", cfg.Fourth)
	assert.Equal(t, "second", cfg.Second)
	assert.Equal(t, "third_default", cfg.Third)
	assert.Equal(t, "way_back", cfg.Optional)

	// Test 3: Fill all positions up to a gap
	cfg = ComplexConfig{}
	p, err = NewParserFromStruct(&cfg)
	assert.NoError(t, err)
	assert.True(t, p.ParseString("first middle second third fourth fifth sixth"), "should parse multiple args")
	assert.Empty(t, p.GetErrors())
	assert.Equal(t, "first", cfg.First)
	assert.Equal(t, "middle", cfg.Fourth)
	assert.Equal(t, "second", cfg.Second)
	assert.Equal(t, "fifth", cfg.Third)
	assert.Equal(t, "way_back", cfg.Optional)

	// Test 4: Test position preservation
	posArgs := p.GetPositionalArgs()
	assert.Equal(t, 8, len(posArgs), "should have 8 positional args (7 from input + 1 default)")

	expected := []struct {
		pos   int
		value string
	}{
		{0, "first"},     // bound to First
		{1, "middle"},    // bound to Fourth
		{2, "second"},    // bound to Second
		{3, "third"},     // unbound
		{4, "fourth"},    // unbound
		{5, "fifth"},     // bound to Third
		{6, "sixth"},     // unbound
		{10, "way_back"}, // bound to Optional (default)
	}

	for i, exp := range expected {
		assert.Equal(t, exp.pos, posArgs[i].Position,
			fmt.Sprintf("position at index %d should be %d", i, exp.pos))
		assert.Equal(t, exp.value, posArgs[i].Value,
			fmt.Sprintf("value at position %d should be %s", exp.pos, exp.value))
	}

	// Test 5: Very large gap
	type LargeGapConfig struct {
		Start string `goopt:"pos:{idx:0};required:true"`
		Far   string `goopt:"pos:{idx:100};default:far_default"`
	}
	var lgCfg LargeGapConfig
	p, err = NewParserFromStruct(&lgCfg)
	assert.NoError(t, err)
	assert.True(t, p.ParseString("start"), "should handle large gaps")
	assert.Equal(t, "start", lgCfg.Start)
	assert.Equal(t, "far_default", lgCfg.Far)

	// Test 6: Multiple optional gaps
	type MultiGapConfig struct {
		First string `goopt:"pos:0;required:true"`
		Gap1  string `goopt:"pos:2;default:gap1_default"`
		Gap2  string `goopt:"pos:4;default:gap2_default"`
		Gap3  string `goopt:"pos:6;default:gap3_default"`
		Last  string `goopt:"pos:8;default:last_default"`
	}
	var mgCfg MultiGapConfig
	p, err = NewParserFromStruct(&mgCfg)
	assert.NoError(t, err)

	// Test partial fill
	assert.True(t, p.ParseString("first second third"), "should handle multiple gaps")
	assert.Equal(t, "first", mgCfg.First)
	assert.Equal(t, "third", mgCfg.Gap1)
	assert.Equal(t, "gap2_default", mgCfg.Gap2)
	assert.Equal(t, "gap3_default", mgCfg.Gap3)
	assert.Equal(t, "last_default", mgCfg.Last)

	// Test 7: Error cases
	p, err = NewParserFromStruct(&mgCfg)
	assert.NoError(t, err)
	assert.False(t, p.ParseString(""), "should fail with missing required arg")
	assert.NotEmpty(t, p.GetErrors())
}

func BenchmarkParse(b *testing.B) {
	parser := NewParser()
	parser.AddFlag("verbose", NewArg(WithShortFlag("v")))
	parser.AddFlag("output", NewArg(WithShortFlag("o")))
	parser.AddFlag("input", NewArg(WithShortFlag("i")))
	parser.AddFlag("config", NewArg(WithShortFlag("c")))
	parser.AddFlag("format", NewArg(WithShortFlag("f")))
	parser.AddFlag("loglevel", NewArg(WithShortFlag("l")))
	parser.AddFlag("logfile", NewArg(WithShortFlag("L")))
	parser.AddFlag("logformat", NewArg(WithShortFlag("F")))
	parser.AddFlag("logrotate", NewArg(WithShortFlag("R")))
	parser.AddFlag("logrotate-interval", NewArg(WithShortFlag("r")))
	args := []string{"-v", "--output", "test", "--input", "test", "--config", "test", "--format", "test", "--loglevel", "test", "--logfile", "test", "--logformat", "test", "--logrotate", "test", "--logrotate-interval", "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(args)
	}
}
func TestParser_PrintPositionalArgs(t *testing.T) {
	tests := []struct {
		name    string
		config  interface{}
		want    string
		wantErr bool
	}{
		{
			name: "basic positional args",
			config: struct {
				Source   string `goopt:"name:source;pos:0;desc:Source file"`
				Dest     string `goopt:"name:dest;pos:1;desc:Destination file"`
				Optional string `goopt:"name:optional;pos:5;desc:Optional file"`
			}{},
			want: `
Positional Arguments:
 source "Source file" (position: 0)
 dest "Destination file" (position: 1)
 optional "Optional file" (position: 5)
`,
		},
		{
			name: "no positional args",
			config: struct {
				Flag1 string `goopt:"name:flag1"`
				Flag2 string `goopt:"name:flag2"`
			}{},
			want: "",
		},
		{
			name: "mixed flags and positions",
			config: struct {
				Source  string `goopt:"name:source;pos:0;desc:Source file"`
				Verbose bool   `goopt:"name:verbose"`
				Dest    string `goopt:"name:dest;pos:1;desc:Destination file"`
			}{},
			want: `
Positional Arguments:
 source "Source file" (position: 0)
 dest "Destination file" (position: 1)
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewParserFromInterface(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewParserFromStruct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			var buf bytes.Buffer
			p.PrintPositionalArgs(&buf)
			got := buf.String()

			if got != tt.want {
				t.Errorf("PrintPositionalArgs() output mismatch:\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestParser_SequentialCommandParsingWithCascadingFlags(t *testing.T) {
	parser := NewParser()
	parser.AddCommand(&Command{
		Name:        "nexus",
		Description: "nexus commands",
		Subcommands: []Command{
			{
				Name:        "copy",
				Description: "copy commands",
				Subcommands: []Command{
					{
						Name:        "blobs",
						Description: "copy blob",
						Callback: func(cmdLine *Parser, command *Command) error {
							assert.True(t, cmdLine.GetOrDefault("fromServer", "", command.Path()) == "server1")
							assert.True(t, cmdLine.GetOrDefault("toServer", "", command.Path()) == "server2")
							return nil
						},
					},
					{
						Name:        "repos",
						Description: "copy repo",
						Callback: func(cmdLine *Parser, command *Command) error {
							assert.True(t, cmdLine.GetOrDefault("fromServer", "", command.Path()) == "server1")
							assert.True(t, cmdLine.GetOrDefault("toServer", "", command.Path()) == "server2")
							return nil
						},
					},
					{
						Name:        "roles",
						Description: "copy role",
						Callback: func(cmdLine *Parser, command *Command) error {
							assert.True(t, cmdLine.GetOrDefault("fromServer", "", command.Path()) == "server1")
							assert.True(t, cmdLine.GetOrDefault("toServer", "", command.Path()) == "server2")
							return nil
						},
					},
				},
			},
		},
	})
	// shared flags apply to all subcommands
	parser.AddFlag("fromServer", NewArg(WithShortFlag("f")), "nexus copy")
	parser.AddFlag("toServer", NewArg(WithShortFlag("t")), "nexus copy")

	assert.True(t, parser.ParseString("nexus copy blobs nexus copy repos nexus copy roles --fromServer server1 --toServer server2"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_OverrideParentFlagInSubcommand verifies that a flag defined in a subcommand
// takes precedence over the same flag defined in a parent command
func TestParser_OverrideParentFlagInSubcommand(t *testing.T) {
	parser := NewParser()
	parser.AddCommand(&Command{
		Name:        "parent",
		Description: "parent command",
		Subcommands: []Command{
			{
				Name:        "child",
				Description: "child command",
				Callback: func(cmdLine *Parser, command *Command) error {
					// The child's specific flag value should override the parent's
					value, found := cmdLine.Get("shared", command.Path())
					assert.True(t, found)
					assert.Equal(t, "child-value", value)
					return nil
				},
			},
		},
	})

	// Add flag to parent command
	parser.AddFlag("shared", NewArg(), "parent")

	// Override in child command
	parser.AddFlag("shared", NewArg(), "parent child")

	// Parse with both flags specified
	assert.True(t, parser.ParseString("parent child --shared parent-value --shared child-value"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_FlagsInheritance verifies that short flags defined in parent commands
// are accessible from subcommands
func TestParser_FlagsInheritance(t *testing.T) {
	parser := NewParser()
	parser.AddCommand(&Command{
		Name:        "app",
		Description: "application command",
		Subcommands: []Command{
			{
				Name:        "config",
				Description: "configuration command",
				Subcommands: []Command{
					{
						Name:        "set",
						Description: "set configuration",
						Callback: func(cmdLine *Parser, command *Command) error {
							// Should be able to access the short flag from parent
							debug, err := cmdLine.GetBool("debug", command.Path())
							assert.Nil(t, err)
							assert.True(t, debug)

							verbose, err := cmdLine.GetBool("verbose", command.Path())
							assert.Nil(t, err)
							assert.True(t, verbose)
							return nil
						},
					},
				},
			},
		},
	})

	// Add flags to parent commands with short forms
	parser.AddFlag("debug", NewArg(WithShortFlag("d"), WithType(types.Standalone)), "app")
	parser.AddFlag("verbose", NewArg(WithShortFlag("v"), WithType(types.Standalone)), "app config")

	// Test with short flags
	assert.True(t, parser.ParseString("app config set -d -v"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())

	parser.ClearErrors()
	// Test with long flags too
	assert.True(t, parser.ParseString("app config set --debug --verbose"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_MixedGlobalAndCommandFlags tests the interaction between global flags
// and command-specific flags, ensuring proper precedence and access
func TestParser_MixedGlobalAndCommandFlags(t *testing.T) {
	var parser *Parser
	setup := func() {
		parser = NewParser()

		// Add a global flag
		parser.AddFlag("format", NewArg(WithDefaultValue("json")))

		parser.AddCommand(&Command{
			Name:        "resource",
			Description: "resource command",
			Subcommands: []Command{
				{
					Name:        "list",
					Description: "list resources",
					Callback: func(cmdLine *Parser, command *Command) error {
						// Access global flag
						format, found := cmdLine.Get("format", command.Path())
						assert.True(t, found)
						assert.Equal(t, "yaml", format)

						// Access parent command flag
						filter, found := cmdLine.Get("filter", command.Path())
						assert.True(t, found)
						assert.Equal(t, "type=database", filter)

						// Access command-specific flag
						limit, err := cmdLine.GetInt("limit", 32, command.Path())
						assert.Nil(t, err)
						assert.Equal(t, int64(10), limit)

						return nil
					},
				},
				{
					Name:        "create",
					Description: "create resource",
					Callback: func(cmdLine *Parser, command *Command) error {
						// Access global flag with default (not overridden for this command)
						format, found := cmdLine.Get("format")
						assert.True(t, found)
						assert.Equal(t, "json", format)

						// Access parent command flag
						filter, found := cmdLine.Get("filter", command.Path())
						assert.True(t, found)
						assert.Equal(t, "type=database", filter)

						return nil
					},
				},
			},
		})

		parser.AddFlag("filter", NewArg(), "resource")

		// Add a flag to a specific subcommand
		parser.AddFlag("limit", NewArg(), "resource list")

	}

	setup()

	// Test mixed flag access in the list command
	assert.True(t, parser.ParseString("resource list --format yaml --filter type=database --limit 10"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())

	// Clear for the next test
	setup()

	// Test with the create command which doesn't override the global format
	assert.True(t, parser.ParseString("resource create --filter type=database"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_SharedFlags_DeepCommandNesting tests flags defined at various levels
// of a deeply nested command structure to verify they cascade properly
func TestParser_SharedFlags_DeepCommandNesting(t *testing.T) {
	parser := NewParser()

	// Create a deeply nested command structure (6 levels)
	level5Cmd := Command{
		Name:        "level5",
		Description: "deepest level command",
		Callback: func(cmdLine *Parser, command *Command) error {
			// Verify all flags are accessible at the deepest level
			rootFlag, found := cmdLine.Get("rootFlag", command.Path())
			assert.True(t, found)
			assert.Equal(t, "root-value", rootFlag)

			level1Flag, found := cmdLine.Get("level1Flag", command.Path())
			assert.True(t, found)
			assert.Equal(t, "level1-value", level1Flag)

			level2Flag, found := cmdLine.Get("level2Flag", command.Path())
			assert.True(t, found)
			assert.Equal(t, "level2-value", level2Flag)

			level3Flag, found := cmdLine.Get("level3Flag", command.Path())
			assert.True(t, found)
			assert.Equal(t, "level3-value", level3Flag)

			level4Flag, found := cmdLine.Get("level4Flag", command.Path())
			assert.True(t, found)
			assert.Equal(t, "level4-value", level4Flag)

			level5Flag, found := cmdLine.Get("level5Flag", command.Path())
			assert.True(t, found)
			assert.Equal(t, "level5-value", level5Flag)

			return nil
		},
	}

	level4Cmd := Command{
		Name:        "level4",
		Description: "level 4 command",
		Subcommands: []Command{level5Cmd},
	}

	level3Cmd := Command{
		Name:        "level3",
		Description: "level 3 command",
		Subcommands: []Command{level4Cmd},
	}

	level2Cmd := Command{
		Name:        "level2",
		Description: "level 2 command",
		Subcommands: []Command{level3Cmd},
	}

	level1Cmd := Command{
		Name:        "level1",
		Description: "level 1 command",
		Subcommands: []Command{level2Cmd},
	}

	rootCmd := Command{
		Name:        "root",
		Description: "root command",
		Subcommands: []Command{level1Cmd},
	}

	err := parser.AddCommand(&rootCmd)
	assert.Nil(t, err)

	// Add flags at each level
	parser.AddFlag("rootFlag", NewArg(), "root")
	parser.AddFlag("level1Flag", NewArg(), "root level1")
	parser.AddFlag("level2Flag", NewArg(), "root level1 level2")
	parser.AddFlag("level3Flag", NewArg(), "root level1 level2 level3")
	parser.AddFlag("level4Flag", NewArg(), "root level1 level2 level3 level4")
	parser.AddFlag("level5Flag", NewArg(), "root level1 level2 level3 level4 level5")

	// Test execution with all flags
	cmdStr := "root level1 level2 level3 level4 level5 --rootFlag root-value --level1Flag level1-value " +
		"--level2Flag level2-value --level3Flag level3-value --level4Flag level4-value --level5Flag level5-value"

	assert.True(t, parser.ParseString(cmdStr))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_OverridePriority tests that when the same flag is defined
// at multiple levels, the most specific one takes precedence
func TestParser_OverridePriority(t *testing.T) {
	parser := NewParser()

	// Create a command hierarchy with 4 levels
	level3Cmd := Command{
		Name:        "level3",
		Description: "level 3 command",
		Callback: func(cmdLine *Parser, command *Command) error {
			// Should get high verbosity (most specific)
			verbosity, found := cmdLine.Get("verbose", command.Path())
			assert.True(t, found)
			assert.Equal(t, "high", verbosity)
			return nil
		},
	}

	level2Cmd := Command{
		Name:        "level2",
		Description: "level 2 command",
		Subcommands: []Command{level3Cmd},
		Callback: func(cmdLine *Parser, command *Command) error {
			// Should get medium verbosity (from this level)
			verbosity, found := cmdLine.Get("verbose", command.Path())
			assert.True(t, found)
			assert.Equal(t, "medium", verbosity)
			return nil
		},
	}

	level1Cmd := Command{
		Name:        "level1",
		Description: "level 1 command",
		Subcommands: []Command{level2Cmd},
		Callback: func(cmdLine *Parser, command *Command) error {
			// Should get low verbosity (from root)
			verbosity, found := cmdLine.Get("verbose", command.Path())
			assert.True(t, found)
			assert.Equal(t, "low", verbosity)
			return nil
		},
	}

	rootCmd := Command{
		Name:        "root",
		Description: "root command",
		Subcommands: []Command{level1Cmd},
	}

	err := parser.AddCommand(&rootCmd)
	assert.Nil(t, err)

	// Define the same flag at different levels with different default values
	parser.AddFlag("verbose", NewArg(WithDefaultValue("low")), "root")
	parser.AddFlag("verbose", NewArg(WithDefaultValue("medium")), "root level1 level2")
	parser.AddFlag("verbose", NewArg(WithDefaultValue("high")), "root level1 level2 level3")

	// Test execution with explicit values
	cmdStr := "root level1 level2 level3 --verbose high"
	assert.True(t, parser.ParseString(cmdStr))
	assert.Empty(t, parser.GetErrors())

	// Execute each command's callback to verify correct flag resolution
	assert.Equal(t, 0, parser.ExecuteCommands())

	// Test with a command that doesn't override the default values
	parser.ClearErrors()
	cmdStr = "root level1 level2"
	assert.True(t, parser.ParseString(cmdStr))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())

	// Test with the root command
	parser.ClearErrors()
	cmdStr = "root level1"
	assert.True(t, parser.ParseString(cmdStr))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_ExistingPathComponents tests flags that already have path components
// and verifies they're handled correctly in different command contexts
func TestParser_ExistingPathComponents(t *testing.T) {
	parser := NewParser()

	// Create a command structure
	configCmd := Command{
		Name:        "config",
		Description: "configuration command",
		// Remove the callback from the parent command since it has subcommands
		Subcommands: []Command{
			{
				Name:        "show",
				Description: "show configuration",
				Callback: func(cmdLine *Parser, command *Command) error {
					// Should find settings in subcommand
					settings, found := cmdLine.Get("settings", command.Path())
					assert.True(t, found)
					assert.Equal(t, "config-specific", settings)
					return nil
				},
			},
			{
				Name:        "set",
				Description: "set configuration",
				Callback: func(cmdLine *Parser, command *Command) error {
					// Should also find settings in this subcommand
					settings, found := cmdLine.Get("settings", command.Path())
					assert.True(t, found)
					assert.Equal(t, "config-specific", settings)
					return nil
				},
			},
		},
	}

	settingsCmd := Command{
		Name:        "settings",
		Description: "settings command",
		Callback: func(cmdLine *Parser, command *Command) error {
			// This should NOT find the settings@config flag since we're in a different command
			// Instead it should find the global settings flag if defined
			settings, found := cmdLine.Get("settings", command.Path())
			assert.True(t, found)
			assert.Equal(t, "global-value", settings)
			return nil
		},
	}

	rootCmd := Command{
		Name:        "app",
		Description: "application command",
		Subcommands: []Command{configCmd, settingsCmd},
	}

	err := parser.AddCommand(&rootCmd)
	assert.Nil(t, err)

	// Add a flag with an explicit path component - use the full path
	parser.AddFlag("settings@app config", NewArg())

	// Also try adding with the partial path
	parser.AddFlag("settings", NewArg(), "app", "config")

	// Add a global flag with the same name
	parser.AddFlag("settings", NewArg())

	// Test with valid subcommand
	cmdStr := "app config show --settings config-specific"
	assert.True(t, parser.ParseString(cmdStr))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())

	// Test with different subcommand
	parser.ClearErrors()
	cmdStr = "app config set --settings config-specific"
	assert.True(t, parser.ParseString(cmdStr))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())

	// Test with settings command
	parser.ClearErrors()
	cmdStr = "app settings --settings global-value"
	assert.True(t, parser.ParseString(cmdStr))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

func TestParser_ScalableShortFlagResolution(t *testing.T) {
	parser := NewParser()

	// Set up a command hierarchy
	parser.AddCommand(&Command{
		Name: "parent",
		Subcommands: []Command{
			{
				Name: "child",
			},
		},
	})

	// Add flags
	parser.AddFlag("verbose", NewArg(WithShortFlag("v"), WithType(types.Standalone)), "parent")

	// Test that child can access parent's short flag
	assert.True(t, parser.ParseString("parent child -v"))
	assert.Empty(t, parser.GetErrors())

	// Verify the flag was set in the child context
	value, found := parser.Get("verbose", "parent child")
	assert.True(t, found)
	assert.Equal(t, "true", value)
}

// Helper function for comparing string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
