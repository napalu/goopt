package goopt

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/napalu/goopt/v2/validation"

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
	assert.Contains(t, *writer.data, " │───── wacky9\n")
	assert.Contains(t, *writer.data, " └────── wacky10\n")

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

	expectedOutput := `Usage: ` + os.Args[0] + `

Global Flags:

 --help "Display help" (optional)

Commands:
 +  create "Create resources"
 ├─  ** create user "Manage users"
 │   │  --email or -e "Email for user creation" (optional)
 └─  **  ** create user type "Specify user type"
 │   │   │  --username "Username for user creation" (required)
 │   │   │  --firstName "User first name" (optional)
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

			err := parser.processStructCommands(val, "", 0, 10, nil)
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
	// Test command execution error handling.
	// Note: Error messages use the global default bundle translations.
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
						// Use a standard error since we can no longer customize messages
						return errs.ErrCommandCallbackError.WithArgs(command.Name)
					},
				}
				return p.AddCommand(cmd)
			},
			args:        []string{"test"},
			execMethod:  "single",
			wantErrs:    map[string]string{"test": "error in command callback: test"},
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
				"cmd1": "error in command callback: cmd1",
				"cmd2": "error in command callback: cmd2",
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
				"test": "error processing command test: error in command callback: test: custom error",
			},
			description: "Should execute commands during Parse when ExecOnParse is true",
		},
		{
			name: "execute on parse",
			setupFunc: func(p *Parser) error {
				cmd := &Command{
					Name:        "test",
					ExecOnParse: true,
					Callback: func(cmdLine *Parser, command *Command) error {
						return errs.ErrCommandCallbackError.WithArgs(command.Name).Wrap(errors.New("custom error"))
					},
				}
				return p.AddCommand(cmd)
			},
			args:       []string{"test"},
			execMethod: "onParse",
			wantErrs: map[string]string{
				"test": "error processing command test: error in command callback: test: custom error",
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
					expectedMsg := tt.wantErrs[kv.Key]
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
					expectedMsg := tt.wantErrs[kv.Key]
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
				"--log-level or -l \"Set logging level\" (required)",
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
 source "Source file" (positional: 0)
 dest "Destination file" (positional: 1)
 optional "Optional file" (positional: 5)
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
 source "Source file" (positional: 0)
 dest "Destination file" (positional: 1)
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

func TestParser_StructCallbackOnTerminalCommand(t *testing.T) {
	t.Run("Callback on terminal command should be executed", func(t *testing.T) {
		type Commands struct {
			Create struct {
				Name    string
				Execute CommandFunc
			} `goopt:"kind:command;name:create;desc:Create something"`
		}

		executed := false
		cmds := &Commands{}
		cmds.Create.Execute = func(cmdLine *Parser, command *Command) error {
			executed = true
			return nil
		}

		parser, err := NewParserFromStruct(cmds)
		assert.NoError(t, err)
		parser.SetExecOnParse(true)

		// Parse with the create command
		result := parser.ParseString("create")
		assert.True(t, result)
		assert.True(t, executed, "Callback should have been executed")
	})

	t.Run("Callback on non-terminal command should return error", func(t *testing.T) {
		type Commands struct {
			Create struct {
				Execute CommandFunc
				User    struct {
					Username string `goopt:"short:u;desc:User name"`
				} `goopt:"kind:command;name:user;desc:Create user"`
			} `goopt:"kind:command;name:create;desc:Create resources"`
		}

		executed := false
		cmds := &Commands{}
		cmds.Create.Execute = func(cmdLine *Parser, command *Command) error {
			executed = true
			return nil
		}

		parser, err := NewParserFromStruct(cmds)
		assert.NoError(t, err)
		parser.SetExecOnParse(true)
		assert.False(t, parser.ParseString("create user --username foo"))
		assert.Len(t, parser.GetErrors(), 1, "Should set error for callback on non-terminal command")
		// Check that the error is a processing command error by checking its key
		// The error might be wrapped, so we need to unwrap it
		checkErr := parser.GetErrors()[0]
		for checkErr != nil {
			if trErr, ok := checkErr.(*i18n.TrError); ok {
				if trErr.Key() == errs.ErrProcessingCommandKey {
					break // Found it
				}
			}
			checkErr = errors.Unwrap(checkErr)
		}
		assert.NotNil(t, checkErr, "Should find ErrProcessingCommand in error chain")
		assert.False(t, executed, "Callback should not have been executed")
	})

	t.Run("Multiple callbacks in different terminal commands", func(t *testing.T) {
		type Commands struct {
			Create struct {
				Execute CommandFunc
			} `goopt:"kind:command;name:create;desc:Create resources"`

			Delete struct {
				Execute CommandFunc
			} `goopt:"kind:command;name:delete;desc:Delete resources"`
		}

		createExecuted := false
		deleteExecuted := false

		cmds := &Commands{}
		cmds.Create.Execute = func(cmdLine *Parser, command *Command) error {
			createExecuted = true
			return nil
		}
		cmds.Delete.Execute = func(cmdLine *Parser, command *Command) error {
			deleteExecuted = true
			return nil
		}

		parser, err := NewParserFromStruct(cmds)
		assert.NoError(t, err)
		parser.SetExecOnParse(true)
		// Parse with the create command
		result := parser.ParseString("create")
		assert.True(t, result)
		assert.True(t, createExecuted, "Create callback should have been executed")
		assert.False(t, deleteExecuted, "Delete callback should not have been executed")

		// Reset flags
		createExecuted = false
		deleteExecuted = false
		// Parse with the delete command
		result = parser.Parse([]string{"program", "delete"})
		assert.True(t, result)
		assert.False(t, createExecuted, "Create callback should not have been executed")
		assert.True(t, deleteExecuted, "Delete callback should have been executed")
	})

	t.Run("Deep nested callbacks work correctly", func(t *testing.T) {
		type Commands struct {
			Resources struct {
				Create struct {
					User struct {
						Admin struct {
							Execute CommandFunc
						} `goopt:"kind:command;name:admin;desc:Create admin user"`
					} `goopt:"kind:command;name:user;desc:Create user"`
				} `goopt:"kind:command;name:create;desc:Create resources"`
			} `goopt:"kind:command;name:resources;desc:Manage resources"`
		}

		executed := false
		cmdPath := ""

		cmds := &Commands{}
		cmds.Resources.Create.User.Admin.Execute = func(cmdLine *Parser, command *Command) error {
			executed = true
			cmdPath = command.Path()
			return nil
		}

		parser, err := NewParserFromStruct(cmds)
		assert.NoError(t, err)
		parser.SetExecOnParse(true)
		// Parse with the full command path
		result := parser.ParseString("resources create user admin")
		assert.True(t, result)
		assert.True(t, executed, "Deeply nested callback should have been executed")
		assert.Equal(t, "resources create user admin", cmdPath, "Command path should be correct")
	})

	t.Run("Callback receives correct command and parser", func(t *testing.T) {
		type Commands struct {
			Create struct {
				Execute CommandFunc
			} `goopt:"kind:command;name:create;desc:Create resources"`
		}

		var receivedParser *Parser
		var receivedCommand *Command

		cmds := &Commands{}
		cmds.Create.Execute = func(cmdLine *Parser, command *Command) error {
			receivedParser = cmdLine
			receivedCommand = command
			return nil
		}

		parser, err := NewParserFromStruct(cmds)
		assert.NoError(t, err)
		parser.SetExecOnParse(true)
		// Parse with the create command
		result := parser.ParseString("create")
		assert.True(t, result)

		// Verify that the callback received the correct parser instance
		assert.Equal(t, parser, receivedParser, "Should receive the same parser instance")

		// Verify that the correct command was passed
		assert.NotNil(t, receivedCommand, "Command should not be nil")
		assert.Equal(t, "create", receivedCommand.Name, "Should receive the create command")
	})
}

func TestParser_TestStructContext(t *testing.T) {
	// Define a test struct
	type TestConfig struct {
		Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
		Output  string `goopt:"short:o;desc:Output file"`
	}

	t.Run("StructContext with NewParserFromStruct", func(t *testing.T) {
		// Create a struct and parser
		cfg := &TestConfig{
			Verbose: false,
			Output:  "default.txt",
		}
		parser, err := NewParserFromStruct(cfg)
		if err != nil {
			t.Fatalf("Failed to create parser: %v", err)
		}

		// Check HasStructCtx
		if !parser.HasStructCtx() {
			t.Error("HasStructCtx() returned false, expected true")
		}

		// Check GetStructCtx
		structCtx := parser.GetStructCtx()
		if structCtx == nil {
			t.Fatal("GetStructCtx() returned nil, expected *TestConfig")
		}

		// Check type assertion
		gotCfg, ok := structCtx.(*TestConfig)
		if !ok {
			t.Errorf("GetStructCtx() returned wrong type, expected *TestConfig")
		}

		// Verify it's the same struct we passed in
		if gotCfg != cfg {
			t.Errorf("GetStructCtx() returned different struct instance, expected same instance")
		}

		// Check the values
		if gotCfg.Verbose != cfg.Verbose || gotCfg.Output != cfg.Output {
			t.Errorf("GetStructCtx() returned struct with incorrect values")
		}

		// Test generic GetStructCtxAs
		typedCfg, ok := GetStructCtxAs[*TestConfig](parser)
		if !ok {
			t.Errorf("GetStructCtxAs[*TestConfig]() returned ok=false, expected true")
		}

		// Verify it's the same struct we passed in
		if typedCfg != cfg {
			t.Errorf("GetStructCtxAs[*TestConfig]() returned different struct instance, expected same instance")
		}

		// Try with wrong type
		_, ok = GetStructCtxAs[*int](parser)
		if ok {
			t.Errorf("GetStructCtxAs[*int]() returned ok=true, expected false for wrong type")
		}
	})

	t.Run("StructContext with NewParser", func(t *testing.T) {
		// Create parser without struct
		parser := NewParser()

		// Check HasStructCtx
		if parser.HasStructCtx() {
			t.Error("HasStructCtx() returned true, expected false")
		}

		// Check GetStructCtx
		structCtx := parser.GetStructCtx()
		if structCtx != nil {
			t.Errorf("GetStructCtx() returned %v, expected nil", structCtx)
		}

		// Check generic GetStructCtxAs
		_, ok := GetStructCtxAs[*TestConfig](parser)
		if ok {
			t.Errorf("GetStructCtxAs[*TestConfig]() returned ok=true, expected false for nil struct context")
		}
	})

	t.Run("GetStructCtxAs with nil parser", func(t *testing.T) {
		// Test with nil parser
		var parser *Parser = nil
		_, ok := GetStructCtxAs[*TestConfig](parser)
		if ok {
			t.Errorf("GetStructCtxAs[*TestConfig](nil) returned ok=true, expected false")
		}
	})

	t.Run("Command callback with struct context", func(t *testing.T) {
		// Create a struct with a command that has a callback
		type CommandConfig struct {
			Verbose bool `goopt:"short:v;desc:Enable verbose output"`
			Create  struct {
				Output string `goopt:"short:o;desc:Output file"`
				Exec   CommandFunc
			} `goopt:"kind:command;desc:Create a resource"`
		}

		var callbackCalled bool
		var callbackConfig *CommandConfig

		// Create the config and set the callback
		cfg := &CommandConfig{
			Verbose: true,
		}
		cfg.Create.Output = "test.txt"
		cfg.Create.Exec = func(p *Parser, cmd *Command) error {
			callbackCalled = true
			// Try to get the struct context
			config, ok := GetStructCtxAs[*CommandConfig](p)
			if !ok {
				t.Errorf("Failed to get struct context in callback")
				return nil
			}
			callbackConfig = config
			return nil
		}

		// Create parser and execute callback
		parser, err := NewParserFromStruct(cfg, WithExecOnParse(true))
		if err != nil {
			t.Fatalf("Failed to create parser: %v", err)
		}

		assert.True(t, parser.ParseString("create"))

		// Verify callback was called and had access to config
		if !callbackCalled {
			t.Errorf("Command callback was not executed")
		}
		if callbackConfig != cfg {
			t.Errorf("Callback received wrong struct context")
		}
	})
}

func TestParserI18n(t *testing.T) {
	p := NewParser()

	// Test language methods
	p.SetSystemLanguage(language.Spanish)

	bundle := i18n.NewEmptyBundle()
	p.SetUserBundle(bundle)

	if p.GetUserBundle() != bundle {
		t.Error("GetUserBundle() failed")
	}

	// Test GetSystemBundle()
	if p.GetSystemBundle() == nil {
		t.Error("GetSystemBundle() should not be nil")
	}
}

func TestUtilityMethods(t *testing.T) {
	p := NewParser()
	p.AddFlag("test", NewArg())

	// Test DescribeFlag/GetDescription
	p.DescribeFlag("test", "Test flag")
	if p.GetDescription("test") != "Test flag" {
		t.Error("Description methods failed")
	}

	// Test Remove
	if !p.Remove("test") {
		t.Error("Remove() should return true")
	}

	// Test HasPositionalArgs
	if p.HasPositionalArgs() {
		t.Error("HasPositionalArgs() should be false")
	}
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

func TestParser_GetStdout(t *testing.T) {
	tests := []struct {
		name   string
		stdout io.Writer
	}{
		{
			name:   "default stdout",
			stdout: nil,
		},
		{
			name:   "custom stdout",
			stdout: &bytes.Buffer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()

			if tt.stdout != nil {
				p.SetStdout(tt.stdout)
			}

			got := p.GetStdout()

			if tt.stdout != nil && got != tt.stdout {
				t.Errorf("GetStdout() = %v, want %v", got, tt.stdout)
			}
		})
	}
}

func TestParser_GetShortFlag(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*Parser)
		flag        string
		commandPath []string
		want        string
		wantErr     bool
	}{
		{
			name: "existing short flag",
			setup: func(p *Parser) {
				p.AddFlag("verbose", NewArg(WithShortFlag("v")))
			},
			flag: "verbose",
			want: "v",
		},
		{
			name: "no short flag defined",
			setup: func(p *Parser) {
				p.AddFlag("verbose", NewArg())
			},
			flag:    "verbose",
			wantErr: true,
		},
		{
			name:    "non-existent flag",
			setup:   func(p *Parser) {},
			flag:    "nonexistent",
			wantErr: true,
		},
		{
			name: "flag in command path",
			setup: func(p *Parser) {
				cmd := NewCommand(WithName("test"))
				p.AddCommand(cmd)
				p.AddFlag("verbose", NewArg(WithShortFlag("v"), WithType(types.Standalone)), "test")
			},
			flag:        "verbose",
			commandPath: []string{"test"},
			want:        "v",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			tt.setup(p)

			got, err := p.GetShortFlag(tt.flag, tt.commandPath...)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetShortFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("GetShortFlag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParser_FlagPath(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want string
	}{
		{
			name: "simple flag",
			flag: "verbose",
			want: "",
		},
		{
			name: "flag with command path",
			flag: "verbose@cmd.subcmd",
			want: "cmd.subcmd",
		},
		{
			name: "flag with single command",
			flag: "verbose@cmd",
			want: "cmd",
		},
		{
			name: "empty flag",
			flag: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			if got := p.FlagPath(tt.flag); got != tt.want {
				t.Errorf("FlagPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArgument_DisplayID(t *testing.T) {
	tests := []struct {
		name string
		arg  *Argument
		want string
	}{
		{
			name: "positional argument",
			arg: &Argument{
				Position: util.NewOfType(0),
			},
			want: "pos0",
		},
		{
			name: "positional argument at position 5",
			arg: &Argument{
				Position: util.NewOfType(5),
			},
			want: "pos5",
		},
		{
			name: "non-positional with uniqueID and description key",
			arg: &Argument{
				uniqueID:       "12345678-abcd-efgh-ijkl-mnopqrstuvwx",
				DescriptionKey: "mykey",
			},
			want: "12345678-mykey",
		},
		{
			name: "non-positional with uniqueID only",
			arg: &Argument{
				uniqueID:       "abcdefgh-1234-5678-90ab-cdefghijklmn",
				DescriptionKey: "",
			},
			want: "abcdefgh-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.arg.DisplayID(); got != tt.want {
				t.Errorf("DisplayID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParser_SetExecOnParseComplete(t *testing.T) {
	p := NewParser()

	// Set the value and ensure no panic
	p.SetExecOnParseComplete(true)

	if !p.callbackOnParseComplete {
		t.Error("SetExecOnParseComplete(true) should set callbackOnParseComplete to true")
	}
}

func TestParser_GetPreValidationFilter(t *testing.T) {
	p := NewParser()

	// Add a flag with a pre-validation filter
	p.AddFlag("test", NewArg(WithPreValidationFilter(strings.ToUpper)))

	// Get the filter
	filter, err := p.GetPreValidationFilter("test")
	if err != nil {
		t.Errorf("GetPreValidationFilter returned error: %v", err)
	}

	if filter == nil {
		t.Error("GetPreValidationFilter should return the filter")
	}

	// Test the filter works
	if filter("hello") != "HELLO" {
		t.Error("Filter should convert to uppercase")
	}
}

func TestParser_GetPostValidationFilter(t *testing.T) {
	p := NewParser()

	// Add a flag with a post-validation filter
	p.AddFlag("test", NewArg(WithPostValidationFilter(strings.ToLower)))

	// Get the filter
	filter, err := p.GetPostValidationFilter("test")
	if err != nil {
		t.Errorf("GetPostValidationFilter returned error: %v", err)
	}

	if filter == nil {
		t.Error("GetPostValidationFilter should return the filter")
	}

	// Test the filter works
	if filter("HELLO") != "hello" {
		t.Error("Filter should convert to lowercase")
	}
}

func TestParser_GetAcceptPatterns(t *testing.T) {
	p := NewParser()

	// Add a flag with accept patterns
	p.AddFlag("test", NewArg(WithAcceptedValues([]types.PatternValue{
		{Pattern: "^[0-9]+$", Description: "numbers only"},
		{Pattern: "^[a-z]+$", Description: "lowercase letters only"},
	})))

	// Get the patterns
	patterns, err := p.GetAcceptPatterns("test")
	if err != nil {
		t.Errorf("GetAcceptPatterns returned error: %v", err)
	}

	if len(patterns) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(patterns))
	}
}

func TestCommand_AddSubcommand(t *testing.T) {
	cmd := NewCommand(WithName("parent"))

	// Add a subcommand
	subcmd := NewCommand(WithName("child"))
	cmd.AddSubcommand(subcmd)

	if len(cmd.Subcommands) != 1 {
		t.Error("AddSubcommand should add the subcommand")
	}

	if cmd.Subcommands[0].Name != "child" {
		t.Error("Subcommand should have the correct name")
	}
}

func TestCommand_WithCommandDescriptionKey(t *testing.T) {
	cmd := NewCommand(
		WithName("test"),
		WithCommandDescriptionKey("test.command.desc"),
	)

	if cmd.DescriptionKey != "test.command.desc" {
		t.Errorf("Expected DescriptionKey 'test.command.desc', got '%s'", cmd.DescriptionKey)
	}
}

func TestParser_DependsOnFlag(t *testing.T) {
	p := NewParser()

	// Add flags
	p.AddFlag("verbose", NewArg())
	p.AddFlag("debug", NewArg())

	// Test adding dependency
	err := p.DependsOnFlag("debug", "verbose")
	if err != nil {
		t.Errorf("DependsOnFlag should not return error for valid flags: %v", err)
	}

	// Test adding dependency for non-existent flag
	err = p.DependsOnFlag("nonexistent", "verbose")
	if err == nil {
		t.Error("DependsOnFlag should return error for non-existent flag")
	}
}

// TestParser_ShortFlagScoping verifies that short flags can be reused across different command contexts
func TestParser_ShortFlagScoping(t *testing.T) {
	parser := NewParser()

	// Add commands
	parser.AddCommand(&Command{
		Name:        "generate",
		Description: "Generate command",
		Callback: func(p *Parser, c *Command) error {
			// Verify we get the correct package value for generate
			pkg, found := p.Get("gen-package", "generate")
			assert.True(t, found)
			assert.Equal(t, "gen-pkg-value", pkg)
			return nil
		},
	})

	parser.AddCommand(&Command{
		Name:        "extract",
		Description: "Extract command",
		Callback: func(p *Parser, c *Command) error {
			// Verify we get the correct package value for extract
			pkg, found := p.Get("ext-package", "extract")
			assert.True(t, found)
			assert.Equal(t, "ext-pkg-value", pkg)
			return nil
		},
	})

	// Add flags with same short flag -p for different commands
	err := parser.AddFlag("gen-package", NewArg(WithShortFlag("p"), WithDescription("Package for generate")), "generate")
	assert.Nil(t, err, "Should be able to add -p for generate command")

	err = parser.AddFlag("ext-package", NewArg(WithShortFlag("p"), WithDescription("Package for extract")), "extract")
	assert.Nil(t, err, "Should be able to add -p for extract command")

	// Test parsing with context-specific short flags
	assert.True(t, parser.ParseString("generate -p gen-pkg-value"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())

	parser.ClearErrors()

	assert.True(t, parser.ParseString("extract -p ext-pkg-value"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_ShortFlagGlobalVsCommand tests interaction between global and command short flags
func TestParser_ShortFlagGlobalVsCommand(t *testing.T) {
	parser := NewParser()

	// Add global flag with -v
	err := parser.AddFlag("verbose", NewArg(WithShortFlag("v"), WithType(types.Standalone)))
	assert.Nil(t, err)

	// Add command
	parser.AddCommand(&Command{
		Name: "test",
		Callback: func(p *Parser, c *Command) error {
			// Check global flag is accessible
			verbose, _ := p.GetBool("verbose")
			assert.True(t, verbose, "Global verbose should be true")
			return nil
		},
	})

	// Try to add command flag with same short flag - should fail with global conflict
	err = parser.AddFlag("version", NewArg(WithShortFlag("v")), "test")
	assert.NotNil(t, err, "Should not be able to add -v for command when global -v exists")
	if err != nil {
		assert.Error(t, err, errs.ErrShortFlagConflictContext)
	}
}

// TestParser_ShortFlagHierarchy tests short flag resolution in nested commands
func TestParser_ShortFlagHierarchy(t *testing.T) {
	parser := NewParser()

	// Create nested command structure
	parser.AddCommand(&Command{
		Name: "app",
		Subcommands: []Command{
			{
				Name: "db",
				Subcommands: []Command{
					{
						Name: "migrate",
						Callback: func(p *Parser, c *Command) error {
							// Should resolve flags in order: migrate -> db -> app -> global
							force, _ := p.GetBool("force", c.Path())
							dryRun, _ := p.GetBool("dry-run", c.Path())
							verbose, _ := p.GetBool("verbose", c.Path())

							assert.True(t, force, "Should get migrate-level force")
							assert.True(t, dryRun, "Should get db-level dry-run")
							assert.True(t, verbose, "Should get app-level verbose")
							return nil
						},
					},
				},
			},
		},
	})

	// Add flags at different levels
	parser.AddFlag("verbose", NewArg(WithShortFlag("v"), WithType(types.Standalone)), "app")
	parser.AddFlag("dry-run", NewArg(WithShortFlag("n"), WithType(types.Standalone)), "app db")
	parser.AddFlag("force", NewArg(WithShortFlag("f"), WithType(types.Standalone)), "app db migrate")

	// Parse and verify
	assert.True(t, parser.ParseString("app db migrate -f -n -v"))
	assert.Empty(t, parser.GetErrors())
	assert.Equal(t, 0, parser.ExecuteCommands())
}

// TestParser_ShortFlagMultipleCommands tests multiple commands with overlapping short flags
func TestParser_ShortFlagMultipleCommands(t *testing.T) {
	parser := NewParser()

	commands := []string{"list", "show", "get", "describe"}
	for _, cmd := range commands {
		cmdName := cmd // Capture for closure
		parser.AddCommand(&Command{
			Name: cmdName,
			Callback: func(p *Parser, c *Command) error {
				format, _ := p.Get(cmdName+"-format", cmdName)
				assert.Equal(t, cmdName+"-json", format)
				return nil
			},
		})

		// Each command has its own -f flag
		err := parser.AddFlag(cmd+"-format", NewArg(WithShortFlag("f")), cmd)
		assert.Nil(t, err, "Should be able to add -f for %s command", cmd)
	}

	// Test each command
	for _, cmd := range commands {
		parser.ClearErrors()
		cmdStr := cmd + " -f " + cmd + "-json"
		assert.True(t, parser.ParseString(cmdStr))
		assert.Empty(t, parser.GetErrors())
		assert.Equal(t, 0, parser.ExecuteCommands())
	}
}

// TestParser_ShortFlagBackwardCompatibility ensures old code still works
func TestParser_ShortFlagBackwardCompatibility(t *testing.T) {
	parser := NewParser()

	// Old style: only global flags with short forms
	parser.AddFlag("verbose", NewArg(WithShortFlag("v"), WithType(types.Standalone)))
	parser.AddFlag("output", NewArg(WithShortFlag("o")))

	// Should work as before
	assert.True(t, parser.ParseString("-v -o output.txt"))
	verbose, _ := parser.GetBool("verbose")
	output, _ := parser.Get("output")

	assert.True(t, verbose)
	assert.Equal(t, "output.txt", output)
}

type MigrateCommand struct {
	Up     struct{ Exec CommandFunc } `goopt:"kind:command;name:up;desc:Run pending migrations"`
	Down   struct{ Exec CommandFunc } `goopt:"kind:command;name:down;desc:Rollback last migration"`
	Status struct{ Exec CommandFunc } `goopt:"kind:command;name:status;desc:Show migration status"`
	Create struct {
		Name string `goopt:"pos:0;desc:Migration name"`
		Exec CommandFunc
	} `goopt:"kind:command;name:create;desc:Create new migration"`
}

type PluginCommand struct {
	List    struct{ Exec CommandFunc } `goopt:"kind:command;name:list;desc:List installed plugins"`
	Install struct {
		Path string `goopt:"pos:0;desc:Plugin path or URL;required:true"`
		Exec CommandFunc
	} `goopt:"kind:command;name:install;desc:Install a plugin"`
	Remove struct {
		Name string `goopt:"pos:0;desc:Plugin name;required:true"`
		Exec CommandFunc
	} `goopt:"kind:command;name:remove;desc:Remove a plugin"`
	Enable struct {
		Name string `goopt:"pos:0;desc:Plugin name;required:true"`
		Exec CommandFunc
	} `goopt:"kind:command;name:enable;desc:Enable a plugin"`
	Disable struct {
		Name    string `goopt:"pos:0;desc:Plugin name;required:true"`
		Verbose bool   `goopt:"short:v;desc:Show verbose output"`
		Path    string `goopt:"pos:1;desc:Plugin path;required:true"`
		Exec    CommandFunc
	} `goopt:"kind:command;name:disable;desc:Disable a plugin"`
}

type testCommandPositional struct {
	Help    bool           `goopt:"short:h;desc:Show help"`
	Migrate MigrateCommand `goopt:"kind:command;name:migrate;desc:Database migration commands"`
	Plugin  PluginCommand  `goopt:"kind:command;name:plugin;desc:Import data from other systems"`
}

func TestParser_CommandPositionalWithFlagOrder(t *testing.T) {
	var cmd *testCommandPositional

	// Test case: flag after positionals in single command
	cmd = &testCommandPositional{}
	p, err := NewParserFromStruct(cmd)
	assert.NoError(t, err)
	ok := p.ParseString("plugin disable name1 path1 -v")
	if !ok {
		t.Logf("Parse errors for 'plugin disable name1 path1 -v': %v", p.GetErrors())
	}
	assert.True(t, ok)
	assert.Equal(t, "name1", cmd.Plugin.Disable.Name)
	assert.Equal(t, "path1", cmd.Plugin.Disable.Path)
	assert.True(t, cmd.Plugin.Disable.Verbose)

	// Test case: flag between positionals
	cmd = &testCommandPositional{}
	p, err = NewParserFromStruct(cmd)
	assert.NoError(t, err)
	ok = p.ParseString("plugin disable name2 -v path2")
	if !ok {
		t.Logf("Parse errors for 'plugin disable name2 -v path2': %v", p.GetErrors())
	}
	assert.True(t, ok)
	assert.Equal(t, "name2", cmd.Plugin.Disable.Name)
	assert.Equal(t, "path2", cmd.Plugin.Disable.Path)
	assert.True(t, cmd.Plugin.Disable.Verbose)
}

func TestParser_CommandPositional(t *testing.T) {
	var cmd *testCommandPositional
	cmd = &testCommandPositional{}
	p, err := NewParserFromStruct(cmd)
	assert.NoError(t, err)
	ok := p.ParseString("plugin install test")
	if !ok {
		t.Logf("Parse errors for 'plugin install test': %v", p.GetErrors())
	}
	assert.True(t, ok)
	assert.Equal(t, "test", cmd.Plugin.Install.Path)
	cmd = &testCommandPositional{}
	p, err = NewParserFromStruct(&cmd)
	assert.NoError(t, err)
	ok = p.ParseString("migrate create test plugin remove jar")
	if !ok {
		t.Logf("Parse errors: %v", p.GetErrors())
	}
	assert.True(t, ok)
	assert.Equal(t, "test", cmd.Migrate.Create.Name)
	assert.Equal(t, "jar", cmd.Plugin.Remove.Name)
	cmd = &testCommandPositional{}
	p, err = NewParserFromStruct(&cmd)
	assert.NoError(t, err)
	ok = p.ParseString("plugin disable test2 -v ./loc")
	if !ok {
		t.Logf("Parse errors: %v", p.GetErrors())
	}
	assert.True(t, ok)
	assert.Equal(t, "test2", cmd.Plugin.Disable.Name)
	assert.Equal(t, "./loc", cmd.Plugin.Disable.Path)
	assert.True(t, cmd.Plugin.Disable.Verbose)

	// Test chained commands without flags first
	cmd = &testCommandPositional{}
	p, err = NewParserFromStruct(cmd)
	assert.NoError(t, err)
	ok = p.ParseString("migrate create test1 plugin disable test2 ./loc")
	if !ok {
		t.Logf("Parse errors for chained without flags: %v", p.GetErrors())
	}
	assert.True(t, ok)
	assert.Equal(t, "test1", cmd.Migrate.Create.Name)
	assert.Equal(t, "test2", cmd.Plugin.Disable.Name)
	assert.Equal(t, "./loc", cmd.Plugin.Disable.Path)

	// Now test with the flag - place flag before last positional
	cmd = &testCommandPositional{}
	p, err = NewParserFromStruct(cmd)
	assert.NoError(t, err)
	ok = p.ParseString("migrate create test1 plugin disable -v test2 ./loc")
	if !ok {
		t.Logf("Parse errors for chained with flag before positionals: %v", p.GetErrors())
	}
	assert.True(t, ok)
	assert.Equal(t, "test1", cmd.Migrate.Create.Name)
	assert.Equal(t, "test2", cmd.Plugin.Disable.Name)
	assert.Equal(t, "./loc", cmd.Plugin.Disable.Path)
	assert.True(t, cmd.Plugin.Disable.Verbose)
}

type Test struct {
	Include []string `goopt:"short:i;desc:a slice of strings"`
}

func TestParser_RepeatedFlagsWithChainedType(t *testing.T) {
	t.Run("using struct-tags", func(t *testing.T) {
		test := &Test{}
		p, err := NewParserFromStruct(test)
		assert.NoError(t, err)
		success := p.Parse([]string{"-i", "path1", "--include", "path2", "-i", "path3"})
		assert.True(t, success)
		assert.Equal(t, 0, p.GetErrorCount())
		assert.Equal(t, []string{"path1", "path2", "path3"}, test.Include)
	})
	t.Run("repeated string slice flags", func(t *testing.T) {
		var includes []string
		p := NewParser()
		err := p.BindFlag(&includes, "include", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// Test repeated flag pattern
		success := p.Parse([]string{"--include", "path1", "--include", "path2", "--include", "path3"})
		assert.True(t, success)
		assert.Equal(t, 0, p.GetErrorCount())
		assert.Equal(t, []string{"path1", "path2", "path3"}, includes)
	})

	t.Run("repeated int slice flags", func(t *testing.T) {
		var ports []int
		p := NewParser()
		err := p.BindFlag(&ports, "port", NewArg(WithType(types.Chained), WithShortFlag("p")))
		assert.NoError(t, err)

		// Test repeated flag pattern with short flags
		success := p.Parse([]string{"-p", "8080", "-p", "8081", "-p", "8082"})
		assert.True(t, success)
		assert.Equal(t, []int{8080, 8081, 8082}, ports)
	})

	t.Run("mixed comma-separated and repeated flags", func(t *testing.T) {
		var tags []string
		p := NewParser()
		err := p.BindFlag(&tags, "tag", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// Test mixed pattern
		success := p.Parse([]string{"--tag", "dev,prod", "--tag", "staging", "--tag", "qa"})
		assert.True(t, success)
		assert.Equal(t, []string{"dev", "prod", "staging", "qa"}, tags)
	})

	t.Run("repeated flags with custom delimiter", func(t *testing.T) {
		var values []string
		p := NewParser()
		p.SetListDelimiterFunc(func(r rune) bool { return r == ';' })
		err := p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// Test with custom delimiter
		success := p.Parse([]string{"--value", "a;b", "--value", "c;d"})
		assert.True(t, success)
		assert.Equal(t, []string{"a", "b", "c", "d"}, values)
	})

	t.Run("repeated flags in command context", func(t *testing.T) {
		type Config struct {
			Deploy struct {
				Hosts []string `goopt:"name:host;type:chained"`
			} `goopt:"kind:command;name:deploy"`
		}

		var cfg Config
		p, err := NewParserFromStruct(&cfg)
		assert.NoError(t, err)

		// Test repeated flags in command context
		success := p.Parse([]string{"deploy", "--host", "server1", "--host", "server2", "--host", "server3"})
		assert.True(t, success)
		assert.Equal(t, []string{"server1", "server2", "server3"}, cfg.Deploy.Hosts)
	})

	t.Run("GetList with repeated flags", func(t *testing.T) {
		var includes []string
		p := NewParser()
		err := p.BindFlag(&includes, "include", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		success := p.Parse([]string{"--include", "path1", "--include", "path2"})
		assert.True(t, success)

		// GetList should return all values
		list, err := p.GetList("include")
		assert.NoError(t, err)
		assert.Equal(t, []string{"path1", "path2"}, list)
	})

	t.Run("repeated boolean slice flags", func(t *testing.T) {
		var flags []bool
		p := NewParser()
		err := p.BindFlag(&flags, "flag", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		success := p.Parse([]string{"--flag", "true", "--flag", "false", "--flag", "true"})
		assert.True(t, success)
		assert.Equal(t, []bool{true, false, true}, flags)
	})

	t.Run("repeated float slice flags", func(t *testing.T) {
		var weights []float64
		p := NewParser()
		err := p.BindFlag(&weights, "weight", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		success := p.Parse([]string{"--weight", "1.5", "--weight", "2.7", "--weight", "3.14"})
		assert.True(t, success)
		assert.Equal(t, []float64{1.5, 2.7, 3.14}, weights)
	})

	t.Run("single occurrence of chained flag", func(t *testing.T) {
		var values []string
		p := NewParser()
		err := p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// Should work normally with single occurrence
		success := p.Parse([]string{"--value", "a,b,c"})
		assert.True(t, success)
		assert.Equal(t, []string{"a", "b", "c"}, values)
	})

	t.Run("empty repeated flags", func(t *testing.T) {
		var values []string
		p := NewParser()
		err := p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// No flags provided
		success := p.Parse([]string{})
		assert.True(t, success)
		assert.Empty(t, values)
	})

	t.Run("repeated flags with invalid values", func(t *testing.T) {
		var ports []int
		p := NewParser()
		err := p.BindFlag(&ports, "port", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// Test with invalid int values
		success := p.Parse([]string{"--port", "8080", "--port", "not-a-number", "--port", "8082"})
		assert.False(t, success)
		assert.Greater(t, p.GetErrorCount(), 0)
		// Valid values are still collected
		assert.Equal(t, []int{8080, 8082}, ports)
	})

	t.Run("repeated flags with accepted values", func(t *testing.T) {
		var envs []string
		p := NewParser()
		err := p.BindFlag(&envs, "env", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// Add accepted values constraint
		err = p.AcceptPatterns("env", []types.PatternValue{
			{Pattern: "^(dev|staging|prod)$", Description: "Environment name"},
		})
		assert.NoError(t, err)

		// Test with valid values
		success := p.Parse([]string{"--env", "dev", "--env", "staging"})
		assert.True(t, success)
		assert.Equal(t, []string{"dev", "staging"}, envs)

		// Reset for invalid test
		envs = nil
		p = NewParser()
		err = p.BindFlag(&envs, "env", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)
		err = p.AcceptPatterns("env", []types.PatternValue{
			{Pattern: "^(dev|staging|prod)$", Description: "Environment name"},
		})
		assert.NoError(t, err)

		// Test with invalid value
		success = p.Parse([]string{"--env", "dev", "--env", "test"})
		assert.False(t, success)
		assert.Greater(t, p.GetErrorCount(), 0)
	})
}

// TestRepeatedFlagsEdgeCases tests edge cases for repeated flags
func TestParser_RepeatedFlagsEdgeCases(t *testing.T) {
	t.Run("repeated flags with empty values between", func(t *testing.T) {
		var values []string
		p := NewParser()
		err := p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		success := p.Parse([]string{"--value", "a", "--value", "", "--value", "b"})
		assert.True(t, success)
		// strings.FieldsFunc skips empty strings by default
		assert.Equal(t, []string{"a", "b"}, values)
	})

	t.Run("repeated flags with whitespace-only values", func(t *testing.T) {
		var values []string
		p := NewParser()
		p.SetListDelimiterFunc(func(r rune) bool { return r == ',' })
		err := p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		success := p.Parse([]string{"--value", "  ", "--value", "b"})
		assert.True(t, success)
		assert.Equal(t, []string{"  ", "b"}, values)
	})

	t.Run("repeated flags reset between parses", func(t *testing.T) {
		var values []string
		p := NewParser()
		err := p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		// First parse
		success := p.Parse([]string{"--value", "a", "--value", "b"})
		assert.True(t, success)
		assert.Equal(t, []string{"a", "b"}, values)

		// Reset values and parse again
		values = nil
		p = NewParser()
		err = p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		success = p.Parse([]string{"--value", "c", "--value", "d"})
		assert.True(t, success)
		assert.Equal(t, []string{"c", "d"}, values)
	})

	t.Run("repeated flags with very long values", func(t *testing.T) {
		var values []string
		p := NewParser()
		err := p.BindFlag(&values, "value", NewArg(WithType(types.Chained)))
		assert.NoError(t, err)

		longValue := string(make([]byte, 1000))
		for i := range longValue {
			longValue = longValue[:i] + "a" + longValue[i+1:]
		}

		success := p.Parse([]string{"--value", longValue, "--value", "short"})
		assert.True(t, success)
		assert.Equal(t, 2, len(values))
		assert.Equal(t, 1000, len(values[0]))
		assert.Equal(t, "short", values[1])
	})
}

// TestRepeatedFlagsWithStructTags tests repeated flags when using struct tags
func TestParser_FlagsWithStructTags(t *testing.T) {
	t.Run("struct with various repeated slice types", func(t *testing.T) {
		type Config struct {
			Strings   []string        `goopt:"name:string;type:chained"`
			Ints      []int           `goopt:"name:int;type:chained"`
			Floats    []float64       `goopt:"name:float;type:chained"`
			Durations []time.Duration `goopt:"name:duration;type:chained"`
		}

		var cfg Config
		p, err := NewParserFromStruct(&cfg)
		assert.NoError(t, err)

		success := p.Parse([]string{
			"--string", "a", "--string", "b",
			"--int", "1", "--int", "2",
			"--float", "1.5", "--float", "2.5",
			"--duration", "1h", "--duration", "30m",
		})
		assert.True(t, success)
		assert.Equal(t, []string{"a", "b"}, cfg.Strings)
		assert.Equal(t, []int{1, 2}, cfg.Ints)
		assert.Equal(t, []float64{1.5, 2.5}, cfg.Floats)
		assert.Equal(t, []time.Duration{time.Hour, 30 * time.Minute}, cfg.Durations)
	})
}

func TestParser_ValidationHooks(t *testing.T) {
	t.Run("validate email flag", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("email", NewArg(
				WithType(types.Single),
				WithValidator(validation.Email()),
			)),
		)
		assert.NoError(t, err)

		// Valid email
		success := parser.Parse([]string{"--email", "test@example.com"})
		assert.True(t, success)
		assert.Equal(t, "test@example.com", parser.options["email"])

		// Invalid email
		parser = NewParser()
		parser.AddFlag("email", NewArg(
			WithType(types.Single),
			WithValidator(validation.Email()),
		))
		success = parser.Parse([]string{"--email", "not-an-email"})
		assert.False(t, success)
		assert.Greater(t, len(parser.errors), 0)
	})

	t.Run("validate URL with schemes", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("url", NewArg(
				WithType(types.Single),
				WithValidator(validation.URL("http", "https")),
			)),
		)
		assert.NoError(t, err)

		// Valid HTTPS URL
		success := parser.Parse([]string{"--url", "https://example.com"})
		assert.True(t, success)

		// Invalid scheme
		parser = NewParser()
		parser.AddFlag("url", NewArg(
			WithType(types.Single),
			WithValidator(validation.URL("http", "https")),
		))
		success = parser.Parse([]string{"--url", "ftp://example.com"})
		assert.False(t, success)
	})

	t.Run("validate range", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("port", NewArg(
				WithType(types.Single),
				WithValidator(validation.Port()),
			)),
		)
		assert.NoError(t, err)

		// Valid port
		success := parser.Parse([]string{"--port", "8080"})
		assert.True(t, success)

		// Invalid port (too high)
		parser = NewParser()
		parser.AddFlag("port", NewArg(
			WithType(types.Single),
			WithValidator(validation.Port()),
		))
		success = parser.Parse([]string{"--port", "70000"})
		assert.False(t, success)

		// Invalid port (not a number)
		parser = NewParser()
		parser.AddFlag("port", NewArg(
			WithType(types.Single),
			WithValidator(validation.Port()),
		))
		success = parser.Parse([]string{"--port", "abc"})
		assert.False(t, success)
	})

	t.Run("validate string length", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("username", NewArg(
				WithType(types.Single),
				WithValidators(
					validation.MinLength(3),
					validation.MaxLength(20),
				),
			)),
		)
		assert.NoError(t, err)

		// Valid length
		success := parser.Parse([]string{"--username", "johndoe"})
		assert.True(t, success)

		// Too short
		parser = NewParser()
		parser.AddFlag("username", NewArg(
			WithType(types.Single),
			WithValidator(validation.MinLength(3)),
		))
		success = parser.Parse([]string{"--username", "ab"})
		assert.False(t, success)
	})

	t.Run("multiple validators with All", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("password", NewArg(
				WithType(types.Single),
				WithValidator(validation.All(
					validation.MinLength(8),
					validation.Regex(`[A-Z]`, "Must contain uppercase"), // At least one uppercase
					validation.Regex(`[0-9]`, "Must contain digit"),     // At least one number
				)),
			)),
		)
		assert.NoError(t, err)

		// Valid password
		success := parser.Parse([]string{"--password", "Password123"})
		assert.True(t, success)

		// Invalid - no uppercase
		parser = NewParser()
		parser.AddFlag("password", NewArg(
			WithType(types.Single),
			WithValidator(validation.All(
				validation.MinLength(8),
				validation.Regex(`[A-Z]`, "Must contain uppercase"),
			)),
		))
		success = parser.Parse([]string{"--password", "password123"})
		assert.False(t, success)
	})

	t.Run("validators with Any", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("id", NewArg(
				WithType(types.Single),
				WithValidator(validation.Any(
					validation.Email(),
					validation.Regex(`^[0-9]{6,}$`, "Pattern: ^[0-9]{6,}$"), // 6+ digit ID
				)),
			)),
		)
		assert.NoError(t, err)

		// Valid email
		success := parser.Parse([]string{"--id", "user@example.com"})
		assert.True(t, success)

		// Valid numeric ID
		parser = NewParser()
		parser.AddFlag("id", NewArg(
			WithType(types.Single),
			WithValidator(validation.Any(
				validation.Email(),
				validation.Regex(`^[0-9]{6,}$`, "Pattern: ^[0-9]{6,}$"),
			)),
		))
		success = parser.Parse([]string{"--id", "123456"})
		assert.True(t, success)

		// Invalid - neither email nor 6+ digits
		parser = NewParser()
		parser.AddFlag("id", NewArg(
			WithType(types.Single),
			WithValidator(validation.Any(
				validation.Email(),
				validation.Regex(`^[0-9]{6,}$`, "Pattern: ^[0-9]{6,}$"),
			)),
		))
		success = parser.Parse([]string{"--id", "12345"})
		assert.False(t, success)
	})

	t.Run("validate standalone flag", func(t *testing.T) {
		// Custom validator that only accepts "true"
		onlyTrue := func(value string) error {
			if value != "true" {
				return errors.New("value must be true")
			}
			return nil
		}

		parser, err := NewParserWith(
			WithFlag("enabled", NewArg(
				WithType(types.Standalone),
				WithValidator(onlyTrue),
			)),
		)
		assert.NoError(t, err)

		// Valid - defaults to true
		success := parser.Parse([]string{"--enabled"})
		assert.True(t, success)

		// Invalid - explicitly false
		parser = NewParser()
		parser.AddFlag("enabled", NewArg(
			WithType(types.Standalone),
			WithValidator(onlyTrue),
		))
		success = parser.Parse([]string{"--enabled", "false"})
		assert.False(t, success)
	})

	t.Run("validate with filters and validators", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("name", NewArg(
				WithType(types.Single),
				WithPreValidationFilter(strings.TrimSpace),
				WithPostValidationFilter(strings.ToLower),
				WithValidator(validation.MinLength(3)),
			)),
		)
		assert.NoError(t, err)

		// Valid after filtering
		success := parser.Parse([]string{"--name", "  JOHN  "})
		assert.True(t, success)
		assert.Equal(t, "john", parser.options["name"])

		// Invalid after filtering (too short)
		parser = NewParser()
		parser.AddFlag("name", NewArg(
			WithType(types.Single),
			WithPreValidationFilter(strings.TrimSpace),
			WithValidator(validation.MinLength(3)),
		))
		success = parser.Parse([]string{"--name", "  AB  "})
		assert.False(t, success)
	})

	t.Run("add validators after flag creation", func(t *testing.T) {
		parser := NewParser()
		parser.AddFlag("age", NewArg(WithType(types.Single)))

		// Add validators later
		err := parser.AddFlagValidators("age",
			validation.Integer(),
			validation.Range(0, 150),
		)
		assert.NoError(t, err)

		// Valid age
		success := parser.Parse([]string{"--age", "25"})
		assert.True(t, success)

		// Invalid age (not a number)
		parser = NewParser()
		parser.AddFlag("age", NewArg(WithType(types.Single)))
		parser.AddFlagValidators("age", validation.Integer())
		success = parser.Parse([]string{"--age", "twenty-five"})
		assert.False(t, success)
	})

	t.Run("positional argument validation", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("file", NewArg(
				WithType(types.Single),
				WithPosition(0),
				WithValidator(validation.FileExtension(".txt", ".md")),
			)),
		)
		assert.NoError(t, err)

		// Valid extension
		success := parser.Parse([]string{"document.txt"})
		assert.True(t, success)

		// Invalid extension
		parser = NewParser()
		parser.AddFlag("file", NewArg(
			WithType(types.Single),
			WithPosition(0),
			WithValidator(validation.FileExtension(".txt", ".md")),
		))
		success = parser.Parse([]string{"document.pdf"})
		assert.False(t, success)
	})

	t.Run("custom validator", func(t *testing.T) {
		evenNumber := validation.Custom(func(value string) error {
			num, err := strconv.Atoi(value)
			if err != nil {
				return errors.New("value must be a number")
			}
			if num%2 != 0 {
				return errors.New("value must be even")
			}
			return nil
		})

		parser, err := NewParserWith(
			WithFlag("count", NewArg(
				WithType(types.Single),
				WithValidator(evenNumber),
			)),
		)
		assert.NoError(t, err)

		// Valid even number
		success := parser.Parse([]string{"--count", "4"})
		assert.True(t, success)

		// Invalid odd number
		parser = NewParser()
		parser.AddFlag("count", NewArg(
			WithType(types.Single),
			WithValidator(evenNumber),
		))
		success = parser.Parse([]string{"--count", "5"})
		assert.False(t, success)
	})

	// Note: Required() validator has been removed.
	// Use required:true flag attribute instead for required non-empty values.

	t.Run("clear and replace validators", func(t *testing.T) {
		parser := NewParser()
		parser.AddFlag("value", NewArg(WithType(types.Single)))

		// Add initial validator
		parser.AddFlagValidators("value", validation.MinLength(5))

		// Replace with new validators
		err := parser.SetFlagValidators("value",
			validation.MaxLength(10),
			validation.AlphaNumeric(),
		)
		assert.NoError(t, err)

		// Valid under new rules
		success := parser.Parse([]string{"--value", "abc123"})
		assert.True(t, success)

		// Would have failed old rule (min 5) but passes new rules
		parser = NewParser()
		parser.AddFlag("value", NewArg(WithType(types.Single)))
		parser.SetFlagValidators("value", validation.MaxLength(10))
		success = parser.Parse([]string{"--value", "ab"})
		assert.True(t, success)

		// Clear all validators
		err = parser.ClearFlagValidators("value")
		assert.NoError(t, err)

		// Any value should work now
		parser = NewParser()
		parser.AddFlag("value", NewArg(WithType(types.Single)))
		parser.AddFlagValidators("value", validation.MinLength(100)) // Add restrictive validator
		parser.ClearFlagValidators("value")                          // Clear it
		success = parser.Parse([]string{"--value", "x"})             // Short value should work
		assert.True(t, success)
	})

	t.Run("validate hostname", func(t *testing.T) {
		_, err := NewParserWith(
			WithFlag("host", NewArg(
				WithType(types.Single),
				WithValidator(validation.Hostname()),
			)),
		)
		assert.NoError(t, err)

		// Valid hostnames
		validHosts := []string{
			"localhost",
			"example.com",
			"sub.example.com",
			"test-server",
			"192-168-1-1",
		}

		for _, host := range validHosts {
			p := NewParser()
			p.AddFlag("host", NewArg(
				WithType(types.Single),
				WithValidator(validation.Hostname()),
			))
			success := p.Parse([]string{"--host", host})
			assert.True(t, success, "Expected %s to be valid", host)
		}

		// Invalid hostnames
		invalidHosts := []string{
			"-example.com",           // starts with hyphen
			"example.com-",           // ends with hyphen
			"ex ample.com",           // contains space
			"example..com",           // double dot
			strings.Repeat("a", 254), // too long
		}

		for _, host := range invalidHosts {
			p := NewParser()
			p.AddFlag("host", NewArg(
				WithType(types.Single),
				WithValidator(validation.Hostname()),
			))
			success := p.Parse([]string{"--host", host})
			assert.False(t, success, "Expected %s to be invalid", host)
		}
	})
}

func TestParser_ValidationWithSecureFlags(t *testing.T) {
	t.Run("validate secure password", func(t *testing.T) {
		// This test would need mock input, so we'll test the validator setup
		parser := NewParser()
		parser.AddFlag("password", NewArg(
			WithType(types.Single),
			WithSecurePrompt("Enter password: "),
			WithValidators(
				validation.MinLength(8),
				validation.Regex(`[A-Z]`, "Must contain uppercase"),
				validation.Regex(`[a-z]`, "Must contain lowercase"),
				validation.Regex(`[0-9]`, "Must contain digit"),
			),
		))

		// Verify validators were added
		flagInfo, found := parser.acceptedFlags.Get("password")
		assert.True(t, found)
		assert.Len(t, flagInfo.Argument.Validators, 4)
	})
}

func TestParser_ValidationIntegration(t *testing.T) {
	t.Run("complex validation scenario", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("email", NewArg(
				WithType(types.Single),
				WithRequired(true),
				WithValidator(validation.Email()),
			)),
			WithFlag("age", NewArg(
				WithType(types.Single),
				WithValidators(
					validation.Integer(),
					validation.Range(18, 100),
				),
			)),
			WithFlag("website", NewArg(
				WithType(types.Single),
				WithValidator(validation.URL("http", "https")),
			)),
			WithFlag("username", NewArg(
				WithType(types.Single),
				WithRequired(true),
				WithValidators(
					validation.MinLength(3),
					validation.MaxLength(20),
					validation.Identifier(),
				),
			)),
		)
		assert.NoError(t, err)

		// All valid
		success := parser.Parse([]string{
			"--email", "user@example.com",
			"--age", "25",
			"--website", "https://example.com",
			"--username", "john_doe",
		})
		assert.True(t, success)

		// Invalid username (has hyphen, not identifier)
		parser = NewParser()
		parser.AddFlag("username", NewArg(
			WithType(types.Single),
			WithValidator(validation.Identifier()),
		))
		success = parser.Parse([]string{"--username", "john-doe"})
		assert.False(t, success)
	})

	t.Run("validation with parser config functions", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("port", NewArg(WithType(types.Single))),
			WithFlagValidators("port", validation.Port()),
		)
		assert.NoError(t, err)

		success := parser.Parse([]string{"--port", "8080"})
		assert.True(t, success)
	})
}

func TestParser_BuiltInValidators(t *testing.T) {
	tests := []struct {
		name      string
		validator validation.ValidatorFunc
		valid     []string
		invalid   []string
	}{
		{
			name:      "Integer",
			validator: validation.Integer(),
			valid:     []string{"0", "123", "-456", "+789"},
			invalid:   []string{"", "abc", "12.34", "1e5"},
		},
		{
			name:      "Float",
			validator: validation.Float(),
			valid:     []string{"0", "123", "-456.78", "1.23e4", ".5"},
			invalid:   []string{"", "abc", "1.2.3"},
		},
		{
			name:      "Boolean",
			validator: validation.Boolean(),
			valid:     []string{"true", "false", "1", "0", "True", "FALSE"},
			invalid:   []string{"", "yes", "no", "2"},
		},
		{
			name:      "AlphaNumeric",
			validator: validation.AlphaNumeric(),
			valid:     []string{"abc", "ABC", "123", "abc123", "ABC123"},
			invalid:   []string{"", "abc-123", "abc_123", "abc 123", "abc!"},
		},
		{
			name:      "NoWhitespace",
			validator: validation.NoWhitespace(),
			valid:     []string{"abc", "123", "abc-123", "abc_123"},
			invalid:   []string{"abc 123", "abc\t123", "abc\n123", " abc"},
		},
		{
			name:      "OneOf",
			validator: validation.IsOneOf("red", "green", "blue"),
			valid:     []string{"red", "green", "blue"},
			invalid:   []string{"", "yellow", "RED", "Green"},
		},
		{
			name:      "NotIn",
			validator: validation.IsNotOneOf("admin", "root", "system"),
			valid:     []string{"user", "guest", "john"},
			invalid:   []string{"admin", "root", "system"},
		},
		{
			name:      "IP",
			validator: validation.IP(),
			valid:     []string{"192.168.1.1", "0.0.0.0", "255.255.255.255", "::1", "2001:db8::1"},
			invalid:   []string{"", "192.168.1", "192.168.1.256", "not.an.ip", "192.168.1.1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, valid := range tt.valid {
				err := tt.validator(valid)
				assert.NoError(t, err, "Expected %q to be valid", valid)
			}

			for _, invalid := range tt.invalid {
				err := tt.validator(invalid)
				assert.Error(t, err, "Expected %q to be invalid", invalid)
			}
		})
	}
}

func TestParser_CombinedAcceptedValuesAndValidators(t *testing.T) {
	t.Run("email from specific domains", func(t *testing.T) {
		type Config struct {
			// Must be a valid email AND from specific domains
			Email string `goopt:"name:email;accepted:{pattern:.*@(company|example)\\.com$,desc:Company or Example email};validators:email()"`
		}

		tests := []struct {
			email      string
			shouldPass bool
			errorHint  string
		}{
			{"user@company.com", true, ""},
			{"admin@example.com", true, ""},
			{"user@gmail.com", false, "Company or Example email"}, // Valid email but wrong domain
			{"invalid-email", false, "Company or Example email"},  // Invalid format
			{"user@", false, "Company or Example email"},          // Invalid format
		}

		for _, tt := range tests {
			parser, _ := NewParserFromStruct(&Config{})
			success := parser.Parse([]string{"cmd", "--email", tt.email})

			if tt.shouldPass {
				assert.True(t, success, "Expected %s to be valid", tt.email)
			} else {
				assert.False(t, success, "Expected %s to be invalid", tt.email)
				if tt.errorHint != "" {
					errors := parser.GetErrors()
					found := false
					for _, err := range errors {
						if strings.Contains(err.Error(), tt.errorHint) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected error to contain '%s' for %s", tt.errorHint, tt.email)
				}
			}
		}
	})

	t.Run("project ID with format and length", func(t *testing.T) {
		type Config struct {
			// Must match pattern AND be exactly 12 characters
			ProjectID string `goopt:"name:project;accepted:{pattern:^PROJ-[0-9]+$,desc:Project ID format};validators:length(12)"`
		}

		tests := []struct {
			id         string
			shouldPass bool
		}{
			{"PROJ-1234567", true},   // Correct format and length
			{"PROJ-123", false},      // Correct format, wrong length
			{"PROJ-12345678", false}, // Correct format, wrong length
			{"TEST-1234567", false},  // Wrong format
			{"PROJ1234567", false},   // Missing dash
		}

		for _, tt := range tests {
			parser, _ := NewParserFromStruct(&Config{})
			success := parser.Parse([]string{"cmd", "--project", tt.id})

			if tt.shouldPass {
				assert.True(t, success, "Expected %s to be valid", tt.id)
			} else {
				assert.False(t, success, "Expected %s to be invalid", tt.id)
			}
		}
	})

	t.Run("port with pattern and range", func(t *testing.T) {
		type Config struct {
			// Must be in specific patterns AND within range
			Port int `goopt:"name:port;accepted:{pattern:^(8080|8443|9[0-9]{3})$,desc:Port 8080/8443/9xxx};validators:range(8000,9999)"`
		}

		tests := []struct {
			port       string
			shouldPass bool
		}{
			{"8080", true},   // Allowed by pattern
			{"8443", true},   // Allowed by pattern
			{"9000", true},   // Matches 9xxx pattern and in range
			{"9999", true},   // Matches 9xxx pattern and in range
			{"8081", false},  // In range but not in pattern
			{"7080", false},  // Not in pattern or range
			{"10000", false}, // Not in pattern or range
		}

		for _, tt := range tests {
			parser, _ := NewParserFromStruct(&Config{})
			success := parser.Parse([]string{"cmd", "--port", tt.port})

			if tt.shouldPass {
				assert.True(t, success, "Expected port %s to be valid", tt.port)
			} else {
				assert.False(t, success, "Expected port %s to be invalid", tt.port)
			}
		}
	})

	t.Run("chained values with both validations", func(t *testing.T) {
		type Config struct {
			// Each tag must match pattern AND be from allowed set
			Tags []string `goopt:"name:tags;type:chained;accepted:{pattern:^[a-z]+-[0-9]+$,desc:Environment tag format};validators:oneof(isoneof(env-1,env-2,test-1,test-2,prod-1,prod-2))"`
		}

		parser, _ := NewParserFromStruct(&Config{})

		// Valid: all match pattern and are in allowed set
		success := parser.Parse([]string{"cmd", "--tags", "env-1,test-2,prod-1"})
		assert.True(t, success, "Expected valid tags to pass")

		// Invalid: env-3 matches pattern but not in allowed set
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--tags", "env-1,env-3"})
		assert.False(t, success2, "Expected env-3 to fail validator")

		// Invalid: ENV-1 is in set but doesn't match lowercase pattern
		parser3, _ := NewParserFromStruct(&Config{})
		success3 := parser3.Parse([]string{"cmd", "--tags", "ENV-1"})
		assert.False(t, success3, "Expected ENV-1 to fail pattern")
	})

	t.Run("validation order - accepted values first", func(t *testing.T) {
		type Config struct {
			// Pattern check should fail before email validation
			Email string `goopt:"name:email;accepted:{pattern:^[a-z]+@example\\.com$,desc:Lowercase example.com email};validators:email()"`
		}

		parser, _ := NewParserFromStruct(&Config{})
		success := parser.Parse([]string{"cmd", "--email", "User@example.com"})
		assert.False(t, success, "Expected uppercase to fail pattern check")

		errors := parser.GetErrors()
		found := false
		for _, err := range errors {
			// Should get pattern error, not email validation error
			if strings.Contains(err.Error(), "Lowercase example.com email") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected pattern error, not email validator error")
	})

	t.Run("complex validation with filters", func(t *testing.T) {
		type Config struct {
			// Combine with secure field (though we can't test secure input easily)
			APIKey string `goopt:"name:api-key;validators:regex(^[A-Z]{4}-[0-9]{4}-[A-Z]{4}$),length(14)"`

			// Multiple validators on username
			Username string `goopt:"name:username;validators:minlength(3),maxlength(20),alphanumeric(),nowhitespace()"`
		}

		parser, _ := NewParserFromStruct(&Config{})
		success := parser.Parse([]string{"cmd",
			"--api-key", "ABCD-1234-EFGH",
			"--username", "johndoe123",
		})
		assert.True(t, success, "Expected valid values to pass")

		// Test invalid API key format
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--api-key", "abcd-1234-efgh"})
		assert.False(t, success2, "Expected lowercase to fail")

		// Test invalid username (with space)
		parser3, _ := NewParserFromStruct(&Config{})
		success3 := parser3.Parse([]string{"cmd", "--username", "john doe"})
		assert.False(t, success3, "Expected username with space to fail")
	})
}

func TestParser_ValidatorEdgeCases(t *testing.T) {
	t.Run("empty value handling", func(t *testing.T) {
		type Config struct {
			Optional string `goopt:"name:optional;validators:minlength(5)"`
			Required string `goopt:"name:required;required:true;validators:minlength(5)"`
		}

		// Empty optional field should not trigger validator
		parser, _ := NewParserFromStruct(&Config{})
		success := parser.Parse([]string{"cmd", "--required", "hello"})
		assert.True(t, success, "Expected parsing to succeed with empty optional")

		// Empty required field should fail on required check, not validator
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd"})
		assert.False(t, success2, "Expected parsing to fail on required")
		errors := parser2.GetErrors()
		found := false
		for _, err := range errors {
			if strings.Contains(err.Error(), "required flag missing") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected required flag error")
	})

	t.Run("default values with validators", func(t *testing.T) {
		type Config struct {
			Port int `goopt:"name:port;default:8080;validators:range(1000,9999)"`
		}

		// Default value should pass validation
		cfg := &Config{}
		parser, _ := NewParserFromStruct(cfg)
		success := parser.Parse([]string{"cmd"})
		assert.True(t, success, "Expected default value to pass validation")
		assert.Equal(t, 8080, cfg.Port, "Expected default value to be set")
	})
}

func TestParser_ComposableValidatorsInStructTags(t *testing.T) {
	t.Run("OneOf validator in struct tag", func(t *testing.T) {
		type Config struct {
			// Accept either 5-digit ZIP or ZIP+4 format
			ZipCode string `goopt:"validators:oneof(regex({pattern:^\\d{5}$,desc:5-digit ZIP}),regex({pattern:^\\d{5}-\\d{4}$,desc:ZIP+4}))"`
		}

		tests := []struct {
			args  []string
			valid bool
			desc  string
		}{
			{[]string{"cmd", "--zip-code", "12345"}, true, "5-digit ZIP"},
			{[]string{"cmd", "--zip-code", "12345-6789"}, true, "ZIP+4 format"},
			{[]string{"cmd", "--zip-code", "1234"}, false, "too short"},
			{[]string{"cmd", "--zip-code", "123456"}, false, "too long"},
			{[]string{"cmd", "--zip-code", "abcde"}, false, "not digits"},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				parser, err := NewParserFromStruct(&Config{}, WithFlagNameConverter(ToKebabCase))
				assert.NoError(t, err)

				success := parser.Parse(tt.args)
				assert.Equal(t, tt.valid, success, "Parse result mismatch for %s", tt.desc)
			})
		}
	})

	t.Run("Not validator in struct tag", func(t *testing.T) {
		type Config struct {
			// Must be alphanumeric but NOT a reserved name
			Username string `goopt:"validators:all(alphanumeric,not(isoneof(admin,root,system,guest)))"`
		}

		tests := []struct {
			args  []string
			valid bool
			desc  string
		}{
			{[]string{"cmd", "--username", "john123"}, true, "valid username"},
			{[]string{"cmd", "--username", "admin"}, false, "reserved name"},
			{[]string{"cmd", "--username", "root"}, false, "reserved name"},
			{[]string{"cmd", "--username", "user-name"}, false, "not alphanumeric"},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				parser, err := NewParserFromStruct(&Config{}, WithFlagNameConverter(ToKebabCase))
				assert.NoError(t, err)

				success := parser.Parse(tt.args)
				assert.Equal(t, tt.valid, success, "Parse result mismatch for %s", tt.desc)
			})
		}
	})

	t.Run("Nested composition in struct tag", func(t *testing.T) {
		type Config struct {
			// Either (secret key: 32+ chars starting with sk_) OR (public key: 36+ chars starting with pk_)
			APIKey string `goopt:"validators:oneof(all(minlength(32),regex({pattern:^sk_,desc:Secret key})),all(minlength(36),regex({pattern:^pk_,desc:Public key})))"`
		}

		tests := []struct {
			args  []string
			valid bool
			desc  string
		}{
			{[]string{"cmd", "--api-key", "sk_" + strings.Repeat("a", 29)}, true, "valid secret key (32 chars)"},
			{[]string{"cmd", "--api-key", "pk_" + strings.Repeat("b", 33)}, true, "valid public key (36 chars)"},
			{[]string{"cmd", "--api-key", "sk_short"}, false, "secret key too short"},
			{[]string{"cmd", "--api-key", "pk_short"}, false, "public key too short"},
			{[]string{"cmd", "--api-key", "invalid_" + strings.Repeat("c", 40)}, false, "wrong prefix"},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				parser, err := NewParserFromStruct(&Config{}, WithFlagNameConverter(ToKebabCase))
				assert.NoError(t, err)

				success := parser.Parse(tt.args)
				assert.Equal(t, tt.valid, success, "Parse result mismatch for %s", tt.desc)
			})
		}
	})

	t.Run("All validator explicit in struct tag", func(t *testing.T) {
		type Config struct {
			// Explicitly use All (though comma-separated has same effect)
			Password string `goopt:"validators:all(minlength(8),regex({pattern:[A-Z],desc:uppercase}),regex({pattern:[0-9],desc:digit}))"`
		}

		tests := []struct {
			args  []string
			valid bool
			desc  string
		}{
			{[]string{"cmd", "--password", "Secret123"}, true, "valid password"},
			{[]string{"cmd", "--password", "secret123"}, false, "no uppercase"},
			{[]string{"cmd", "--password", "SecretPW"}, false, "no digit"},
			{[]string{"cmd", "--password", "Sec1"}, false, "too short"},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				parser, err := NewParserFromStruct(&Config{}, WithFlagNameConverter(ToKebabCase))
				assert.NoError(t, err)

				success := parser.Parse(tt.args)
				assert.Equal(t, tt.valid, success, "Parse result mismatch for %s", tt.desc)
			})
		}
	})

	t.Run("Complex nested validation", func(t *testing.T) {
		type Config struct {
			// Must be one of the valid ID formats, but NOT test IDs
			ID string `goopt:"validators:all(oneof(regex({pattern:^EMP-\\d{6}$,desc:Employee}),regex({pattern:^USR-\\d{8}$,desc:User})),not(isoneof(EMP-000000,USR-00000000)))"`
		}

		tests := []struct {
			args  []string
			valid bool
			desc  string
		}{
			{[]string{"cmd", "--id", "EMP-123456"}, true, "valid employee ID"},
			{[]string{"cmd", "--id", "USR-12345678"}, true, "valid user ID"},
			{[]string{"cmd", "--id", "EMP-000000"}, false, "test employee ID"},
			{[]string{"cmd", "--id", "USR-00000000"}, false, "test user ID"},
			{[]string{"cmd", "--id", "ADM-123456"}, false, "invalid prefix"},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				parser, err := NewParserFromStruct(&Config{}, WithFlagNameConverter(ToKebabCase))
				assert.NoError(t, err)

				success := parser.Parse(tt.args)
				assert.Equal(t, tt.valid, success, "Parse result mismatch for %s", tt.desc)
			})
		}
	})
}

func TestComposableValidatorsProgrammatic(t *testing.T) {
	t.Run("OneOf with regex validators", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("id", NewArg(
				WithDescription("User or Employee ID"),
				WithValidator(validation.OneOf(
					validation.Regex("^EMP-\\d{6}$", "Employee ID (EMP-123456)"),
					validation.Regex("^USR-\\d{8}$", "User ID (USR-12345678)"),
					validation.Regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", "UUID"),
				)),
			)),
		)
		assert.NoError(t, err)

		tests := []struct {
			args  []string
			valid bool
			desc  string
		}{
			{[]string{"cmd", "--id", "EMP-123456"}, true, "employee ID"},
			{[]string{"cmd", "--id", "USR-12345678"}, true, "user ID"},
			{[]string{"cmd", "--id", "550e8400-e29b-41d4-a716-446655440000"}, true, "UUID"},
			{[]string{"cmd", "--id", "invalid"}, false, "invalid format"},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				parser.ClearErrors() // Reset parser state
				success := parser.Parse(tt.args)
				assert.Equal(t, tt.valid, success, "Parse result mismatch for %s", tt.desc)
			})
		}
	})

	t.Run("Deep nesting of composable validators", func(t *testing.T) {
		// Complex rule: (Email OR (URL but not localhost)) AND (not containing "test")
		parser, err := NewParserWith(
			WithFlag("contact", NewArg(
				WithDescription("Contact info"),
				WithValidator(validation.All(
					validation.OneOf(
						validation.Email(),
						validation.All(
							validation.URL("http", "https"),
							validation.Not(validation.Regex("://localhost", "localhost URL")),
						),
					),
					validation.Not(validation.Regex("test", "contains 'test'")),
				)),
			)),
		)
		assert.NoError(t, err)

		tests := []struct {
			args  []string
			valid bool
			desc  string
		}{
			{[]string{"cmd", "--contact", "user@example.com"}, true, "valid email"},
			{[]string{"cmd", "--contact", "https://example.com"}, true, "valid URL"},
			{[]string{"cmd", "--contact", "http://localhost:8080"}, false, "localhost URL"},
			{[]string{"cmd", "--contact", "test@example.com"}, false, "contains 'test'"},
			{[]string{"cmd", "--contact", "https://test.com"}, false, "contains 'test'"},
		}

		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				parser.ClearErrors() // Reset parser state
				success := parser.Parse(tt.args)
				assert.Equal(t, tt.valid, success, "Parse result mismatch for %s", tt.desc)
			})
		}
	})
}

func TestParser_ValidatorParsing(t *testing.T) {
	t.Run("Parse oneof validator specs", func(t *testing.T) {
		validators, err := validation.ParseValidators([]string{
			"oneof(email,url,integer)",
		})
		assert.NoError(t, err)
		assert.Len(t, validators, 1)

		// Test the validator works
		validator := validators[0]
		assert.NoError(t, validator("user@example.com"), "should accept email")
		assert.NoError(t, validator("http://example.com"), "should accept URL")
		assert.NoError(t, validator("12345"), "should accept integer")
		assert.Error(t, validator("not-valid"), "should reject invalid input")
	})

	t.Run("Parse nested composition", func(t *testing.T) {
		validators, err := validation.ParseValidators([]string{
			"oneof(all(minlength(10),email),all(url,maxlength(50)))",
		})
		assert.NoError(t, err)
		assert.Len(t, validators, 1)

		// Test the validator works
		validator := validators[0]
		assert.NoError(t, validator("longuser@example.com"), "10+ char email")
		assert.NoError(t, validator("http://short.com"), "URL under 50 chars")
		assert.Error(t, validator("a@b.c"), "email too short")
		assert.Error(t, validator("http://"+strings.Repeat("x", 50)+".com"), "URL too long")
	})

	t.Run("Parse not validator", func(t *testing.T) {
		validators, err := validation.ParseValidators([]string{
			"not(isoneof(admin,root,system))",
		})
		assert.NoError(t, err)
		assert.Len(t, validators, 1)

		// Test the validator works
		validator := validators[0]
		assert.NoError(t, validator("user"), "should accept non-reserved")
		assert.Error(t, validator("admin"), "should reject reserved")
		assert.Error(t, validator("root"), "should reject reserved")
	})
}

func TestParser_ValidatorsParenthesesSyntax(t *testing.T) {
	t.Run("all validators should support parentheses syntax", func(t *testing.T) {
		type Config struct {
			// Simple validators
			Email  string `goopt:"name:email;validators:email()"`
			Number string `goopt:"name:number;validators:integer()"`

			// Validators with single argument
			MinLength string `goopt:"name:minlength;validators:minlength(5)"`
			MaxLength string `goopt:"name:maxlength;validators:maxlength(10)"`
			Pattern   string `goopt:"name:pattern;validators:regex(^[A-Z]+$)"`

			// Validators with multiple arguments
			Range   string `goopt:"name:range;validators:range(1,100)"`
			OneOf   string `goopt:"name:oneof;validators:isoneof(red,green,blue)"`
			FileExt string `goopt:"name:file;validators:fileext(.jpg,.png,.gif)"`

			// Complex regex with special characters
			Code     string `goopt:"name:code;validators:regex(^[A-Z]{2,4}-[0-9]{3,5}$)"`
			TimeCode string `goopt:"name:time;validators:regex(^[0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{3}$)"`

			// Compositional validators
			Composite string `goopt:"name:composite;validators:oneof(email(),regex(^[0-9]{10}$))"`
			NotEmail  string `goopt:"name:notemail;validators:not(email())"`
			AllChecks string `goopt:"name:allchecks;validators:all(minlength(5),maxlength(20),alphanumeric())"`

			// Multiple validators
			Username string `goopt:"name:username;validators:minlength(3),maxlength(20),alphanumeric()"`
		}

		// Test that parser can be created
		_, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err, "Parser should be created successfully with parentheses syntax")

		// Test some valid cases
		config := &Config{}
		parser2, _ := NewParserFromStruct(config)

		// Test email
		success := parser2.Parse([]string{"cmd", "--email", "test@example.com"})
		if !success {
			t.Logf("Email validation failed. Errors: %v", parser2.GetErrors())
		}
		assert.True(t, success)
		assert.Equal(t, "test@example.com", config.Email)

		// Test range
		parser3, _ := NewParserFromStruct(&Config{})
		success = parser3.Parse([]string{"cmd", "--range", "50"})
		assert.True(t, success)

		// Test oneof
		parser4, _ := NewParserFromStruct(&Config{})
		success = parser4.Parse([]string{"cmd", "--oneof", "green"})
		assert.True(t, success)

		// Test complex regex
		parser5, _ := NewParserFromStruct(&Config{})
		success = parser5.Parse([]string{"cmd", "--code", "ABC-1234"})
		assert.True(t, success)

		// Test compositional
		parser6, _ := NewParserFromStruct(&Config{})
		success = parser6.Parse([]string{"cmd", "--composite", "test@example.com"})
		assert.True(t, success)
	})

	t.Run("describable regex with parentheses", func(t *testing.T) {
		type Config struct {
			// Describable regex using cleaner syntax within parentheses
			Phone string `goopt:"name:phone;validators:regex(pattern:^\\+?[0-9]{10,15}$,desc:International phone number)"`
		}

		_, err := NewParserFromStruct(&Config{})
		if err != nil {
			t.Logf("Parser creation error: %v", err)
		}
		assert.NoError(t, err)

		// Valid phone
		config := &Config{}
		parser2, _ := NewParserFromStruct(config)
		success := parser2.Parse([]string{"cmd", "--phone", "+12345678901"})
		if !success {
			t.Logf("Phone validation failed. Errors: %v", parser2.GetErrors())
		}
		assert.True(t, success)
		assert.Equal(t, "+12345678901", config.Phone)

		// Invalid phone
		parser3, _ := NewParserFromStruct(&Config{})
		success = parser3.Parse([]string{"cmd", "--phone", "invalid"})
		assert.False(t, success)
	})

	t.Run("backward compatible JSON-like regex syntax", func(t *testing.T) {
		type Config struct {
			// JSON-like syntax should still work (using pattern without commas)
			Email string `goopt:"name:email;validators:regex({pattern:^[a-z]+@[a-z]+\\.[a-z]+$,desc:Lowercase email only})"`
		}

		_, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		// Valid email
		config := &Config{}
		parser2, _ := NewParserFromStruct(config)
		success := parser2.Parse([]string{"cmd", "--email", "test@example.com"})
		if !success {
			t.Logf("JSON-like syntax validation failed. Errors: %v", parser2.GetErrors())
		}
		assert.True(t, success)
		assert.Equal(t, "test@example.com", config.Email)
	})

	t.Run("colon syntax should not work anymore", func(t *testing.T) {
		type Config struct {
			// Old colon syntax should fail
			MinLen string `goopt:"name:minlen;validators:minlength:5"`
		}

		_, err := NewParserFromStruct(&Config{})
		// The error should happen during struct parsing when validators are created
		assert.Error(t, err, "Parser creation should fail with colon syntax validators")

		// Check that it's the right kind of error using errors.Is
		assert.True(t, errors.Is(err, errs.ErrProcessingField), "Should be a field processing error")

		// Check that the underlying cause is the validator syntax error
		var validatorErr *i18n.TrError
		if errors.As(err, &validatorErr) {
			// The wrapped error should be about parentheses syntax
			cause := errors.Unwrap(err)
			for cause != nil {
				if errors.Is(cause, errs.ErrValidatorMustUseParentheses) {
					break
				}
				cause = errors.Unwrap(cause)
			}
			assert.True(t, errors.Is(cause, errs.ErrValidatorMustUseParentheses), "Should have validator parentheses error in chain")
		}
	})
}

func TestParser_ValidatorsWithEscapedCommas(t *testing.T) {
	t.Run("regex with quantifier using escaped comma", func(t *testing.T) {
		type Config struct {
			// Now we can use {5,10} in regex by escaping the comma!
			Code string `goopt:"name:code;validators:regex(^[A-Z]{2,4}-[0-9]{3,5}$)"`
		}

		_, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		// Valid codes
		validTests := []string{
			"AB-123",     // Min lengths
			"ABCD-12345", // Max lengths
			"ABC-1234",   // Mid lengths
		}

		for _, code := range validTests {
			t.Run("valid_"+code, func(t *testing.T) {
				config := &Config{}
				parser2, _ := NewParserFromStruct(config)
				success := parser2.Parse([]string{"cmd", "--code", code})
				assert.True(t, success, "Expected %s to be valid", code)
				assert.Equal(t, code, config.Code)
			})
		}

		// Invalid codes
		invalidTests := []string{
			"A-123",     // Too few letters
			"ABCDE-123", // Too many letters
			"AB-12",     // Too few digits
			"AB-123456", // Too many digits
			"ab-123",    // Lowercase
			"AB_123",    // Wrong separator
		}

		for _, code := range invalidTests {
			t.Run("invalid_"+code, func(t *testing.T) {
				parser2, _ := NewParserFromStruct(&Config{})
				success := parser2.Parse([]string{"cmd", "--code", code})
				assert.False(t, success, "Expected %s to be invalid", code)
			})
		}
	})

	t.Run("multiple validators with escaped characters", func(t *testing.T) {
		type Config struct {
			// Complex pattern with escaped comma and colon
			TimeCode string `goopt:"name:time;validators:regex(^[0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{3}$),length(12)"`
		}

		config := &Config{}
		parser, err := NewParserFromStruct(config)
		assert.NoError(t, err)

		// Valid: HH:MM:SS,mmm format
		success := parser.Parse([]string{"cmd", "--time", "12:34:56,789"})
		assert.True(t, success)
		assert.Equal(t, "12:34:56,789", config.TimeCode)

		// Invalid: wrong format
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--time", "12-34-56.789"})
		assert.False(t, success2)
	})

	t.Run("password complexity with quantifiers", func(t *testing.T) {
		type Config struct {
			// Password must be 8-20 chars with specific requirements
			Password string `goopt:"name:password;validators:all(minlength(8),maxlength(20),regex([a-z]),regex([A-Z]),regex([0-9]),regex([@$!%*?&]))"`
		}

		_, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		// Valid passwords
		validPasswords := []string{
			"Pass123!",           // 8 chars - minimum
			"MyP@ssw0rd123",      // Medium length
			"VeryStr0ng!Pass123", // Near maximum
		}

		for _, pwd := range validPasswords {
			t.Run("valid_pwd_len_"+strconv.Itoa(len(pwd)), func(t *testing.T) {
				config := &Config{}
				parser2, _ := NewParserFromStruct(config)
				success := parser2.Parse([]string{"cmd", "--password", pwd})
				assert.True(t, success, "Expected password to be valid")
				assert.Equal(t, pwd, config.Password)
			})
		}

		// Invalid passwords
		invalidPasswords := []struct {
			pwd    string
			reason string
		}{
			{"Pass123", "no special char"},
			{"pass123!", "no uppercase"},
			{"PASS123!", "no lowercase"},
			{"Password!", "no number"},
			{"Pas12!", "too short"},
			{"ThisPasswordIsWay2Long!123", "too long"},
		}

		for _, test := range invalidPasswords {
			t.Run("invalid_"+test.reason, func(t *testing.T) {
				parser2, _ := NewParserFromStruct(&Config{})
				success := parser2.Parse([]string{"cmd", "--password", test.pwd})
				assert.False(t, success, "Expected password to be invalid: %s", test.reason)
			})
		}
	})

	t.Run("version number pattern", func(t *testing.T) {
		type Config struct {
			Version string `goopt:"name:version;validators:regex(^v?[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$)"`
		}

		_, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		validVersions := []string{
			"1.0.0",
			"v1.0.0",
			"12.34.56",
			"123.456.789",
		}

		for _, ver := range validVersions {
			config := &Config{}
			parser2, _ := NewParserFromStruct(config)
			success := parser2.Parse([]string{"cmd", "--version", ver})
			assert.True(t, success, "Expected %s to be valid", ver)
		}

		invalidVersions := []string{
			"1.0",        // Too few parts
			"1.0.0.0",    // Too many parts
			"1234.0.0",   // Too many digits
			"a.b.c",      // Not numbers
			"1.0.0-beta", // Extra suffix
		}

		for _, ver := range invalidVersions {
			parser2, _ := NewParserFromStruct(&Config{})
			success := parser2.Parse([]string{"cmd", "--version", ver})
			assert.False(t, success, "Expected %s to be invalid", ver)
		}
	})
}

// hasError checks if any error in the slice matches the target error using errors.Is
func hasError(errs []error, target error) bool {
	for _, err := range errs {
		if errors.Is(err, target) {
			return true
		}
		// Also check wrapped errors
		var wrappedErr interface{ Unwrap() error }
		if errors.As(err, &wrappedErr) {
			for unwrapped := wrappedErr.Unwrap(); unwrapped != nil; {
				if errors.Is(unwrapped, target) {
					return true
				}
				if w, ok := unwrapped.(interface{ Unwrap() error }); ok {
					unwrapped = w.Unwrap()
				} else {
					break
				}
			}
		}
	}
	return false
}

func TestStructTagValidators(t *testing.T) {
	t.Run("email validator", func(t *testing.T) {
		type Config struct {
			Email string `goopt:"name:email;validators:email()"`
		}
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		success := parser.Parse([]string{"cmd", "--email", "user@example.com"})
		assert.True(t, success)
	})

	t.Run("email validator - invalid", func(t *testing.T) {
		type Config struct {
			Email string `goopt:"name:email;validators:email()"`
		}
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		success := parser.Parse([]string{"cmd", "--email", "invalid-email"})
		assert.False(t, success)

		errors := parser.GetErrors()
		assert.NotEmpty(t, errors)
		assert.True(t, hasError(errors, errs.ErrInvalidEmailFormat), "Expected ErrInvalidEmailFormat")
	})

	t.Run("minlength validator", func(t *testing.T) {
		type Config struct {
			Password string `goopt:"name:password;validators:minlength(8)"`
		}
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		// Test too short
		success := parser.Parse([]string{"cmd", "--password", "short"})
		assert.False(t, success)
		errors := parser.GetErrors()
		assert.True(t, hasError(errors, errs.ErrMinLength), "Expected ErrMinLength")

		// Test valid length
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--password", "longpassword"})
		assert.True(t, success2)
	})

	t.Run("multiple validators", func(t *testing.T) {
		type Config struct {
			Username string `goopt:"name:username;validators:minlength(3),maxlength(20),alphanumeric()"`
		}

		// Valid username
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--username", "user123"})
		assert.True(t, success)

		// Too short
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--username", "ab"})
		assert.False(t, success2)

		// Non-alphanumeric
		parser3, _ := NewParserFromStruct(&Config{})
		success3 := parser3.Parse([]string{"cmd", "--username", "user-name"})
		assert.False(t, success3)
	})

	t.Run("range validator", func(t *testing.T) {
		type Config struct {
			Age int `goopt:"name:age;validators:range(18,100)"`
		}

		// Valid age
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--age", "25"})
		assert.True(t, success)

		// Too young
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--age", "17"})
		assert.False(t, success2)
		errors := parser2.GetErrors()
		assert.True(t, hasError(errors, errs.ErrValueBetween), "Expected ErrValueBetween")
	})

	t.Run("oneof validator", func(t *testing.T) {
		type Config struct {
			Color string `goopt:"name:color;validators:oneof(isoneof(red,green,blue))"`
		}

		// Valid color
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--color", "red"})
		assert.True(t, success)

		// Invalid color
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--color", "yellow"})
		assert.False(t, success2)
		errors := parser2.GetErrors()

		// Check that validation failed properly
		assert.True(t, len(errors) > 0, "Expected validation errors")
		// The error should be a validation error - check for wrapped ErrValueMustBeOneOf
		foundValidationError := false
		for _, err := range errors {
			if hasError([]error{err}, errs.ErrValueMustBeOneOf) || hasError([]error{err}, errs.ErrValidationCombinedFailed) {
				foundValidationError = true
				break
			}
		}
		assert.True(t, foundValidationError, "Expected validation error for invalid value")
	})

	t.Run("oneof validator with chained", func(t *testing.T) {
		type Config struct {
			Colors []string `goopt:"name:colors;type:chained;validators:oneof(isoneof(red,green,blue))"`
		}

		// Valid colors
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--colors", "red,green,blue"})
		assert.True(t, success)

		// Invalid color in list
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--colors", "red,yellow,blue"})
		assert.False(t, success2)
		errors := parser2.GetErrors()

		// Check that validation failed properly
		assert.True(t, len(errors) > 0, "Expected validation errors")
		// The error should be a validation error - check for wrapped ErrValueMustBeOneOf
		foundValidationError := false
		for _, err := range errors {
			if hasError([]error{err}, errs.ErrValueMustBeOneOf) || hasError([]error{err}, errs.ErrValidationCombinedFailed) {
				foundValidationError = true
				break
			}
		}
		assert.True(t, foundValidationError, "Expected validation error for invalid chained value")
	})

	t.Run("regex validator", func(t *testing.T) {
		type Config struct {
			Code string `goopt:"name:code;validators:regex(^[A-Z]{3}-\\d{3}$)"`
		}

		// Valid code
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--code", "ABC-123"})
		assert.True(t, success)

		// Invalid code
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--code", "abc-123"})
		assert.False(t, success2)
	})

	t.Run("url validator with schemes", func(t *testing.T) {
		type Config struct {
			Website string `goopt:"name:website;validators:url(https,http)"`
		}

		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--website", "https://example.com"})
		assert.True(t, success)
	})

	t.Run("file extension validator", func(t *testing.T) {
		type Config struct {
			File string `goopt:"name:file;validators:fileext(.jpg,.png,.gif)"`
		}

		// Valid extension
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--file", "image.png"})
		assert.True(t, success)

		// Invalid extension
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--file", "document.pdf"})
		assert.False(t, success2)
	})

	t.Run("integer validator on string field", func(t *testing.T) {
		type Config struct {
			Count string `goopt:"name:count;validators:integer"`
		}

		// Valid integer
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--count", "42"})
		assert.True(t, success)

		// Invalid integer
		parser2, _ := NewParserFromStruct(&Config{})
		success2 := parser2.Parse([]string{"cmd", "--count", "42.5"})
		assert.False(t, success2)
	})

	t.Run("invalid validator spec - should fail", func(t *testing.T) {
		type Config struct {
			Value string `goopt:"name:value;validators:unknown_validator"`
		}

		// Parser creation should succeed
		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)

		// But parsing should fail due to unknown validator
		success := parser.Parse([]string{"cmd", "--value", "anything"})
		assert.False(t, success)

		// Check that the error mentions the unknown validator
		parserErrors := parser.GetErrors()
		assert.NotEmpty(t, parserErrors)
		foundUnknownValidator := false
		for _, err := range parserErrors {
			if errors.Is(err, errs.ErrUnknownValidator) {
				foundUnknownValidator = true
				break
			}
		}
		assert.True(t, foundUnknownValidator, "Expected to find ErrUnknownValidator in errors")
	})

	t.Run("validator with required flag", func(t *testing.T) {
		type Config struct {
			ApiKey string `goopt:"name:api-key;required:true;validators:length(32)"`
		}

		parser, err := NewParserFromStruct(&Config{})
		assert.NoError(t, err)
		success := parser.Parse([]string{"cmd", "--api-key", "12345678901234567890123456789012"})
		assert.True(t, success)
	})
}

func TestParser_StructTagValidatorCombinations(t *testing.T) {
	// Test more complex validator combinations

	type Config struct {
		// Email with multiple validations
		AdminEmail string `goopt:"name:admin-email;validators:email,minlength(10);path:cmd"`

		// Port number with range
		Port int `goopt:"name:port;validators:range(1024,65535);path:cmd"`

		// Percentage with min/max
		Percentage float64 `goopt:"name:percentage;validators:min(0),max(100);path:cmd"`

		// Strong password requirements
		Password string `goopt:"name:password;validators:minlength(12);path:cmd"`

		// Identifier with specific pattern
		ProjectID string `goopt:"name:project-id;validators:regex(pattern:^proj-[0-9]{4}$,desc:goopt.msg.help_mode_flags_desc);path:cmd"`
	}

	parser, err := NewParserFromStruct(&Config{})
	assert.NoError(t, err)

	// Test valid inputs
	validArgs := []string{
		"cmd",
		"--admin-email", "admin@example.com",
		"--port", "8080",
		"--percentage", "75.5",
		"--password", "SecurePass123!",
		"--project-id", "proj-1234",
	}

	success := parser.Parse(validArgs)
	assert.True(t, success, "Expected valid inputs to parse successfully")

	// Test various invalid inputs
	invalidTests := []struct {
		args        []string
		expectedErr error
	}{
		{
			args:        []string{"cmd", "--admin-email", "a@b.c"}, // Too short
			expectedErr: errs.ErrMinLength,
		},
		{
			args:        []string{"cmd", "--port", "999"}, // Below range
			expectedErr: errs.ErrValueBetween,
		},
		{
			args:        []string{"cmd", "--percentage", "150"}, // Above max
			expectedErr: errs.ErrValueAtMost,
		},
		{
			args:        []string{"cmd", "--password", "shortpass"}, // Too short
			expectedErr: errs.ErrMinLength,
		},
		{
			args:        []string{"cmd", "--project-id", "proj-12"}, // Wrong format
			expectedErr: errs.ErrPatternMatch,
		},
	}

	for _, test := range invalidTests {
		parser, _ := NewParserFromStruct(&Config{})
		success := parser.Parse(test.args)
		assert.False(t, success, "Expected parsing to fail for args: %v", test.args)

		errors := parser.GetErrors()
		assert.True(t, hasError(errors, test.expectedErr), "Expected %v for args %v, got: %v", test.expectedErr, test.args, errors)
		fmt.Println(errors[0])
	}
}

func TestParser_StructTagValidatorIntegration(t *testing.T) {
	// Test integration with other struct tag features
	type ServerConfig struct {
		Host string `goopt:"name:host;desc:Server hostname;validators:hostname;default:localhost"`
		Port int    `goopt:"name:port;short:p;desc:Server port;validators:range(1,65535);default:8080"`

		Admin struct {
			Email    string `goopt:"name:email;desc:Admin email;validators:email;required:true"`
			Username string `goopt:"name:username;desc:Admin username;validators:identifier,minlength(3)"`
		} `goopt:"name:admin"`

		Features []string `goopt:"name:features;type:chained;validators:oneof(isoneof(auth,api,metrics,logging))"`
	}

	config := &ServerConfig{}
	parser, err := NewParserFromStruct(config)
	assert.NoError(t, err)

	// Test with valid inputs including chained values
	args := []string{
		"cmd",
		"--host", "api.example.com",
		"--port", "443",
		"--admin.email", "admin@example.com",
		"--admin.username", "admin_user",
		"--features", "auth,api,metrics",
	}

	success := parser.Parse(args)
	if !success {
		t.Logf("Parsing errors: %v", parser.GetErrors())
	}
	assert.True(t, success, "Expected parsing to succeed")
	assert.Equal(t, "api.example.com", config.Host)
	assert.Equal(t, 443, config.Port)
	assert.Equal(t, "admin@example.com", config.Admin.Email)
	assert.Equal(t, "admin_user", config.Admin.Username)
	assert.Equal(t, []string{"auth", "api", "metrics"}, config.Features)

	// Test invalid feature
	parser2, _ := NewParserFromStruct(&ServerConfig{})
	args2 := []string{
		"cmd",
		"--admin.email", "admin@example.com",
		"--features", "auth,invalid_feature",
	}

	success2 := parser2.Parse(args2)
	if success2 {
		t.Logf("Unexpected success - config: %+v", config)
	} else {
		t.Logf("Failed as expected with ErrValueMustBeOneOf")
	}
	assert.False(t, success2, "Expected parsing to fail with invalid feature")
}

func TestParser_AcceptedValuesI18n(t *testing.T) {
	t.Run("desc as translation key in user bundle", func(t *testing.T) {
		// Create a user bundle with translations
		userBundle := i18n.NewEmptyBundle()

		userBundle.AddLanguage(language.English, map[string]string{
			"format.description": "Supported output formats",
			"env.description":    "Environment to deploy to",
		})

		parser := NewParser()
		parser.SetUserBundle(userBundle)
		// Add flag with accepted values using translation keys
		parser.AddFlag("format", NewArg(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "json|yaml|xml", Description: "format.description"},
			}),
		))

		// Try invalid value
		success := parser.Parse([]string{"--format", "pdf"})
		assert.False(t, success)

		// Check that error message contains translated description
		assert.Len(t, parser.errors, 1)
		errMsg := parser.errors[0].Error()
		assert.Contains(t, errMsg, "Supported output formats")
		assert.NotContains(t, errMsg, "format.description") // Should not show the key
	})

	t.Run("desc as literal string when not a translation key", func(t *testing.T) {
		parser := NewParser()

		// Add flag with accepted values using literal descriptions
		parser.AddFlag("mode", NewArg(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "fast|slow", Description: "Processing speed"},
			}),
		))

		// Try invalid value
		success := parser.Parse([]string{"--mode", "medium"})
		assert.False(t, success)

		// Check that error message contains literal description
		assert.Len(t, parser.errors, 1)
		errMsg := parser.errors[0].Error()
		assert.Contains(t, errMsg, "Processing speed")
	})

	t.Run("multiple accepted patterns with mixed translation keys and literals", func(t *testing.T) {
		// Create a user bundle
		userBundle := i18n.NewEmptyBundle()
		userBundle.SetDefaultLanguage(language.English)
		userBundle.AddLanguage(language.English, map[string]string{
			"file.json.desc": "JSON configuration files",
		})

		parser := NewParser()
		parser.SetUserBundle(userBundle)

		// Add flag with multiple accepted values using NewArgE for better error handling
		arg, err := NewArgE(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: `.*\.json$`, Description: "file.json.desc"},    // Translation key
				{Pattern: `.*\.yaml$`, Description: "YAML config files"}, // Literal
				{Pattern: `.*\.toml$`, Description: "file.toml.desc"},    // Non-existent key (stays literal)
			}),
		)
		assert.NoError(t, err)
		err = parser.AddFlag("config", arg)
		assert.NoError(t, err)

		// Try invalid value
		success := parser.Parse([]string{"--config", "config.xml"})
		assert.False(t, success)

		// Check error message
		assert.Len(t, parser.errors, 1)
		errMsg := parser.errors[0].Error()
		// Should show all three patterns with appropriate descriptions
		assert.Contains(t, errMsg, "JSON configuration files") // Translated
		assert.Contains(t, errMsg, "YAML config files")        // Literal
		assert.Contains(t, errMsg, "file.toml.desc")           // Key not found, shown as-is
	})

	t.Run("fallback to system bundle", func(t *testing.T) {
		parser := NewParser()

		// Add flag using a goopt system translation key
		parser.AddFlag("required", NewArg(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "yes|no", Description: "goopt.error.required_flag"}, // System key
			}),
		))

		// Try invalid value
		success := parser.Parse([]string{"--required", "maybe"})
		assert.False(t, success)

		// Check that it found the system translation
		assert.Len(t, parser.errors, 1)
		errMsg := parser.errors[0].Error()
		assert.Contains(t, errMsg, "required flag missing") // System translation
	})

	t.Run("pattern shown when description is empty", func(t *testing.T) {
		parser := NewParser()

		// Add flag with no description
		parser.AddFlag("id", NewArg(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "[A-Z]{3}[0-9]{4}", Description: ""},
			}),
		))

		// Try invalid value
		success := parser.Parse([]string{"--id", "invalid"})
		assert.False(t, success)

		// Check that pattern is shown
		assert.Len(t, parser.errors, 1)
		errMsg := parser.errors[0].Error()
		assert.Contains(t, errMsg, "[A-Z]{3}[0-9]{4}")
	})

	t.Run("user bundle takes precedence over system bundle", func(t *testing.T) {
		// Create user bundle that overrides a system key
		userBundle := i18n.NewEmptyBundle()
		userBundle.SetDefaultLanguage(language.English)
		userBundle.AddLanguage(language.English, map[string]string{
			"goopt.error.required_flag": "Custom required message",
		})

		parser := NewParser()
		parser.SetUserBundle(userBundle)

		// Add flag using the same key
		parser.AddFlag("test", NewArg(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "a|b", Description: "goopt.error.required_flag"},
			}),
		))

		// Try invalid value
		success := parser.Parse([]string{"--test", "c"})
		assert.False(t, success)

		// Should use user's translation, not system's
		assert.Len(t, parser.errors, 1)
		errMsg := parser.errors[0].Error()
		assert.Contains(t, errMsg, "Custom required message")
		assert.NotContains(t, errMsg, "required flag missing")
	})

	t.Run("different languages", func(t *testing.T) {
		// Create user bundle with multiple languages
		userBundle := i18n.NewEmptyBundle()
		userBundle.SetDefaultLanguage(language.English)
		userBundle.AddLanguage(language.English, map[string]string{
			"speed.desc": "Speed setting",
		})
		userBundle.AddLanguage(language.German, map[string]string{
			"speed.desc": "Geschwindigkeitseinstellung",
		})
		userBundle.AddLanguage(language.French, map[string]string{
			"speed.desc": "Réglage de vitesse",
		})

		// Test each language
		testCases := []struct {
			lang     language.Tag
			expected string
		}{
			{language.English, "Speed setting"},
			{language.German, "Geschwindigkeitseinstellung"},
			{language.French, "Réglage de vitesse"},
		}

		for _, tc := range testCases {
			t.Run(tc.lang.String(), func(t *testing.T) {
				// Create a new bundle for each test with the desired language
				testBundle := i18n.NewEmptyBundle()
				testBundle.SetDefaultLanguage(tc.lang)
				testBundle.AddLanguage(language.English, map[string]string{
					"speed.desc": "Speed setting",
				})
				testBundle.AddLanguage(language.German, map[string]string{
					"speed.desc": "Geschwindigkeitseinstellung",
				})
				testBundle.AddLanguage(language.French, map[string]string{
					"speed.desc": "Réglage de vitesse",
				})

				parser := NewParser()
				parser.SetUserBundle(testBundle)

				parser.AddFlag("speed", NewArg(
					WithType(types.Single),
					WithAcceptedValues([]types.PatternValue{
						{Pattern: "fast|slow", Description: "speed.desc"},
					}),
				))

				success := parser.Parse([]string{"--speed", "medium"})
				assert.False(t, success)

				assert.Len(t, parser.errors, 1)
				errMsg := parser.errors[0].Error()
				assert.Contains(t, errMsg, tc.expected)
			})
		}
	})
}

func TestParser_AcceptedValuesChained(t *testing.T) {
	t.Run("chained values with i18n", func(t *testing.T) {
		// Create user bundle
		userBundle := i18n.NewEmptyBundle()
		userBundle.SetDefaultLanguage(language.English)
		userBundle.AddLanguage(language.English, map[string]string{
			"log.levels": "Available log levels",
		})

		parser := NewParser()
		parser.SetUserBundle(userBundle)

		// Add chained flag
		parser.AddFlag("levels", NewArg(
			WithType(types.Chained),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "debug|info|warn|error", Description: "log.levels"},
			}),
		))

		// Try with invalid value in chain
		success := parser.Parse([]string{"--levels", "debug,info,fatal"})
		assert.False(t, success)

		// Check error contains translated description
		assert.Len(t, parser.errors, 1)
		errMsg := parser.errors[0].Error()
		assert.Contains(t, errMsg, "Available log levels")
	})
}

func TestParser_Getters(t *testing.T) {
	t.Run("GetAutoHelp", func(t *testing.T) {
		parser := NewParser()

		// Default should be true
		assert.True(t, parser.GetAutoHelp())

		// Test setting to false
		parser.SetAutoHelp(false)
		assert.False(t, parser.GetAutoHelp())

		// Test setting back to true
		parser.SetAutoHelp(true)
		assert.True(t, parser.GetAutoHelp())
	})

	t.Run("GetHelpFlags", func(t *testing.T) {
		parser := NewParser()

		// Default should be ["help", "h"]
		flags := parser.GetHelpFlags()
		assert.Equal(t, []string{"help", "h"}, flags)

		// Test custom flags
		parser.SetHelpFlags([]string{"ayuda", "a"})
		flags = parser.GetHelpFlags()
		assert.Equal(t, []string{"ayuda", "a"}, flags)
	})

	t.Run("GetAutoVersion", func(t *testing.T) {
		parser := NewParser()

		// Default should be true
		assert.True(t, parser.GetAutoVersion())

		// Test setting to false
		parser.SetAutoVersion(false)
		assert.False(t, parser.GetAutoVersion())
	})

	t.Run("GetVersionFlags", func(t *testing.T) {
		parser := NewParser()

		// Default should be ["version", "v"]
		flags := parser.GetVersionFlags()
		assert.Equal(t, []string{"version", "v"}, flags)

		// Test custom flags
		parser.SetVersionFlags([]string{"ver", "V"})
		flags = parser.GetVersionFlags()
		assert.Equal(t, []string{"ver", "V"}, flags)
	})
}

func TestParser_ExecutionHooks(t *testing.T) {
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

func TestParser_HookUseCases(t *testing.T) {
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

func TestParser_ExecuteCommandWithHooks(t *testing.T) {
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

func TestParser_HookErrorHandling(t *testing.T) {
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

func TestParser_HooksWithStructTags(t *testing.T) {
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

func TestParser_ValidationHook(t *testing.T) {
	t.Run("simple validation hook", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("min", NewArg(
				WithType(types.Single),
				WithValidator(validation.Integer()),
			)),
			WithFlag("max", NewArg(
				WithType(types.Single),
				WithValidator(validation.Integer()),
			)),
			WithValidationHook(func(p *Parser) error {
				minStr, _ := p.Get("min")
				maxStr, _ := p.Get("max")

				if minStr != "" && maxStr != "" {
					minVal, _ := strconv.Atoi(minStr)
					maxVal, _ := strconv.Atoi(maxStr)

					if minVal > maxVal {
						return errors.New("min cannot be greater than max")
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid case
		success := parser.Parse([]string{"cmd", "--min", "1", "--max", "10"})
		assert.True(t, success)

		// Invalid case
		parser2, _ := NewParserWith(
			WithFlag("min", NewArg(WithType(types.Single), WithValidator(validation.Integer()))),
			WithFlag("max", NewArg(WithType(types.Single), WithValidator(validation.Integer()))),
			WithValidationHook(func(p *Parser) error {
				minVal, err := p.GetInt("min", 64)
				assert.NoError(t, err)
				maxVal, err := p.GetInt("max", 64)
				assert.NoError(t, err)
				if minVal > maxVal {
					return errors.New("min cannot be greater than max")
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--min", "10", "--max", "5"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "min cannot be greater than max")
	})

	t.Run("struct-based validation hook", func(t *testing.T) {
		type DateRange struct {
			StartDate string `goopt:"name:start-date"`
			EndDate   string `goopt:"name:end-date"`
			MaxDays   int    `goopt:"name:max-days;default:30"`
		}

		config := &DateRange{}
		parser, err := NewParserFromStruct(config,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*DateRange](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.StartDate != "" && cfg.EndDate != "" {
					start, err1 := time.Parse("2006-01-02", cfg.StartDate)
					end, err2 := time.Parse("2006-01-02", cfg.EndDate)

					if err1 != nil || err2 != nil {
						return errors.New("invalid date format, use YYYY-MM-DD")
					}

					if start.After(end) {
						return errors.New("start date must be before end date")
					}

					days := int(end.Sub(start).Hours() / 24)
					if days > cfg.MaxDays {
						return fmt.Errorf("date range exceeds maximum of %d days", cfg.MaxDays)
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid case
		success := parser.Parse([]string{"cmd", "--start-date", "2024-01-01", "--end-date", "2024-01-15"})
		assert.True(t, success)

		// Invalid: start after end
		config2 := &DateRange{}
		parser2, _ := NewParserFromStruct(config2,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*DateRange](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.StartDate != "" && cfg.EndDate != "" {
					start, _ := time.Parse("2006-01-02", cfg.StartDate)
					end, _ := time.Parse("2006-01-02", cfg.EndDate)

					if start.After(end) {
						return errors.New("start date must be before end date")
					}
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--start-date", "2024-01-15", "--end-date", "2024-01-01"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "start date must be before end date")
	})

	t.Run("conditional validation", func(t *testing.T) {
		type ServerConfig struct {
			Environment string `goopt:"name:env;validators:isoneof(dev,test,prod)"`
			Debug       bool   `goopt:"name:debug"`
			LogLevel    string `goopt:"name:log-level;default:info"`
		}

		config := &ServerConfig{}
		parser, err := NewParserFromStruct(config,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*ServerConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				// Production-specific rules
				if cfg.Environment == "prod" {
					if cfg.Debug {
						return errors.New("debug mode not allowed in production")
					}

					if cfg.LogLevel == "debug" {
						return errors.New("debug log level not allowed in production")
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid: dev with debug
		success := parser.Parse([]string{"cmd", "--env", "dev", "--debug"})
		assert.True(t, success)

		// Invalid: prod with debug
		config2 := &ServerConfig{}
		parser2, _ := NewParserFromStruct(config2,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*ServerConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.Environment == "prod" && cfg.Debug {
					return errors.New("debug mode not allowed in production")
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--env", "prod", "--debug"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "debug mode not allowed in production")
	})

	t.Run("mutex flags validation", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("config-file", NewArg(WithType(types.Single))),
			WithFlag("config-url", NewArg(WithType(types.Single))),
			WithFlag("config-inline", NewArg(WithType(types.Single))),
			WithValidationHook(func(p *Parser) error {
				count := 0
				if _, has := p.Get("config-file"); has {
					count++
				}
				if _, has := p.Get("config-url"); has {
					count++
				}
				if _, has := p.Get("config-inline"); has {
					count++
				}

				if count > 1 {
					return errors.New("only one config source can be specified")
				}

				if count == 0 {
					return errors.New("at least one config source must be specified")
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid: one source
		success := parser.Parse([]string{"cmd", "--config-file", "config.yaml"})
		assert.True(t, success)

		// Invalid: multiple sources
		parser2, _ := NewParserWith(
			WithFlag("config-file", NewArg(WithType(types.Single))),
			WithFlag("config-url", NewArg(WithType(types.Single))),
			WithValidationHook(func(p *Parser) error {
				count := 0
				if _, has := p.Get("config-file"); has {
					count++
				}
				if _, has := p.Get("config-url"); has {
					count++
				}

				if count > 1 {
					return errors.New("only one config source can be specified")
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--config-file", "config.yaml", "--config-url", "http://example.com/config"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "only one config source can be specified")
	})

	t.Run("validation hook runs after field validation", func(t *testing.T) {
		validationHookCalled := false

		parser, err := NewParserWith(
			WithFlag("email", NewArg(
				WithType(types.Single),
				WithValidator(validation.Email()),
			)),
			WithValidationHook(func(p *Parser) error {
				validationHookCalled = true
				return nil
			}),
		)
		assert.NoError(t, err)

		// Invalid email - field validation should fail first
		success := parser.Parse([]string{"cmd", "--email", "not-an-email"})
		assert.False(t, success)
		assert.False(t, validationHookCalled, "validation hook should not be called when field validation fails")

		// Valid email - hook should be called
		validationHookCalled = false
		parser2, _ := NewParserWith(
			WithFlag("email", NewArg(
				WithType(types.Single),
				WithValidator(validation.Email()),
			)),
			WithValidationHook(func(p *Parser) error {
				validationHookCalled = true
				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--email", "test@example.com"})
		assert.True(t, success)
		assert.True(t, validationHookCalled, "validation hook should be called when field validation passes")
	})

	t.Run("mixed struct and dynamic flags", func(t *testing.T) {
		type BaseConfig struct {
			Mode string `goopt:"name:mode;validators:isoneof(dev,test,prod)"`
		}

		base := &BaseConfig{}
		parser, err := NewParserFromStruct(base,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*BaseConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				// Check dynamic flags based on mode
				if cfg.Mode == "prod" {
					apiKey, hasKey := p.Get("api-key")
					if !hasKey || apiKey == "" {
						return errors.New("--api-key required in production mode")
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Add dynamic flag
		err = parser.AddFlag("api-key", NewArg(
			WithDescription("API key for production"),
			WithType(types.Single),
		))
		assert.NoError(t, err)

		// Valid: dev mode without api-key
		success := parser.Parse([]string{"cmd", "--mode", "dev"})
		assert.True(t, success)

		// Invalid: prod mode without api-key
		base2 := &BaseConfig{}
		parser2, _ := NewParserFromStruct(base2,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*BaseConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.Mode == "prod" {
					apiKey, hasKey := p.Get("api-key")
					if !hasKey || apiKey == "" {
						return errors.New("--api-key required in production mode")
					}
				}

				return nil
			}),
		)
		_ = parser2.AddFlag("api-key", NewArg(WithType(types.Single)))

		success = parser2.Parse([]string{"cmd", "--mode", "prod"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "--api-key required in production mode")

		// Valid: prod mode with api-key
		base3 := &BaseConfig{}
		parser3, _ := NewParserFromStruct(base3,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*BaseConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.Mode == "prod" {
					apiKey, hasKey := p.Get("api-key")
					if !hasKey || apiKey == "" {
						return errors.New("--api-key required in production mode")
					}
				}

				return nil
			}),
		)
		_ = parser3.AddFlag("api-key", NewArg(WithType(types.Single)))

		success = parser3.Parse([]string{"cmd", "--mode", "prod", "--api-key", "secret123"})
		assert.True(t, success)
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
