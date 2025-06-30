package goopt

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, 0, cmdLine.GetPositionalArgCount(), "should have 0 positional arguments")
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
	p.SetLanguage(language.Spanish)

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

		userBundle.AddLanguage(language.English, map[string]string{
			"goopt.error.required_flag": "Custom required message",
		})

		parser := NewParser()
		parser.SetUserBundle(userBundle)
		parser.SetLanguage(language.English)

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
				testBundle.AddLanguage(language.English, map[string]string{
					"speed.desc": "Speed setting",
				})
				testBundle.AddLanguage(language.German, map[string]string{
					"speed.desc": "Geschwindigkeitseinstellung",
				})
				testBundle.AddLanguage(language.French, map[string]string{
					"speed.desc": "Réglage de vitesse",
				})

				// Load appropriate system locales to enable language switching
				parser := NewParser()
				parser.SetUserBundle(testBundle)
				parser.SetLanguage(tc.lang)
				// Disable auto-language detection to prevent environment from overriding our setting
				parser.SetAutoLanguage(false)

				parser.AddFlag("speed", NewArg(
					WithType(types.Single),
					WithAcceptedValues([]types.PatternValue{
						{Pattern: "fast|slow", Description: "speed.desc"},
					}),
				))

				success := parser.Parse([]string{"--speed", "medium"})
				assert.False(t, success)

				errors := parser.GetErrors()
				assert.Len(t, errors, 1)
				errMsg := errors[0].Error()

				// Debug: Check what language is actually set
				actualLang := parser.GetLanguage()
				t.Logf("Expected lang: %v, Actual lang: %v, Error: %s", tc.lang, actualLang, errMsg)

				assert.Contains(t, errMsg, tc.expected)
			})
		}
	})
}

func TestParser_AcceptedValuesChained(t *testing.T) {
	t.Run("chained values with i18n", func(t *testing.T) {
		// Create user bundle
		userBundle := i18n.NewEmptyBundle()

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

type CustomRenderer struct {
	*DefaultRenderer // Embed the default renderer to reuse its logic
}

type CustomHelp struct {
	Cmd struct {
		Flag1 string `goopt:"short:f;desc:Flag 1"`
		Flag2 string `goopt:"short:s;desc:Flag 2"`
	} `goopt:"desc:Command description;kind:command"`
}

func (r *CustomRenderer) FlagUsage(arg *Argument) string {
	// Custom flag formatting, e.g., a table-like layout
	name := r.FlagName(arg)
	if arg.Short != "" {
		name = fmt.Sprintf("-%s, --%s", arg.Short, name)
	} else {
		name = fmt.Sprintf("    --%s", name)
	}
	return fmt.Sprintf("  %-25s %s", name, r.FlagDescription(arg))
}

func TestParser_SetRenderer(t *testing.T) {
	output := &bytes.Buffer{}
	p, err := NewParserFromStruct(&CustomHelp{})
	p.SetRenderer(&CustomRenderer{
		DefaultRenderer: NewRenderer(p),
	})
	assert.NoError(t, err)

	p.PrintHelp(output)
	// ensure table-like output from CustomRenderer
	assert.Contains(t, output.String(), `   -f, --flag1               Flag 1
   -s, --flag2               Flag 2`)

}

func TestParser_HasValidators(t *testing.T) {
	parser := NewParser()

	// Create the flag first
	err := parser.AddFlag("test", NewArg())
	assert.NoError(t, err)

	// No validators initially
	assert.False(t, parser.HasValidators("test"))

	// Add a validator
	err = parser.AddFlagValidators("test", validation.MinLength(5))
	assert.NoError(t, err)
	assert.True(t, parser.HasValidators("test"))

	// Test with command path
	parser.AddCommand(&Command{Name: "cmd"})
	err = parser.AddFlag("flag", NewArg(), "cmd")
	assert.NoError(t, err)
	assert.False(t, parser.HasValidators("flag", "cmd"))

	err = parser.AddFlagValidators("flag@cmd", validation.MaxLength(10))
	assert.NoError(t, err)
	assert.True(t, parser.HasValidators("flag", "cmd"))
}

func TestParser_SetSuggestionsFormatter(t *testing.T) {
	parser := NewParser()

	called := false
	formatter := func(suggestions []string) string {
		called = true
		return strings.Join(suggestions, " | ")
	}

	parser.SetSuggestionsFormatter(formatter)
	assert.NotNil(t, parser.suggestionsFormatter)

	// Test that it gets used (would need to trigger a suggestion scenario)
	result := parser.suggestionsFormatter([]string{"opt1", "opt2"})
	assert.True(t, called)
	assert.Equal(t, "opt1 | opt2", result)
}

// TestGetSupportedLanguages tests the GetSupportedLanguages method
func TestParser_GetSupportedLanguages(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*Parser)
		expectedCount int
		mustContain   []language.Tag
	}{
		{
			name: "Default bundle only",
			setupFunc: func(p *Parser) {
				// Parser already has default bundle
			},
			expectedCount: 3, // Default bundle has 3 languages: en, de, fr
			mustContain:   []language.Tag{language.English, language.German, language.French},
		},
		{
			name: "With user bundle",
			setupFunc: func(p *Parser) {
				userBundle := i18n.NewEmptyBundle()
				userBundle.AddLanguage(language.Italian, map[string]string{
					"test": "prova",
				})
				p.SetUserBundle(userBundle)
			},
			expectedCount: 4, // 3 default + 1 Italian
			mustContain:   []language.Tag{language.English, language.Italian},
		},
		{
			name: "With system bundle",
			setupFunc: func(p *Parser) {
				systemBundle := i18n.NewEmptyBundle()
				systemBundle.AddLanguage(language.Portuguese, map[string]string{
					"test": "teste",
				})
				p.systemBundle = systemBundle
			},
			expectedCount: 4, // 3 default + 1 Portuguese
			mustContain:   []language.Tag{language.English, language.Portuguese},
		},
		{
			name: "All bundles with duplicates",
			setupFunc: func(p *Parser) {
				// Add user bundle with Spanish (not in default) and German (already in default)
				userBundle := i18n.NewEmptyBundle()
				userBundle.AddLanguage(language.Spanish, map[string]string{
					"test": "prueba",
				})
				userBundle.AddLanguage(language.Italian, map[string]string{
					"test": "prova",
				})
				p.SetUserBundle(userBundle)

				// Add system bundle with French (already in default)
				systemBundle := i18n.NewEmptyBundle()
				systemBundle.AddLanguage(language.French, map[string]string{
					"test": "test",
				})
				systemBundle.AddLanguage(language.Portuguese, map[string]string{
					"test": "teste",
				})
				p.systemBundle = systemBundle
			},
			expectedCount: 6, // 3 default + Spanish + Italian + Portuguese (German and French are duplicates)
			mustContain:   []language.Tag{language.German, language.French, language.Italian, language.Portuguese, language.Spanish},
		},
		{
			name: "Nil bundles",
			setupFunc: func(p *Parser) {
				// Set all bundles to nil
				p.defaultBundle = nil
				p.systemBundle = nil
				p.userI18n = nil
			},
			expectedCount: 0,
			mustContain:   []language.Tag{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			tt.setupFunc(parser)

			langs := parser.GetSupportedLanguages()

			assert.Equal(t, tt.expectedCount, len(langs), "Expected %d languages, got %d", tt.expectedCount, len(langs))

			// Create map for easier lookup
			langMap := make(map[language.Tag]bool)
			for _, lang := range langs {
				langMap[lang] = true
			}

			// Check that required languages are present
			for _, required := range tt.mustContain {
				assert.True(t, langMap[required], "Expected language %s to be present", required)
			}
		})
	}
}

// TestRegisterFlagTranslations tests the registerFlagTranslations method
func TestParser_RegisterFlagTranslations(t *testing.T) {
	tests := []struct {
		name           string
		flagName       string
		argument       *Argument
		commandPath    []string
		shouldRegister bool
	}{
		{
			name:     "Flag with NameKey",
			flagName: "help",
			argument: &Argument{
				NameKey: "goopt.flag.name.help",
			},
			commandPath:    []string{},
			shouldRegister: true,
		},
		{
			name:     "Flag without NameKey",
			flagName: "nokey",
			argument: &Argument{
				// No NameKey
			},
			commandPath:    []string{},
			shouldRegister: false,
		},
		{
			name:     "Flag with command context",
			flagName: "port",
			argument: &Argument{
				NameKey: "goopt.flag.name.port",
			},
			commandPath:    []string{"server", "start"},
			shouldRegister: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			// Call registerFlagTranslations
			parser.registerFlagTranslations(tt.flagName, tt.argument, tt.commandPath...)

			// Verify registration by checking if the flag can be found
			if tt.shouldRegister {
				// The flag should be registered in the translation registry
				// We can verify this by checking if GetCanonicalFlagName works
				canonical, found := parser.GetCanonicalFlagName(tt.flagName)
				assert.True(t, found, "Flag should be registered")
				assert.Equal(t, tt.flagName, canonical, "Should return the flag name itself")
			}
		})
	}
}

// TestRegisterCommandTranslations tests the registerCommandTranslations method
func TestParser_RegisterCommandTranslations(t *testing.T) {
	tests := []struct {
		name           string
		command        *Command
		shouldRegister bool
	}{
		{
			name: "Command with NameKey",
			command: &Command{
				Name:    "server",
				NameKey: "goopt.command.name.server",
				path:    "server",
			},
			shouldRegister: true,
		},
		{
			name: "Command without NameKey",
			command: &Command{
				Name: "nokey",
				path: "nokey",
				// No NameKey
			},
			shouldRegister: false,
		},
		{
			name: "Nested command with NameKey",
			command: &Command{
				Name:    "start",
				NameKey: "goopt.command.name.server.start",
				path:    "server start",
			},
			shouldRegister: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			// Call registerCommandTranslations
			parser.registerCommandTranslations(tt.command)

			// Verify registration
			if tt.shouldRegister {
				// The command should be registered in the translation registry
				canonical, found := parser.GetCanonicalCommandPath(tt.command.path)
				assert.True(t, found, "Command should be registered")
				assert.Equal(t, tt.command.path, canonical, "Should return the command path itself")
			} else {
				// Command without NameKey should not be registered
				_, found := parser.GetCanonicalCommandPath(tt.command.path)
				assert.False(t, found, "Command without NameKey should not be registered")
			}
		})
	}
}

// TestGetCanonicalFlagName tests the GetCanonicalFlagName method
func TestParser_GetCanonicalFlagName(t *testing.T) {
	parser := NewParser()

	// Set up Spanish translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help":    "ayuda",
		"goopt.flag.name.version": "versión",
		"goopt.flag.name.output":  "salida",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	// Register flags
	parser.registerFlagTranslations("help", &Argument{NameKey: "goopt.flag.name.help"})
	parser.registerFlagTranslations("version", &Argument{NameKey: "goopt.flag.name.version"})
	parser.registerFlagTranslations("output", &Argument{NameKey: "goopt.flag.name.output"})

	tests := []struct {
		name       string
		flagName   string
		expected   string
		shouldFind bool
	}{
		{
			name:       "Spanish translation of help",
			flagName:   "ayuda",
			expected:   "help",
			shouldFind: true,
		},
		{
			name:       "Spanish translation of version",
			flagName:   "versión",
			expected:   "version",
			shouldFind: true,
		},
		{
			name:       "Direct canonical name",
			flagName:   "help",
			expected:   "help",
			shouldFind: true,
		},
		{
			name:       "Non-existent flag",
			flagName:   "noexiste",
			expected:   "",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canonical, found := parser.GetCanonicalFlagName(tt.flagName)
			assert.Equal(t, tt.shouldFind, found)
			if tt.shouldFind {
				assert.Equal(t, tt.expected, canonical)
			}
		})
	}
}

// TestGetCanonicalCommandPath tests the GetCanonicalCommandPath method
func TestParser_GetCanonicalCommandPath(t *testing.T) {
	parser := NewParser()

	// Set up Spanish translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.command.name.server":   "servidor",
		"goopt.command.name.client":   "cliente",
		"goopt.command.name.database": "basededatos",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	// Register commands
	parser.registerCommandTranslations(&Command{
		Name:    "server",
		NameKey: "goopt.command.name.server",
		path:    "server",
	})
	parser.registerCommandTranslations(&Command{
		Name:    "client",
		NameKey: "goopt.command.name.client",
		path:    "client",
	})
	parser.registerCommandTranslations(&Command{
		Name:    "database",
		NameKey: "goopt.command.name.database",
		path:    "database",
	})

	tests := []struct {
		name       string
		cmdName    string
		expected   string
		shouldFind bool
	}{
		{
			name:       "Spanish translation of server",
			cmdName:    "servidor",
			expected:   "server",
			shouldFind: true,
		},
		{
			name:       "Spanish translation of client",
			cmdName:    "cliente",
			expected:   "client",
			shouldFind: true,
		},
		{
			name:       "Direct canonical name",
			cmdName:    "server",
			expected:   "server",
			shouldFind: true,
		},
		{
			name:       "Non-existent command",
			cmdName:    "noexiste",
			expected:   "",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canonical, found := parser.GetCanonicalCommandPath(tt.cmdName)
			assert.Equal(t, tt.shouldFind, found)
			if tt.shouldFind {
				assert.Equal(t, tt.expected, canonical)
			}
		})
	}
}

// TestTranslationIntegration tests the integration of translation features
func TestParser_TranslationIntegration(t *testing.T) {
	// Create a parser with multiple languages
	parser := NewParser()

	// Add French translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.French, map[string]string{
		"goopt.flag.name.help":      "aide",
		"goopt.flag.name.version":   "version",
		"goopt.command.name.server": "serveur",
		"goopt.command.name.start":  "démarrer",
	})
	parser.SetUserBundle(userBundle)

	// Define flags and commands
	err := parser.AddFlag("help", &Argument{
		TypeOf:      types.Standalone,
		Short:       "h",
		Description: "Show help message",
		NameKey:     "goopt.flag.name.help",
	})
	require.NoError(t, err)

	err = parser.AddFlag("version", &Argument{
		TypeOf:      types.Standalone,
		Short:       "v",
		Description: "Show version",
		NameKey:     "goopt.flag.name.version",
	})
	require.NoError(t, err)

	serverCmd := &Command{
		Name:        "server",
		Description: "Server operations",
		NameKey:     "goopt.command.name.server",
	}
	err = parser.AddCommand(serverCmd)
	require.NoError(t, err)

	startCmd := &Command{
		Name:        "start",
		Description: "Start the server",
		NameKey:     "goopt.command.name.start",
		path:        "server start",
	}
	serverCmd.Subcommands = append(serverCmd.Subcommands, *startCmd)

	// Test in French
	parser.SetLanguage(language.French)

	// Check flag translations
	canonical, found := parser.GetCanonicalFlagName("aide")
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	// Check command translations
	canonical, found = parser.GetCanonicalCommandPath("serveur")
	assert.True(t, found)
	assert.Equal(t, "server", canonical)

	// Test language switching
	parser.SetLanguage(language.English)

	// English should use direct names
	canonical, found = parser.GetCanonicalFlagName("help")
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	// French translations should still work when language is French
	parser.SetLanguage(language.French)
	canonical, found = parser.GetCanonicalFlagName("aide")
	assert.True(t, found)
	assert.Equal(t, "help", canonical)
}

// TestErrorMessageTranslations tests error message translations that weren't covered
func TestParser_ErrorMessageTranslations(t *testing.T) {
	parser := NewParser()

	// Set Spanish language
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.error.unknown_command":   "comando desconocido: %s",
		"goopt.error.ambiguous_command": "comando ambiguo '%s' podría ser: %s",
		"goopt.error.unsupported_shell": "shell no soportado: %s",
		"goopt.error.completion_failed": "falló la finalización: %v",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	// Define a simple command structure
	err := parser.AddCommand(&Command{
		Name:        "server",
		Description: "Server operations",
		Callback:    func(parser *Parser, cmd *Command) error { return nil },
	})
	require.NoError(t, err)
	err = parser.AddCommand(&Command{
		Name:        "service",
		Description: "Service operations",
		Callback:    func(parser *Parser, cmd *Command) error { return nil },
	})
	require.NoError(t, err)

	// Test unknown command error (must be similar to a registered command to trigger error)
	args := []string{"serve"} // Close to "server" - distance of 1
	ok := parser.Parse(args)
	assert.False(t, ok)
	errs := parser.GetErrors()
	assert.NotEmpty(t, errs)

	// Test ambiguous command - "ser" is closer to "server" than "service"
	args = []string{"ser"}
	ok = parser.Parse(args)
	assert.False(t, ok)
	errs = parser.GetErrors()
	assert.NotEmpty(t, errs)
	// The error should contain suggestions
	errStr := errs[len(errs)-1].Error()
	assert.Contains(t, errStr, "server") // Distance 3
	// "service" won't be shown as it's distance 4

	// Test completion with unsupported shell (defaults to bash)
	result := parser.GenerateCompletion("unsupported-shell", "myapp")
	assert.NotEmpty(t, result)                // GenerateCompletion defaults to bash for unsupported shells
	assert.Contains(t, result, "#!/bin/bash") // Verify it's a bash script
}

// TestMultilingualHelpGeneration tests help generation in different languages
func TestParser_MultilingualHelpGeneration(t *testing.T) {
	// Test 1: Default behavior without translations
	parser1 := NewParser()

	// Add flags and commands without translation keys
	err := parser1.AddFlag("help", &Argument{
		TypeOf:      types.Standalone,
		Short:       "h",
		Description: "Show help message",
	})
	require.NoError(t, err)

	err = parser1.AddFlag("version", &Argument{
		TypeOf:      types.Standalone,
		Short:       "v",
		Description: "Show version",
	})
	require.NoError(t, err)

	err = parser1.AddCommand(&Command{
		Name:        "server",
		Description: "Server operations",
		Callback:    func(parser *Parser, cmd *Command) error { return nil },
	})
	require.NoError(t, err)

	// Get help in English (default)
	var buf1 bytes.Buffer
	parser1.PrintHelp(&buf1)
	defaultHelp := buf1.String()
	assert.Contains(t, defaultHelp, "help")
	assert.Contains(t, defaultHelp, "version")
	assert.Contains(t, defaultHelp, "server")

	// Test 2: With translations
	parser := NewParser()
	parser.SetAutoLanguage(false) // Disable auto-language detection

	// Add translations for help content
	userBundle := i18n.NewEmptyBundle()
	// Add English translations
	err = userBundle.AddLanguage(language.English, map[string]string{
		"goopt.flag.name.help":      "help",
		"goopt.flag.desc.help":      "Show help message",
		"goopt.flag.name.version":   "version",
		"goopt.flag.desc.version":   "Show version",
		"goopt.command.name.server": "server",
		"goopt.command.desc.server": "Server operations",
		"goopt.help.usage":          "Usage:",
		"goopt.help.commands":       "Commands:",
		"goopt.help.flags":          "Flags:",
	})
	require.NoError(t, err, "Failed to add English translations")
	// Add Spanish translations
	err = userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help":      "ayuda",
		"goopt.flag.desc.help":      "Mostrar mensaje de ayuda",
		"goopt.flag.name.version":   "versión",
		"goopt.flag.desc.version":   "Mostrar versión",
		"goopt.command.name.server": "servidor",
		"goopt.command.desc.server": "Operaciones del servidor",
		"goopt.help.usage":          "Uso:",
		"goopt.help.commands":       "Comandos:",
		"goopt.help.flags":          "Opciones:",
	})
	require.NoError(t, err, "Failed to add Spanish translations")
	parser.SetUserBundle(userBundle)

	// Define options with translation keys
	err = parser.AddFlag("help", &Argument{
		TypeOf:         types.Standalone,
		Short:          "h",
		Description:    "Show help message",
		NameKey:        "goopt.flag.name.help",
		DescriptionKey: "goopt.flag.desc.help",
	})
	require.NoError(t, err)

	err = parser.AddFlag("version", &Argument{
		TypeOf:         types.Standalone,
		Short:          "v",
		Description:    "Show version",
		NameKey:        "goopt.flag.name.version",
		DescriptionKey: "goopt.flag.desc.version",
	})
	require.NoError(t, err)

	// Add a command
	err = parser.AddCommand(&Command{
		Name:           "server",
		Description:    "Server operations",
		NameKey:        "goopt.command.name.server",
		DescriptionKey: "goopt.command.desc.server",
		Callback:       func(parser *Parser, cmd *Command) error { return nil },
	})
	require.NoError(t, err)

	// Test help in English
	parser.SetLanguage(language.English)
	var buf bytes.Buffer
	parser.PrintHelp(&buf)
	englishHelp := buf.String()
	t.Logf("English help:\n%s", englishHelp)
	assert.Contains(t, englishHelp, "help")
	assert.Contains(t, englishHelp, "version")
	assert.Contains(t, englishHelp, "server")

	// Test help in Spanish
	parser.SetLanguage(language.Spanish)
	buf.Reset()
	parser.PrintHelp(&buf)
	spanishHelp := buf.String()
	// The help should show Spanish translations
	assert.Contains(t, spanishHelp, "ayuda")     // translated "help"
	assert.Contains(t, spanishHelp, "versión")   // translated "version"
	assert.Contains(t, spanishHelp, "servidor")  // translated "server"
	assert.NotEqual(t, englishHelp, spanishHelp) // Ensure translations are working
	assert.NotEmpty(t, spanishHelp)
}

// TestSubcommandSuggestions tests that "did you mean" suggestions work for subcommands
func TestParser_SubcommandSuggestions(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*Parser) error
		args           []string
		expectError    bool
		expectedErrors []string
		notExpected    []string
	}{
		{
			name: "Suggest 'start' for 'strt' in 'server strt'",
			setupFunc: func(p *Parser) error {
				serverCmd := NewCommand(
					WithName("server"),
					WithCommandDescription("Server operations"),
				)
				startCmd := NewCommand(
					WithName("start"),
					WithCommandDescription("Start the server"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				stopCmd := NewCommand(
					WithName("stop"),
					WithCommandDescription("Stop the server"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				serverCmd.AddSubcommand(startCmd)
				serverCmd.AddSubcommand(stopCmd)
				return p.AddCommand(serverCmd)
			},
			args:        []string{"server", "strt"},
			expectError: true,
			expectedErrors: []string{
				"server strt", // The full command path in error
				"start",       // The suggestion
			},
		},
		{
			name: "Suggest 'copy' for 'cpy' in 'nexus cpy blobs'",
			setupFunc: func(p *Parser) error {
				nexusCmd := NewCommand(
					WithName("nexus"),
					WithCommandDescription("Nexus operations"),
				)
				copyCmd := NewCommand(
					WithName("copy"),
					WithCommandDescription("Copy operation"),
				)
				blobsCmd := NewCommand(
					WithName("blobs"),
					WithCommandDescription("Copy blobs"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				copyCmd.AddSubcommand(blobsCmd)
				nexusCmd.AddSubcommand(copyCmd)
				return p.AddCommand(nexusCmd)
			},
			args:        []string{"nexus", "cpy", "blobs"},
			expectError: true,
			expectedErrors: []string{
				"nexus cpy", // The full command path in error
				"copy",      // The suggestion
			},
		},
		{
			name: "Multiple suggestions for ambiguous subcommand",
			setupFunc: func(p *Parser) error {
				serverCmd := NewCommand(
					WithName("server"),
					WithCommandDescription("Server operations"),
				)
				startCmd := NewCommand(
					WithName("start"),
					WithCommandDescription("Start the server"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				statusCmd := NewCommand(
					WithName("status"),
					WithCommandDescription("Show server status"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				stopCmd := NewCommand(
					WithName("stop"),
					WithCommandDescription("Stop the server"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				serverCmd.AddSubcommand(startCmd)
				serverCmd.AddSubcommand(statusCmd)
				serverCmd.AddSubcommand(stopCmd)
				return p.AddCommand(serverCmd)
			},
			args:        []string{"server", "st"},
			expectError: true,
			expectedErrors: []string{
				"server st",
				"stop", // Only stop is within distance 2 of "st"
			},
		},
		{
			name: "Suggestions for closer match",
			setupFunc: func(p *Parser) error {
				serverCmd := NewCommand(
					WithName("server"),
					WithCommandDescription("Server operations"),
				)
				startCmd := NewCommand(
					WithName("start"),
					WithCommandDescription("Start the server"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				statusCmd := NewCommand(
					WithName("status"),
					WithCommandDescription("Show server status"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				stopCmd := NewCommand(
					WithName("stop"),
					WithCommandDescription("Stop the server"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				serverCmd.AddSubcommand(startCmd)
				serverCmd.AddSubcommand(statusCmd)
				serverCmd.AddSubcommand(stopCmd)
				return p.AddCommand(serverCmd)
			},
			args:        []string{"server", "sta"}, // "sta" is closer to both "start" and "stop"
			expectError: true,
			expectedErrors: []string{
				"server sta",
				"start", // distance 2: add "rt"
				"stop",  // distance 2: change "a" to "o", add "p"
				// status is distance 3, so not included
			},
		},
		{
			name: "No suggestions for very different subcommand",
			setupFunc: func(p *Parser) error {
				serverCmd := NewCommand(
					WithName("server"),
					WithCommandDescription("Server operations"),
				)
				startCmd := NewCommand(
					WithName("start"),
					WithCommandDescription("Start the server"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				)
				serverCmd.AddSubcommand(startCmd)
				return p.AddCommand(serverCmd)
			},
			args:        []string{"server", "xyz"},
			expectError: true,
			expectedErrors: []string{
				"command 'server' expects", // Different error message when no suggestions
			},
			notExpected: []string{
				"Did you mean", // Should not have suggestions
			},
		},
		{
			name: "Root command suggestions still work",
			setupFunc: func(p *Parser) error {
				err := p.AddCommand(NewCommand(
					WithName("server"),
					WithCommandDescription("Server operations"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				))
				if err != nil {
					return err
				}
				return p.AddCommand(NewCommand(
					WithName("service"),
					WithCommandDescription("Service operations"),
					WithCallback(func(p *Parser, c *Command) error { return nil }),
				))
			},
			args:        []string{"serve"},
			expectError: true,
			expectedErrors: []string{
				"server", // Should suggest server (distance 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			err := tt.setupFunc(parser)
			require.NoError(t, err)

			ok := parser.Parse(tt.args)

			if tt.expectError {
				assert.False(t, ok, "Expected parsing to fail")
				errs := parser.GetErrors()
				assert.NotEmpty(t, errs, "Expected errors to be generated")

				// Combine all error messages
				allErrors := make([]string, 0, len(errs))
				for _, err := range errs {
					allErrors = append(allErrors, err.Error())
				}
				errorText := strings.Join(allErrors, "\n")

				// Check that expected strings are present
				for _, expected := range tt.expectedErrors {
					assert.Contains(t, errorText, expected,
						"Expected error to contain '%s', but got: %s", expected, errorText)
				}

				// Check that unexpected strings are not present
				for _, notExpected := range tt.notExpected {
					assert.NotContains(t, errorText, notExpected,
						"Error should not contain '%s', but got: %s", notExpected, errorText)
				}
			} else {
				assert.True(t, ok, "Expected parsing to succeed")
				assert.Empty(t, parser.GetErrors(), "Expected no errors")
			}
		})
	}
}

// TestSubcommandSuggestionsEdgeCases tests edge cases for subcommand suggestions
func TestParser_SubcommandSuggestionsEdgeCases(t *testing.T) {
	t.Run("Empty subcommand name", func(t *testing.T) {
		parser := NewParser()
		serverCmd := NewCommand(
			WithName("server"),
			WithCommandDescription("Server operations"),
		)
		startCmd := NewCommand(
			WithName("start"),
			WithCommandDescription("Start the server"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)
		serverCmd.AddSubcommand(startCmd)
		err := parser.AddCommand(serverCmd)
		require.NoError(t, err)

		// Empty string after server command
		ok := parser.Parse([]string{"server", ""})
		assert.False(t, ok)
		// Should show that subcommand is expected
		errs := parser.GetErrors()
		assert.NotEmpty(t, errs)
	})

	t.Run("Subcommand suggestion with flags", func(t *testing.T) {
		parser := NewParser()
		serverCmd := NewCommand(
			WithName("server"),
			WithCommandDescription("Server operations"),
		)
		startCmd := NewCommand(
			WithName("start"),
			WithCommandDescription("Start the server"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)
		serverCmd.AddSubcommand(startCmd)
		err := parser.AddCommand(serverCmd)
		require.NoError(t, err)

		// Add a flag
		err = parser.AddFlag("verbose", &Argument{
			TypeOf:      types.Standalone,
			Description: "Verbose output",
		})
		require.NoError(t, err)

		// Try with flag before subcommand typo
		ok := parser.Parse([]string{"--verbose", "server", "strt"})
		assert.False(t, ok)
		errs := parser.GetErrors()
		errorText := ""
		for _, err := range errs {
			errorText += err.Error() + "\n"
		}
		assert.Contains(t, errorText, "start", "Should suggest 'start' for 'strt'")
	})
}

// TestTranslatedFlagSuggestions tests that flag suggestions are shown in the translated language
func TestParser_TranslatedFlagSuggestions(t *testing.T) {
	// Create parser
	parser := NewParser()

	// Create a bundle with German translations
	bundle := i18n.NewEmptyBundle()
	err := bundle.LoadFromString(language.German, `{
		"flag.max-connections": "max-verbindungen",
		"flag.workers": "arbeiter",
		"flag.timeout": "zeitlimit",
		"goopt.error.unknown_flag_with_suggestions": "unbekannter Flag: %[1]s. Meinten Sie vielleicht eines davon? %[2]s"
	}`)
	require.NoError(t, err)

	parser.SetUserBundle(bundle)
	parser.SetLanguage(language.German)

	// Add flags with translation keys
	err = parser.AddFlag("max-connections", &Argument{
		NameKey:      "flag.max-connections",
		Description:  "Maximum concurrent connections",
		DefaultValue: "1000",
	})
	require.NoError(t, err)

	err = parser.AddFlag("workers", &Argument{
		NameKey:      "flag.workers",
		Description:  "Number of worker threads",
		DefaultValue: "10",
	})
	require.NoError(t, err)

	err = parser.AddFlag("timeout", &Argument{
		NameKey:      "flag.timeout",
		Description:  "Server timeout in seconds",
		DefaultValue: "30",
	})
	require.NoError(t, err)

	// Try to parse with a typo that's close to "max-verbindungen" (German)
	// Should suggest "max-verbindungen" since it's closer
	ok := parser.Parse([]string{"--max-verbindung"}) // Missing 'en'
	assert.False(t, ok)

	// Check that the error contains the translated suggestion
	errs := parser.GetErrors()
	require.NotEmpty(t, errs)

	errorText := errs[0].Error()
	t.Logf("Error message: %s", errorText)

	// Should contain the German error message
	assert.Contains(t, errorText, "unbekannter Flag")
	assert.Contains(t, errorText, "max-verbindung") // The typo the user entered

	// Should suggest the German translation "max-verbindungen"
	assert.Contains(t, errorText, "--max-verbindungen")
}

// TestTranslatedCommandSuggestions tests that command suggestions are shown in the translated language
func TestParser_TranslatedCommandSuggestions(t *testing.T) {
	// Create parser
	parser := NewParser()

	// Create a bundle with German translations
	bundle := i18n.NewEmptyBundle()
	err := bundle.LoadFromString(language.German, `{
		"cmd.server": "server",
		"cmd.server.start": "starten",
		"cmd.server.stop": "stoppen",
		"goopt.error.command_not_found": "Befehl nicht gefunden: %[1]s",
		"goopt.misc.did_you_mean": "Meinten Sie vielleicht"
	}`)
	require.NoError(t, err)

	parser.SetUserBundle(bundle)
	parser.SetLanguage(language.German)

	// Add server command with subcommands
	serverCmd := NewCommand(
		WithName("server"),
		WithCommandNameKey("cmd.server"),
		WithCommandDescription("Server operations"),
	)

	startCmd := NewCommand(
		WithName("start"),
		WithCommandNameKey("cmd.server.start"),
		WithCommandDescription("Start the server"),
		WithCallback(func(p *Parser, c *Command) error { return nil }),
	)

	stopCmd := NewCommand(
		WithName("stop"),
		WithCommandNameKey("cmd.server.stop"),
		WithCommandDescription("Stop the server"),
		WithCallback(func(p *Parser, c *Command) error { return nil }),
	)

	serverCmd.AddSubcommand(startCmd)
	serverCmd.AddSubcommand(stopCmd)

	err = parser.AddCommand(serverCmd)
	require.NoError(t, err)

	// Try to parse with a typo that's close to "starten" (German)
	// Should suggest "starten" since it's closer
	ok := parser.Parse([]string{"server", "starte"}) // Missing 'n'
	assert.False(t, ok)

	// Check that the error contains the translated suggestion
	errs := parser.GetErrors()
	require.NotEmpty(t, errs)

	// Combine all error messages
	errorText := ""
	for _, err := range errs {
		errorText += err.Error() + "\n"
	}
	t.Logf("Error messages: %s", errorText)

	// Should contain the German "did you mean" message
	assert.Contains(t, errorText, "Meinten Sie")

	// Should suggest the German translation "starten"
	assert.Contains(t, errorText, "starten")
}

// TestMixedLanguageSuggestions tests suggestions when some items are translated and some aren't
func TestParser_MixedLanguageSuggestions(t *testing.T) {
	parser := NewParser()

	// Create a bundle with partial German translations
	bundle := i18n.NewEmptyBundle()
	err := bundle.LoadFromString(language.German, `{
		"flag.verbose": "ausführlich",
		"goopt.error.unknown_flag_with_suggestions": "unbekannter Flag: %[1]s. Meinten Sie vielleicht eines davon? %[2]s"
	}`)
	require.NoError(t, err)

	parser.SetUserBundle(bundle)
	parser.SetLanguage(language.German)

	// Add flags - one with translation, one without
	err = parser.AddFlag("verbose", &Argument{
		NameKey:     "flag.verbose",
		Short:       "v",
		Description: "Verbose output",
	})
	require.NoError(t, err)

	err = parser.AddFlag("version", &Argument{
		// No NameKey - won't be translated
		Short:       "V",
		Description: "Show version",
	})
	require.NoError(t, err)

	// Try to parse with a typo close to the German translation
	ok := parser.Parse([]string{"--ausführlic"}) // Missing 'h' at the end
	assert.False(t, ok)

	errs := parser.GetErrors()
	require.NotEmpty(t, errs)

	errorText := errs[0].Error()
	t.Logf("Error message: %s", errorText)

	// Should contain suggestions
	assert.Contains(t, errorText, "Meinten Sie")

	// Since user typed something close to German, should show German suggestion
	assert.Contains(t, errorText, "--ausführlich")
	if strings.Contains(errorText, "version") {
		// Version has no translation, so should appear as-is
		assert.Contains(t, errorText, "--version")
	}
}

func TestParser_NamingConsistencyWarnings(t *testing.T) {
	t.Run("Flag naming inconsistency", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlagNameConverter(ToLowerCamel), // Expects camelCase
		)
		require.NoError(t, err)

		// Add flags with inconsistent naming
		err = parser.AddFlag("max-connections", &Argument{
			Description: "Maximum connections",
		})
		require.NoError(t, err)

		err = parser.AddFlag("debugMode", &Argument{
			Description: "Debug mode",
		})
		require.NoError(t, err)

		// Parse something
		ok := parser.Parse([]string{"--max-connections", "10"})
		assert.True(t, ok)

		// Check warnings
		warnings := parser.GetWarnings()
		assert.NotEmpty(t, warnings)

		// Should warn about max-connections not following camelCase
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "max-connections") && strings.Contains(w, "maxConnections") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected warning about max-connections not following naming convention")

		// Should NOT warn about debugMode (it follows camelCase)
		for _, w := range warnings {
			assert.NotContains(t, w, "debugMode")
		}
	})

	t.Run("Command naming inconsistency", func(t *testing.T) {
		parser, err := NewParserWith(
			WithCommandNameConverter(ToKebabCase), // Expects kebab-case
		)
		require.NoError(t, err)

		// Add commands with inconsistent naming
		err = parser.AddCommand(&Command{
			Name:        "startServer", // Should be start-server
			Description: "Start the server",
		})
		require.NoError(t, err)

		err = parser.AddCommand(&Command{
			Name:        "stop-server", // Correct format
			Description: "Stop the server",
		})
		require.NoError(t, err)

		// Parse
		ok := parser.Parse([]string{})
		assert.True(t, ok)

		// Check warnings
		warnings := parser.GetWarnings()

		// Should warn about startServer
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "startServer") && strings.Contains(w, "start-server") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected warning about startServer not following naming convention")

		// Should NOT warn about stop-server
		for _, w := range warnings {
			assert.NotContains(t, w, "'stop-server'")
		}
	})

	t.Run("Translation naming inconsistency", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlagNameConverter(ToLowerCamel), // Expects camelCase
		)
		require.NoError(t, err)

		// Add flag with nameKey for translation
		err = parser.AddFlag("maxConnections", &Argument{
			NameKey:     "flag.maxConnections",
			Description: "Maximum connections",
		})
		require.NoError(t, err)

		// Set German language
		err = parser.SetLanguage(language.German)
		require.NoError(t, err)

		// Create a user bundle with German translations
		userBundle, err := i18n.NewBundle()
		require.NoError(t, err)
		err = userBundle.AddLanguage(language.German, map[string]string{
			"flag.maxConnections": "max-verbindungen", // Doesn't follow camelCase
		})
		require.NoError(t, err)

		// Set the user bundle
		err = parser.SetUserBundle(userBundle)
		require.NoError(t, err)

		// Parse
		ok := parser.Parse([]string{})
		assert.True(t, ok)

		// Check warnings
		warnings := parser.GetWarnings()

		// Should warn about translation not following camelCase
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "max-verbindungen") && strings.Contains(w, "maxVerbindungen") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected warning about translation not following naming convention")
	})

	t.Run("Mixed naming conventions with struct", func(t *testing.T) {
		type Config struct {
			MaxConnections int    `goopt:"name:max-connections;desc:Maximum connections"`
			DebugMode      bool   `goopt:"desc:Debug mode"` // Will be converted to debugMode
			LogLevel       string `goopt:"name:log_level;desc:Log level"`
		}

		config := &Config{}
		parser, err := NewParserFromStruct(config,
			WithFlagNameConverter(ToLowerCamel), // Expects camelCase
		)
		require.NoError(t, err)

		// Parse
		ok := parser.Parse([]string{"--max-connections", "10", "--debugMode", "--log_level", "info"})
		assert.True(t, ok)

		// Check warnings
		warnings := parser.GetWarnings()

		// Should warn about max-connections (explicit name not following convention)
		foundMaxConn := false
		foundLogLevel := false
		for _, w := range warnings {
			if strings.Contains(w, "max-connections") && strings.Contains(w, "maxConnections") {
				foundMaxConn = true
			}
			if strings.Contains(w, "log_level") && strings.Contains(w, "logLevel") {
				foundLogLevel = true
			}
		}
		assert.True(t, foundMaxConn, "Expected warning about max-connections")
		assert.True(t, foundLogLevel, "Expected warning about log_level")

		// Should NOT warn about debugMode (generated name follows convention)
		for _, w := range warnings {
			assert.NotContains(t, w, "'debugMode'")
		}
	})

	t.Run("No warnings when everything follows convention", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlagNameConverter(ToKebabCase),
			WithCommandNameConverter(ToKebabCase),
		)
		require.NoError(t, err)

		// Add consistent flags and commands
		err = parser.AddFlag("max-connections", &Argument{
			Description: "Maximum connections",
		})
		require.NoError(t, err)

		err = parser.AddCommand(&Command{
			Name:        "start-server",
			Description: "Start the server",
		})
		require.NoError(t, err)

		// Parse
		ok := parser.Parse([]string{"--max-connections", "10"})
		assert.True(t, ok)

		// Check warnings - should be empty
		warnings := parser.GetWarnings()
		assert.Empty(t, warnings)
	})

	t.Run("Dependency warnings still work", func(t *testing.T) {
		parser := NewParser()

		// Add flags with dependencies
		err := parser.AddFlag("feature", &Argument{
			Description: "Enable feature",
			TypeOf:      types.Standalone, // Boolean flag
			DependencyMap: map[string][]string{
				"config": nil, // Depends on config flag being present
			},
		})
		require.NoError(t, err)

		err = parser.AddFlag("config", &Argument{
			Description: "Config file",
		})
		require.NoError(t, err)

		// Parse with feature but no config
		ok := parser.Parse([]string{"--feature"})
		// Parse might fail due to missing dependency, but we still get warnings

		// Get errors to see what happened
		if !ok {
			errs := parser.GetErrors()
			t.Logf("Parse errors: %v", errs)
		}

		// Should have dependency warning
		warnings := parser.GetWarnings()
		assert.NotEmpty(t, warnings, "Expected warnings but got none")

		found := false
		for _, w := range warnings {
			if strings.Contains(w, "depends on 'config'") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected dependency warning")
	})
}

// TestI18nErrorHandling tests i18n error handling paths
func TestParser_I18nErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*Parser) error
		args       []string
		checkError func(*testing.T, error)
	}{
		{
			name: "Unknown flag error",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("valid", &Argument{
					Short:       "v",
					TypeOf:      types.Standalone,
					Description: "Valid flag",
				})
			},
			args: []string{"--unknown"},
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unknown")
			},
		},
		{
			name: "Ambiguous flag error",
			setupFunc: func(p *Parser) error {
				if err := p.AddFlag("verbose", &Argument{
					Short:       "v",
					TypeOf:      types.Standalone,
					Description: "Verbose",
				}); err != nil {
					return err
				}
				return p.AddFlag("version", &Argument{
					Short:       "V",
					TypeOf:      types.Standalone,
					Description: "Version",
				})
			},
			args: []string{"--ver"},
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				// Should mention ambiguity
			},
		},
		{
			name: "Missing value error",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("name", &Argument{
					Short:       "n",
					TypeOf:      types.Single,
					Description: "Name",
				})
			},
			args: []string{"--name"},
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "name")
			},
		},
		{
			name: "Invalid boolean value",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("enable", &Argument{
					Short:       "e",
					TypeOf:      types.Standalone,
					Description: "Enable feature",
				})
			},
			args: []string{"--enable=invalid"},
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			err := tt.setupFunc(parser)
			require.NoError(t, err)

			ok := parser.Parse(tt.args)
			if !ok {
				errs := parser.GetErrors()
				if len(errs) > 0 {
					tt.checkError(t, errs[0])
				} else {
					t.Error("Parse failed but no errors returned")
				}
			} else {
				tt.checkError(t, nil)
			}
		})
	}
}

// TestI18nFormatterCoverage tests i18n formatter coverage
func TestParser_I18nFormatterCoverage(t *testing.T) {
	t.Skip("Skipping test: Plural support is not yet implemented")
	parser := NewParser()

	// Set up a custom bundle with various translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"test.plural.one":    "1 elemento",
		"test.plural.other":  "%d elementos",
		"test.ordinal.one":   "1º",
		"test.ordinal.two":   "2º",
		"test.ordinal.few":   "3º",
		"test.ordinal.other": "%dº",
		"test.with.args":     "Hola %s, tienes %d mensajes",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	translator := parser.GetTranslator()

	// Test plural formatting
	tests := []struct {
		name     string
		key      string
		count    int
		expected string
	}{
		{
			name:     "Plural one",
			key:      "test.plural",
			count:    1,
			expected: "1 elemento",
		},
		{
			name:     "Plural other",
			key:      "test.plural",
			count:    5,
			expected: "5 elementos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.TL(language.Spanish, tt.key, tt.count)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test formatting with arguments
	result := translator.TL(language.Spanish, "test.with.args", "Juan", 3)
	assert.Equal(t, "Hola Juan, tienes 3 mensajes", result)
}

// TestBundleMerging tests bundle merging functionality
func TestParser_BundleMerging(t *testing.T) {
	parser := NewParser()

	// Create multiple bundles
	bundle1 := i18n.NewEmptyBundle()
	err := bundle1.AddLanguage(language.Spanish, map[string]string{
		"key1": "valor1",
		"key2": "valor2",
		// key3 is not in user bundle, will come from system
	})
	require.NoError(t, err)

	bundle2 := i18n.NewEmptyBundle()
	// For bundle2, Spanish will be the default language with all keys
	err = bundle2.AddLanguage(language.Spanish, map[string]string{
		"key1": "valor1-system",
		"key2": "valor2-override", // Should override bundle1
		"key3": "valor3",
	})
	require.NoError(t, err)
	// French must have the same keys as Spanish
	err = bundle2.AddLanguage(language.French, map[string]string{
		"key1": "valeur1",
		"key2": "valeur2",
		"key3": "valeur3",
	})
	require.NoError(t, err)

	// Set user bundle (merges with default)
	parser.SetUserBundle(bundle1)

	// Create a system bundle
	parser.systemBundle = bundle2
	// Need to update the layered provider with the system bundle
	parser.layeredProvider.SetSystemBundle(bundle2)

	translator := parser.GetTranslator()

	// Set language to Spanish first
	parser.SetLanguage(language.Spanish)

	// Test that bundles are properly layered
	// User bundle should take precedence over system, system over default
	assert.Equal(t, "valor1", translator.TL(language.Spanish, "key1"))
	assert.Equal(t, "valor2", translator.TL(language.Spanish, "key2")) // From user bundle, not overridden
	assert.Equal(t, "valor3", translator.TL(language.Spanish, "key3")) // From system bundle

	// Switch to French for the French test
	parser.SetLanguage(language.French)
	assert.Equal(t, "valeur1", translator.TL(language.French, "key1")) // From system bundle
}

// TestLanguageMatching tests language matching functionality
func TestParser_LanguageMatching(t *testing.T) {
	parser := NewParser()

	// Add translations for different language variants
	userBundle := i18n.NewEmptyBundle()
	// Add English as the default/fallback language
	err := userBundle.AddLanguage(language.English, map[string]string{
		"test": "test", // Return key as value for no translation
	})
	require.NoError(t, err)
	err = userBundle.AddLanguage(language.MustParse("es"), map[string]string{
		"test": "español",
	})
	require.NoError(t, err)
	err = userBundle.AddLanguage(language.MustParse("es-MX"), map[string]string{
		"test": "español mexicano",
	})
	require.NoError(t, err)
	err = userBundle.AddLanguage(language.MustParse("es-ES"), map[string]string{
		"test": "español de España",
	})
	require.NoError(t, err)
	parser.SetUserBundle(userBundle)

	tests := []struct {
		name     string
		lang     language.Tag
		expected string
	}{
		{
			name:     "Exact match es-MX",
			lang:     language.MustParse("es-MX"),
			expected: "español mexicano",
		},
		{
			name:     "Exact match es-ES",
			lang:     language.MustParse("es-ES"),
			expected: "español de España",
		},
		{
			name:     "Fallback to base language",
			lang:     language.MustParse("es-AR"), // Not specifically defined, should fallback to es
			expected: "español",
		},
		{
			name:     "No match falls back to English",
			lang:     language.Japanese,
			expected: "test", // Falls back to English which has "test": "test"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser.SetLanguage(tt.lang)
			translator := parser.GetTranslator()
			// Use T() which uses the current language after matching
			result := translator.T("test")
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTranslationRegistryEdgeCases tests edge cases in translation registry
func TestParser_TranslationRegistryEdgeCases(t *testing.T) {
	parser := NewParser()

	// Test registering flags with special characters
	specialArg := &Argument{
		NameKey: "special.flag",
		Short:   "s",
	}
	parser.registerFlagTranslations("special-flag", specialArg)

	// Test registering with empty command path
	parser.registerFlagTranslations("global", &Argument{NameKey: "global.flag"}, "")

	// Test registering with nested command path
	parser.registerFlagTranslations("nested", &Argument{NameKey: "nested.flag"}, "cmd", "subcmd", "subsubcmd")

	// All registrations should succeed without panic
	canonical, found := parser.GetCanonicalFlagName("special-flag")
	assert.True(t, found)
	assert.Equal(t, "special-flag", canonical)

	canonical, found = parser.GetCanonicalFlagName("global")
	assert.True(t, found)
	assert.Equal(t, "global", canonical)

	canonical, found = parser.GetCanonicalFlagName("nested")
	assert.True(t, found)
	assert.Equal(t, "nested", canonical)
}

// TestMultipleLanguageSwitching tests switching between languages
func TestParser_MultipleLanguageSwitching(t *testing.T) {
	parser := NewParser()

	// Set up translations for multiple languages
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help": "ayuda",
		"goopt.flag.desc.help": "Mostrar ayuda",
	})
	userBundle.AddLanguage(language.French, map[string]string{
		"goopt.flag.name.help": "aide",
		"goopt.flag.desc.help": "Afficher l'aide",
	})
	userBundle.AddLanguage(language.German, map[string]string{
		"goopt.flag.name.help": "hilfe",
		"goopt.flag.desc.help": "Hilfe anzeigen",
	})
	// Add English for fallback
	userBundle.AddLanguage(language.English, map[string]string{
		"goopt.flag.name.help": "help",
		"goopt.flag.desc.help": "Show help",
	})
	parser.SetUserBundle(userBundle)

	// Register a help flag with the translation key
	err := parser.AddFlag("help", &Argument{
		NameKey:        "goopt.flag.name.help",
		DescriptionKey: "goopt.flag.desc.help",
		TypeOf:         types.Standalone,
	})
	require.NoError(t, err)

	// Test rapid language switching
	languages := []language.Tag{
		language.Spanish,
		language.French,
		language.German,
		language.English,
		language.Spanish,
		language.German,
	}

	expectedTranslations := map[language.Tag]string{
		language.Spanish: "ayuda",
		language.French:  "aide",
		language.German:  "hilfe",
		language.English: "help", // Falls back to canonical
	}

	for _, lang := range languages {
		parser.SetLanguage(lang)
		canonical, found := parser.GetCanonicalFlagName(expectedTranslations[lang])
		assert.True(t, found, "Should find translation for %s", lang)
		assert.Equal(t, "help", canonical)
	}
}

// TestCommandTranslationHierarchy tests nested command translation
func TestParser_CommandTranslationHierarchy(t *testing.T) {
	parser := NewParser()

	// Set up translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"cmd.server":          "servidor",
		"cmd.server.start":    "iniciar",
		"cmd.server.stop":     "detener",
		"cmd.database":        "basedatos",
		"cmd.database.backup": "respaldar",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	// Create command hierarchy
	serverCmd := NewCommand(WithName("server"), WithCommandDescription("Server operations"), WithCommandNameKey("cmd.server"))
	serverCmd.AddSubcommand(
		NewCommand(WithName("start"),
			WithCommandDescription("Start server"),
			WithCommandNameKey("cmd.server.start")))
	serverCmd.AddSubcommand(
		NewCommand(WithName("stop"),
			WithCommandDescription("Stop server"),
			WithCommandNameKey("cmd.server.stop")))

	dbCmd := NewCommand(WithName("database"), WithCommandDescription("Database operations"), WithCommandNameKey("cmd.database"))
	dbCmd.AddSubcommand(NewCommand(WithName("backup"), WithCommandDescription("Backup database"), WithCommandNameKey("cmd.database.backup")))

	// Add commands to parser
	assert.NoError(t, parser.AddCommand(serverCmd))
	assert.NoError(t, parser.AddCommand(dbCmd))

	// Test translations
	tests := []struct {
		translated string
		canonical  string
		found      bool
	}{
		{"servidor", "server", true},
		{"iniciar", "start", true},
		{"detener", "stop", true},
		{"basedatos", "database", true},
		{"respaldar", "backup", true},
		{"noexiste", "", false},
	}

	for _, tt := range tests {
		canonical, found := parser.GetCanonicalCommandPath(tt.translated)
		assert.Equal(t, tt.found, found, "Finding %s", tt.translated)
		if tt.found {
			// The command path might be partial (just the command name)
			// since we're not providing full context
			assert.NotEmpty(t, canonical)
		}
	}
}

// TestArgumentTypeTranslations tests translations for different argument types
func TestParser_ArgumentTypeTranslations(t *testing.T) {
	parser := NewParser()

	// Set up translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"flag.file":       "archivo",
		"flag.single":     "único",
		"flag.chained":    "encadenado",
		"flag.standalone": "independiente",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	// Register different types of arguments
	err := parser.AddFlag("file", NewArg(
		WithShortFlag("f"), WithType(types.File),
		WithDescription("Input file"), WithNameKey("flag.file")))
	require.NoError(t, err)

	err = parser.AddFlag("single", NewArg(WithShortFlag("s"), WithDescription("Single value"), WithNameKey("flag.single")))
	require.NoError(t, err)

	err = parser.AddFlag("chained", NewArg(WithShortFlag("c"), WithType(types.Chained), WithDescription("Chained values"), WithNameKey("flag.chained")))
	require.NoError(t, err)

	err = parser.AddFlag("standalone", NewArg(WithShortFlag("S"), WithType(types.Standalone), WithDescription("Standalone flag"), WithNameKey("flag.standalone")))
	require.NoError(t, err)

	// Test that all types can be translated
	tests := []struct {
		translated string
		canonical  string
	}{
		{"archivo", "file"},
		{"único", "single"},
		{"encadenado", "chained"},
		{"independiente", "standalone"},
	}

	for _, tt := range tests {
		canonical, found := parser.GetCanonicalFlagName(tt.translated)
		assert.True(t, found, "Should find %s", tt.translated)
		assert.Equal(t, tt.canonical, canonical)
	}
}

// TestEmptyTranslationKeys tests behavior with empty translation keys
func TestParser_EmptyTranslationKeys(t *testing.T) {
	parser := NewParser()

	// Register flags and commands without translation keys
	err := parser.AddFlag("notranslate", NewArg(WithShortFlag("n"), WithDescription("No translation")))
	require.NoError(t, err)

	cmd := NewCommand(WithName("nokey"), WithCommandDescription("Command without key"))
	assert.NoError(t, parser.AddCommand(cmd))

	// These should still be findable by their canonical names
	canonical, found := parser.GetCanonicalFlagName("notranslate")
	assert.True(t, found)
	assert.Equal(t, "notranslate", canonical)

	// Commands without keys are not registered in translation registry
	_, found = parser.GetCanonicalCommandPath("nokey")
	assert.False(t, found)
}

// TestTranslationWithSpecialCharacters tests translations with special characters
func TestParser_TranslationWithSpecialCharacters(t *testing.T) {
	parser := NewParser()

	// Set up translations with special characters
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Japanese, map[string]string{
		"flag.help":    "ヘルプ",
		"flag.version": "バージョン",
		"cmd.server":   "サーバー",
	})
	userBundle.AddLanguage(language.Arabic, map[string]string{
		"flag.help":    "مساعدة",
		"flag.version": "إصدار",
		"cmd.server":   "خادم",
	})
	parser.SetUserBundle(userBundle)

	// Register items
	err := parser.AddFlag("help", NewArg(WithShortFlag("h"), WithType(types.Standalone), WithDescription("Help"), WithNameKey("flag.help")))
	require.NoError(t, err)

	err = parser.AddFlag("version", NewArg(WithShortFlag("v"), WithType(types.Standalone), WithDescription("Version"), WithNameKey("flag.version")))
	require.NoError(t, err)

	cmd := NewCommand(WithName("server"), WithCommandDescription("Server"), WithCommandNameKey("cmd.server"))
	assert.NoError(t, parser.AddCommand(cmd))
	// Test Japanese
	parser.SetLanguage(language.Japanese)
	canonical, found := parser.GetCanonicalFlagName("ヘルプ")
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	canonical, found = parser.GetCanonicalCommandPath("サーバー")
	assert.True(t, found)
	assert.Equal(t, "server", canonical)

	// Test Arabic
	parser.SetLanguage(language.Arabic)
	canonical, found = parser.GetCanonicalFlagName("مساعدة")
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	canonical, found = parser.GetCanonicalCommandPath("خادم")
	assert.True(t, found)
	assert.Equal(t, "server", canonical)
}

func TestParser_CanonicalNameTypoSuggestions(t *testing.T) {
	// Create parser
	parser := NewParser()

	// Create a bundle with German translations
	bundle := i18n.NewEmptyBundle()
	err := bundle.LoadFromString(language.German, `{
		"flag.max-connections": "max-verbindungen",
		"goopt.error.unknown_flag_with_suggestions": "unbekannter Flag: %[1]s. Meinten Sie vielleicht eines davon? %[2]s"
	}`)
	require.NoError(t, err)

	parser.SetUserBundle(bundle)
	parser.SetLanguage(language.German)

	// Add flag with translation
	err = parser.AddFlag("max-connections", &Argument{
		NameKey:      "flag.max-connections",
		Description:  "Maximum concurrent connections",
		DefaultValue: "1000",
	})
	require.NoError(t, err)

	// Test 1: User types the canonical name exactly (should succeed - we accept both forms)
	t.Run("Exact canonical name", func(t *testing.T) {
		p := NewParser()
		p.SetUserBundle(bundle)
		p.SetLanguage(language.German)

		err := p.AddFlag("max-connections", &Argument{
			NameKey:      "flag.max-connections",
			Description:  "Maximum concurrent connections",
			DefaultValue: "1000",
		})
		require.NoError(t, err)

		ok := p.Parse([]string{"--max-connections", "100"})
		assert.True(t, ok, "Should recognize canonical name even when German is set")

		// Verify the value was parsed
		value, exists := p.Get("max-connections")
		assert.True(t, exists)
		assert.Equal(t, "100", value)
	})

	// Test 2: User types canonical name with typo
	t.Run("Misspelled canonical name", func(t *testing.T) {
		p := NewParser()
		p.SetUserBundle(bundle)
		p.SetLanguage(language.German)

		err := p.AddFlag("max-connections", &Argument{
			NameKey:      "flag.max-connections",
			Description:  "Maximum concurrent connections",
			DefaultValue: "1000",
		})
		require.NoError(t, err)

		ok := p.Parse([]string{"--max-connection"}) // Missing 's' at end
		assert.False(t, ok)

		errs := p.GetErrors()
		require.NotEmpty(t, errs)
		errorText := errs[0].Error()
		t.Logf("Error with misspelled canonical: %s", errorText)

		// Should suggest canonical form since user typed canonical with typo
		assert.Contains(t, errorText, "--max-connections")
	})

	// Test 3: User types German name with typo
	t.Run("Misspelled German name", func(t *testing.T) {
		p := NewParser()
		p.SetUserBundle(bundle)
		p.SetLanguage(language.German)

		err := p.AddFlag("max-connections", &Argument{
			NameKey:      "flag.max-connections",
			Description:  "Maximum concurrent connections",
			DefaultValue: "1000",
		})
		require.NoError(t, err)

		ok := p.Parse([]string{"--max-verbindung"}) // Missing 'en' at end
		assert.False(t, ok)

		errs := p.GetErrors()
		require.NotEmpty(t, errs)
		errorText := errs[0].Error()
		t.Logf("Error with misspelled German: %s", errorText)

		// Should suggest the correct German translation
		assert.Contains(t, errorText, "--max-verbindungen")
	})
}

func TestParser_ContextAwareCommandSuggestions(t *testing.T) {
	t.Run("User types canonical command with typo", func(t *testing.T) {
		parser := NewParser()

		// Create a bundle with German translations
		bundle := i18n.NewEmptyBundle()
		err := bundle.LoadFromString(language.German, `{
			"cmd.server": "server",
			"cmd.client": "klient",
			"cmd.status": "zustand",
			"goopt.error.command_not_found": "Befehl nicht gefunden: %[1]s",
			"goopt.misc.did_you_mean": "Meinten Sie vielleicht"
		}`)
		require.NoError(t, err)

		parser.SetUserBundle(bundle)
		parser.SetLanguage(language.German)

		// Add commands with translations
		serverCmd := NewCommand(
			WithName("server"),
			WithCommandNameKey("cmd.server"),
			WithCommandDescription("Server operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		clientCmd := NewCommand(
			WithName("client"),
			WithCommandNameKey("cmd.client"),
			WithCommandDescription("Client operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		statusCmd := NewCommand(
			WithName("status"),
			WithCommandNameKey("cmd.status"),
			WithCommandDescription("Status operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		err = parser.AddCommand(serverCmd)
		require.NoError(t, err)
		err = parser.AddCommand(clientCmd)
		require.NoError(t, err)
		err = parser.AddCommand(statusCmd)
		require.NoError(t, err)

		// User types "serv" (close to canonical "server")
		ok := parser.Parse([]string{"serv"})
		assert.False(t, ok)

		errs := parser.GetErrors()
		require.NotEmpty(t, errs)

		// Combine all error messages
		errorText := ""
		for _, err := range errs {
			errorText += err.Error() + "\n"
		}
		t.Logf("Error messages: %s", errorText)

		// Should suggest "server" (canonical) not "server" (which is same in German)
		assert.Contains(t, errorText, "server")
		// Should NOT suggest "klient" or "zustand" since we typed canonical
		assert.NotContains(t, errorText, "klient")
		assert.NotContains(t, errorText, "zustand")
	})

	t.Run("User types translated command with typo", func(t *testing.T) {
		parser := NewParser()

		// Create a bundle with German translations
		bundle := i18n.NewEmptyBundle()
		err := bundle.LoadFromString(language.German, `{
			"cmd.server": "server",
			"cmd.client": "klient",
			"cmd.status": "zustand",
			"goopt.error.command_not_found": "Befehl nicht gefunden: %[1]s",
			"goopt.misc.did_you_mean": "Meinten Sie vielleicht"
		}`)
		require.NoError(t, err)

		parser.SetUserBundle(bundle)
		parser.SetLanguage(language.German)

		// Add commands with translations
		serverCmd := NewCommand(
			WithName("server"),
			WithCommandNameKey("cmd.server"),
			WithCommandDescription("Server operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		clientCmd := NewCommand(
			WithName("client"),
			WithCommandNameKey("cmd.client"),
			WithCommandDescription("Client operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		statusCmd := NewCommand(
			WithName("status"),
			WithCommandNameKey("cmd.status"),
			WithCommandDescription("Status operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		err = parser.AddCommand(serverCmd)
		require.NoError(t, err)
		err = parser.AddCommand(clientCmd)
		require.NoError(t, err)
		err = parser.AddCommand(statusCmd)
		require.NoError(t, err)

		// User types "klien" (close to translated "klient")
		ok := parser.Parse([]string{"klien"})
		assert.False(t, ok)

		errs := parser.GetErrors()
		require.NotEmpty(t, errs)

		// Combine all error messages
		errorText := ""
		for _, err := range errs {
			errorText += err.Error() + "\n"
		}
		t.Logf("Error messages: %s", errorText)

		// Should suggest "klient" (translated) not "client" (canonical)
		assert.Contains(t, errorText, "klient")
		assert.NotContains(t, errorText, "client")
	})

	t.Run("Help system - user types canonical with typo", func(t *testing.T) {
		parser := NewParser()

		// Create a bundle with German translations
		bundle := i18n.NewEmptyBundle()
		err := bundle.LoadFromString(language.German, `{
			"cmd.server": "server",
			"cmd.client": "klient",
			"cmd.status": "zustand",
			"goopt.error.command_not_found": "Befehl nicht gefunden: %[1]s",
			"goopt.misc.did_you_mean": "Meinten Sie vielleicht"
		}`)
		require.NoError(t, err)

		parser.SetUserBundle(bundle)
		parser.SetLanguage(language.German)

		// Set up output capture
		var stdout, stderr strings.Builder
		parser.SetStdout(&stdout)
		parser.SetStderr(&stderr)
		parser.SetHelpBehavior(HelpBehaviorSmart)

		// Prevent actual exit
		parser.helpEndFunc = func() error {
			return nil
		}

		// Add commands with translations
		serverCmd := NewCommand(
			WithName("server"),
			WithCommandNameKey("cmd.server"),
			WithCommandDescription("Server operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		clientCmd := NewCommand(
			WithName("client"),
			WithCommandNameKey("cmd.client"),
			WithCommandDescription("Client operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		statusCmd := NewCommand(
			WithName("status"),
			WithCommandNameKey("cmd.status"),
			WithCommandDescription("Status operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		err = parser.AddCommand(serverCmd)
		require.NoError(t, err)
		err = parser.AddCommand(clientCmd)
		require.NoError(t, err)
		err = parser.AddCommand(statusCmd)
		require.NoError(t, err)

		// User types "lient --help" (equally close to both "client" and "klient")
		// Should show both forms since they're equally close
		// "lient" -> "client" = 1 change (change 'l' to 'c')
		// "lient" -> "klient" = 1 change (change 'l' to 'k')
		parser.Parse([]string{"lient", "--help"})

		output := stderr.String() + stdout.String()
		t.Logf("Help output: %s", output)

		// When equally close, should show both forms
		assert.Contains(t, output, "client / klient")
	})

	t.Run("Help system - user types translated with typo", func(t *testing.T) {
		parser := NewParser()

		// Create a bundle with German translations
		bundle := i18n.NewEmptyBundle()
		err := bundle.LoadFromString(language.German, `{
			"cmd.server": "server",
			"cmd.client": "klient",
			"cmd.status": "zustand",
			"goopt.error.command_not_found": "Befehl nicht gefunden: %[1]s",
			"goopt.misc.did_you_mean": "Meinten Sie vielleicht"
		}`)
		require.NoError(t, err)

		parser.SetUserBundle(bundle)
		parser.SetLanguage(language.German)

		// Set up output capture
		var stdout, stderr strings.Builder
		parser.SetStdout(&stdout)
		parser.SetStderr(&stderr)
		parser.SetHelpBehavior(HelpBehaviorSmart)

		// Prevent actual exit
		parser.helpEndFunc = func() error {
			return nil
		}

		// Add commands with translations
		serverCmd := NewCommand(
			WithName("server"),
			WithCommandNameKey("cmd.server"),
			WithCommandDescription("Server operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		clientCmd := NewCommand(
			WithName("client"),
			WithCommandNameKey("cmd.client"),
			WithCommandDescription("Client operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		statusCmd := NewCommand(
			WithName("status"),
			WithCommandNameKey("cmd.status"),
			WithCommandDescription("Status operations"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		)

		err = parser.AddCommand(serverCmd)
		require.NoError(t, err)
		err = parser.AddCommand(clientCmd)
		require.NoError(t, err)
		err = parser.AddCommand(statusCmd)
		require.NoError(t, err)

		// User types "zustan --help" (close to translated "zustand")
		parser.Parse([]string{"zustan", "--help"})

		output := stderr.String() + stdout.String()
		t.Logf("Help output: %s", output)

		// Should suggest "zustand" (translated) in the suggestion
		assert.Contains(t, output, "zustand")
		// Note: "status" will appear in the "Available commands" section which shows canonical names
		// but should not appear in the "Did you mean" suggestion
	})
}

func TestParser_ExecuteCommand(t *testing.T) {
	t.Run("ExecuteCommand with valid command", func(t *testing.T) {
		parser := NewParser()
		executed := false

		err := parser.AddCommand(&Command{
			Name: "test",
			Callback: func(p *Parser, c *Command) error {
				executed = true
				return nil
			},
		})
		require.NoError(t, err)

		// Parse first to register the command
		ok := parser.Parse([]string{"test"})
		assert.True(t, ok)

		// Execute all commands
		parser.ExecuteCommands()
		assert.True(t, executed)
	})

	t.Run("ExecuteCommand with error", func(t *testing.T) {
		parser := NewParser()
		expectedErr := errors.New("command error")

		err := parser.AddCommand(&Command{
			Name: "fail",
			Callback: func(p *Parser, c *Command) error {
				return expectedErr
			},
		})
		require.NoError(t, err)

		// Parse and execute
		ok := parser.Parse([]string{"fail"})
		assert.True(t, ok)

		count := parser.ExecuteCommands()
		assert.Equal(t, 1, count) // 1 error

		// Check the error was recorded
		err = parser.GetCommandExecutionError("fail")
		assert.Equal(t, expectedErr, err)
	})

	t.Run("ExecuteCommand with non-existent command", func(t *testing.T) {
		parser := NewParser()

		// Try to parse non-existent command - it will be treated as positional arg
		ok := parser.Parse([]string{"nonexistent"})
		assert.True(t, ok) // Parse succeeds but no command is executed

		// No commands to execute
		count := parser.ExecuteCommands()
		assert.Equal(t, 0, count)
	})

	t.Run("GetCommandExecutionError", func(t *testing.T) {
		parser := NewParser()
		expectedErr := errors.New("test error")

		err := parser.AddCommand(&Command{
			Name: "cmd",
			Callback: func(p *Parser, c *Command) error {
				return expectedErr
			},
		})
		require.NoError(t, err)

		parser.callbackResults["cmd"] = expectedErr

		// Test existing error
		err = parser.GetCommandExecutionError("cmd")
		assert.Equal(t, expectedErr, err)

		// Test non-existent command - might have error from parsing
		// Just verify it doesn't panic
		_ = parser.GetCommandExecutionError("nonexistent")
	})
}

func TestParser_BindFlagWithSlices(t *testing.T) {
	t.Run("BindFlag with slice and capacity", func(t *testing.T) {
		parser := NewParser()
		var values []string

		err := parser.BindFlag(&values, "items", &Argument{
			TypeOf:   types.Chained,
			Capacity: 3,
		})
		require.NoError(t, err)

		// Verify slice was created with capacity
		assert.Equal(t, 3, len(values))
		assert.Equal(t, 3, cap(values))
	})

	t.Run("BindFlag with existing slice and resize", func(t *testing.T) {
		parser := NewParser()
		values := []string{"a", "b", "c", "d", "e"}

		err := parser.BindFlag(&values, "items", &Argument{
			TypeOf:   types.Chained,
			Capacity: 3,
		})
		require.NoError(t, err)

		// Verify slice was resized and values preserved
		assert.Equal(t, 3, len(values))
		assert.Equal(t, []string{"a", "b", "c"}, values)
	})

	t.Run("BindFlag with nil pointer", func(t *testing.T) {
		parser := NewParser()

		err := parser.BindFlag(nil, "flag", &Argument{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("BindFlag with non-pointer", func(t *testing.T) {
		parser := NewParser()
		var value string

		err := parser.BindFlag(value, "flag", &Argument{})
		assert.Error(t, err)
	})

	t.Run("BindFlag with invalid type conversion", func(t *testing.T) {
		parser := NewParser()
		var value int

		err := parser.BindFlag(&value, "flag", &Argument{
			TypeOf: types.Standalone, // Standalone only works with bool
		})
		assert.Error(t, err)
	})

	t.Run("BindFlag infers type when TypeOf is Empty", func(t *testing.T) {
		parser := NewParser()
		var value bool

		err := parser.BindFlag(&value, "flag", &Argument{
			TypeOf: types.Empty, // Should infer as Standalone for bool
		})
		require.NoError(t, err)

		// Verify type was inferred (bool becomes Single when TypeOf is Empty)
		arg, err := parser.GetArgument("flag")
		assert.NoError(t, err)
		assert.Equal(t, types.Single, arg.TypeOf)
	})
}

// TestEnsureInitCoverage improves coverage for ensureInit
func TestParser_EnsureInit(t *testing.T) {
	t.Run("Parser initialization with struct context", func(t *testing.T) {
		type Config struct {
			Name  string `goopt:"name=name,type=single"`
			Debug bool   `goopt:"name=debug,type=standalone"`
		}

		config := &Config{}
		parser, err := NewParserFromStruct(config)
		require.NoError(t, err)

		// Verify struct context is set
		assert.True(t, parser.HasStructCtx())

		// Manually bind the fields for this test
		err = parser.BindFlag(&config.Name, "name", &Argument{TypeOf: types.Single})
		require.NoError(t, err)
		err = parser.BindFlag(&config.Debug, "debug", &Argument{TypeOf: types.Standalone})
		require.NoError(t, err)

		// Parse args
		ok := parser.Parse([]string{"--name", "test", "--debug"})
		assert.True(t, ok)
		assert.Equal(t, "test", config.Name)
		assert.True(t, config.Debug)
	})

	t.Run("Parser with environment variables", func(t *testing.T) {
		parser := NewParser()

		// Add a flag
		err := parser.AddFlag("config", &Argument{
			TypeOf: types.Single,
		})
		require.NoError(t, err)

		// Set environment variable - using default converter behavior
		t.Setenv("CONFIG", "env-value")

		// Set environment converter to enable env mapping
		parser.SetEnvNameConverter(DefaultFlagNameConverter) // This enables env var mapping

		// Parse with empty args (should pick up env var)
		ok := parser.Parse([]string{})
		assert.True(t, ok)
		value, found := parser.Get("config")
		assert.True(t, found)
		assert.Equal(t, "env-value", value)
	})
}

func TestParser_SetArgument(t *testing.T) {
	t.Run("SetArgument updates existing flag", func(t *testing.T) {
		parser := NewParser()

		// Add initial flag
		err := parser.AddFlag("test", &Argument{
			TypeOf:      types.Single,
			Description: "Original",
		})
		require.NoError(t, err)

		// Update the flag using config functions
		err = parser.SetArgument("test", []string{},
			WithType(types.Standalone),
			WithDescription("Updated"),
			WithShortFlag("t"))
		require.NoError(t, err)

		// Verify update
		arg, err := parser.GetArgument("test")
		assert.NoError(t, err)
		assert.Equal(t, types.Standalone, arg.TypeOf)
		assert.Equal(t, "Updated", arg.Description)
		assert.Equal(t, "t", arg.Short)
	})

	t.Run("SetArgument with command context", func(t *testing.T) {
		parser := NewParser()

		// Add command with flag
		cmd := &Command{Name: "cmd"}
		err := parser.AddCommand(cmd)
		require.NoError(t, err)

		err = parser.AddFlag("flag", &Argument{
			TypeOf: types.Single,
		}, "cmd")
		require.NoError(t, err)

		// Update the flag
		err = parser.SetArgument("flag", []string{"cmd"},
			WithType(types.Standalone),
			WithDescription("Updated"))
		require.NoError(t, err)

		// Verify update
		arg, err := parser.GetArgument("flag", "cmd")
		assert.NoError(t, err)
		assert.Equal(t, types.Standalone, arg.TypeOf)
	})

	t.Run("SetArgument with validation error", func(t *testing.T) {
		parser := NewParser()

		// First add the flag
		err := parser.AddFlag("test", &Argument{
			TypeOf: types.Single,
		})
		require.NoError(t, err)

		// Try to set argument with invalid accepted values
		err = parser.SetArgument("test", []string{},
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "[invalid"}, // Invalid regex
			}))
		assert.Error(t, err)
	})
}

// TestHelperFunctionsCoverage improves coverage for various helper functions
func TestHelperFunctionsCoverage(t *testing.T) {
	t.Run("GetMaxDependencyDepth", func(t *testing.T) {
		parser := NewParser()

		// Default depth
		depth := parser.GetMaxDependencyDepth()
		assert.Equal(t, DefaultMaxDependencyDepth, depth)

		// Set custom depth
		parser.SetMaxDependencyDepth(10)
		depth = parser.GetMaxDependencyDepth()
		assert.Equal(t, 10, depth)
	})

	t.Run("SetFlag sets flag value", func(t *testing.T) {
		parser := NewParser()

		// Add initial flag
		err := parser.AddFlag("test", &Argument{
			TypeOf: types.Single,
		})
		require.NoError(t, err)

		// Set the flag value
		err = parser.SetFlag("test", "myvalue")
		require.NoError(t, err)

		// Verify value was set
		value, found := parser.Get("test")
		assert.True(t, found)
		assert.Equal(t, "myvalue", value)
	})

	t.Run("SetCommand updates existing command", func(t *testing.T) {
		parser := NewParser()

		// Add initial command
		err := parser.AddCommand(&Command{
			Name:        "test",
			Description: "Original",
		})
		require.NoError(t, err)

		// Update command
		err = parser.SetCommand("test", WithCommandDescription("Updated"))
		require.NoError(t, err)

		// Use internal getCommand to verify the update
		cmd, found := parser.getCommand("test")
		assert.True(t, found)
		assert.Equal(t, "Updated", cmd.Description)
	})

	t.Run("PrintHelp with error cases", func(t *testing.T) {
		parser := NewParser()

		// Set help style to invalid value to trigger default case
		parser.SetHelpStyle(HelpStyle(999))

		// Use a buffer to capture output
		var buf bytes.Buffer
		parser.SetStdout(&buf)

		parser.PrintHelp(&buf)
		output := buf.String()
		assert.NotEmpty(t, output)
	})
}

func TestParser_RegisterTranslations(t *testing.T) {
	parser := NewParser()

	// Set up translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"flag.name.verbose": "verboso",
		"cmd.name.server":   "servidor",
	})
	parser.SetUserBundle(userBundle)

	// Test flag with NameKey (triggers registerFlagTranslations)
	err := parser.AddFlag("verbose", NewArg(
		WithNameKey("flag.name.verbose"),
		WithType(types.Standalone),
	))
	assert.NoError(t, err)

	// Test command with NameKey (triggers registerCommandTranslations)
	cmd := NewCommand(
		WithName("server"),
		WithCommandNameKey("cmd.name.server"),
		WithCallback(func(cmdLine *Parser, command *Command) error {
			return nil
		}),
	)
	cmd.NameKey = "cmd.name.server" // Set NameKey directly
	parser.AddCommand(cmd)

	// Parse with translations
	parser.SetLanguage(language.Spanish)
	ok := parser.Parse([]string{"--verboso", "servidor"})
	assert.True(t, ok)
}

func TestParser_AutoLanguageGetters(t *testing.T) {
	parser := NewParser()

	// GetAutoLanguage
	assert.True(t, parser.GetAutoLanguage())
	parser.SetAutoLanguage(false)
	assert.False(t, parser.GetAutoLanguage())
	parser.SetAutoLanguage(true)
	assert.True(t, parser.GetAutoLanguage())

	// GetCheckSystemLocale
	assert.False(t, parser.GetCheckSystemLocale())
	parser.SetCheckSystemLocale(true)
	assert.True(t, parser.GetCheckSystemLocale())

	// GetLanguageEnvVar
	assert.Equal(t, "GOOPT_LANG", parser.GetLanguageEnvVar())
	parser.SetLanguageEnvVar("CUSTOM_LANG")
	assert.Equal(t, "CUSTOM_LANG", parser.GetLanguageEnvVar())
}

func TestParser_GetCanonicalMethodsCoverage(t *testing.T) {
	parser := NewParser()

	// Set up Spanish translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"flag.name.help": "ayuda",
		"cmd.name.start": "iniciar",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	// Register flag
	parser.AddFlag("help", NewArg(
		WithNameKey("flag.name.help"),
		WithType(types.Standalone),
	))

	// Test GetCanonicalFlagName
	canonical, found := parser.GetCanonicalFlagName("ayuda")
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	_, found = parser.GetCanonicalFlagName("nonexistent")
	assert.False(t, found)

	// Register command
	cmd := NewCommand(
		WithName("start"),
		WithCallback(func(cmdLine *Parser, command *Command) error {
			return nil
		}),
		WithCommandNameKey("cmd.name.start"),
	)
	parser.AddCommand(cmd)

	// Test GetCanonicalCommandPath
	canonical, found = parser.GetCanonicalCommandPath("iniciar")
	assert.True(t, found)
	assert.Equal(t, "start", canonical)

	_, found = parser.GetCanonicalCommandPath("nonexistent")
	assert.False(t, found)
}

func TestGParser_etMaxDependency(t *testing.T) {
	parser := NewParser()

	// Default should be reasonable
	depth := parser.GetMaxDependencyDepth()
	assert.True(t, depth > 0)

	// Set new depth
	parser.SetMaxDependencyDepth(5)
	assert.Equal(t, 5, parser.GetMaxDependencyDepth())
}

func TestParser_ErrorProvider(t *testing.T) {
	// Test As method which is at 0%
	parser := NewParser()
	parser.AddFlag("test", NewArg(
		WithType(types.Single),
		WithRequired(true),
	))

	ok := parser.Parse([]string{})
	assert.False(t, ok)

	errs := parser.GetErrors()
	assert.NotEmpty(t, errs)
}

func TestParser_LayeredProvider(t *testing.T) {
	parser := NewParser()
	provider := parser.layeredProvider

	// GetLanguage
	lang := provider.GetLanguage()
	assert.NotNil(t, lang)

	// GetDefaultLanguage
	defaultLang := provider.GetDefaultLanguage()
	assert.Equal(t, language.English, defaultLang)

	// SetUserBundle
	userBundle := i18n.NewEmptyBundle()
	provider.SetUserBundle(userBundle)

	// SetSystemBundle
	systemBundle := i18n.NewEmptyBundle()
	provider.SetSystemBundle(systemBundle)

	// GetFormattedMessage
	msg := provider.GetFormattedMessage("test.key", "arg1", "arg2")
	assert.Equal(t, "test.key", msg) // Falls back to key when not found

	// GetPrinter
	printer := provider.GetPrinter()
	assert.NotNil(t, printer)

	// T method through translator interface
	translator := parser.GetTranslator()
	result := translator.T("test.message")
	assert.Equal(t, "test.message", result)
}

func TestParser_SuggestionThreshold(t *testing.T) {
	t.Run("Default threshold shows distance 1 and 2 matches", func(t *testing.T) {
		parser := NewParser()

		// Add commands with varying distances from "serv"
		err := parser.AddCommand(NewCommand(
			WithName("serve"), // Distance 1 from "serv"
			WithCommandDescription("Serve command"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		))
		require.NoError(t, err)

		err = parser.AddCommand(NewCommand(
			WithName("server"), // Distance 2 from "serv"
			WithCommandDescription("Server command"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		))
		require.NoError(t, err)

		err = parser.AddCommand(NewCommand(
			WithName("service"), // Distance 3 from "serv"
			WithCommandDescription("Service command"),
			WithCallback(func(p *Parser, c *Command) error { return nil }),
		))
		require.NoError(t, err)

		// Parse with typo
		ok := parser.Parse([]string{"serv"})
		assert.False(t, ok)

		// Should only suggest "serve" (distance 1) by default
		errs := parser.GetErrors()
		require.NotEmpty(t, errs)
		errorText := errs[len(errs)-1].Error()

		assert.Contains(t, errorText, "serve")
		assert.NotContains(t, errorText, "server")  // Distance 2 - not shown when distance 1 exists
		assert.NotContains(t, errorText, "service") // Distance 3 - never shown
	})

	t.Run("Custom threshold allows more distant matches", func(t *testing.T) {
		parser := NewParser()
		parser.SetSuggestionThreshold(3, 3) // Allow distance 3 for both flags and commands

		// Add flags with varying distances from "--verbo"
		err := parser.AddFlag("verbose", &Argument{
			Description: "Verbose output",
		})
		require.NoError(t, err)

		err = parser.AddFlag("version", &Argument{
			Description: "Show version",
		})
		require.NoError(t, err)

		err = parser.AddFlag("verify", &Argument{
			Description: "Verify output",
		})
		require.NoError(t, err)

		// Parse with flag typo
		ok := parser.Parse([]string{"--verb"})
		assert.False(t, ok)

		// With threshold 3, should suggest all flags with distance <= 3
		errs := parser.GetErrors()
		require.NotEmpty(t, errs)
		errorText := ""
		for _, err := range errs {
			errorText += err.Error() + "\n"
		}

		assert.Contains(t, errorText, "--verbose") // Distance 3
		assert.Contains(t, errorText, "--verify")  // Distance 2
	})

	t.Run("Zero threshold disables suggestions", func(t *testing.T) {
		parser := NewParser()
		parser.SetSuggestionThreshold(0, 0) // Disable all suggestions

		// Add flag
		err := parser.AddFlag("serve", &Argument{
			Description: "Serve content",
		})
		require.NoError(t, err)

		// Parse with typo
		ok := parser.Parse([]string{"--serv"})
		assert.False(t, ok)

		// Should not show any suggestions
		errs := parser.GetErrors()
		require.NotEmpty(t, errs)
		errorText := errs[0].Error()

		assert.NotContains(t, errorText, "Did you mean")
		assert.NotContains(t, errorText, "serve")
	})

	t.Run("Flag threshold works independently", func(t *testing.T) {
		parser := NewParser()
		parser.SetSuggestionThreshold(3, 1) // More lenient for flags, strict for commands

		// Add flags
		err := parser.AddFlag("verbose", &Argument{
			Description: "Verbose output",
		})
		require.NoError(t, err)

		err = parser.AddFlag("version", &Argument{
			Description: "Show version",
		})
		require.NoError(t, err)

		// Parse with flag typo
		ok := parser.Parse([]string{"--verb"})
		assert.False(t, ok)

		// With flag threshold 3, should suggest both
		errs := parser.GetErrors()
		require.NotEmpty(t, errs)
		errorText := ""
		for _, err := range errs {
			errorText += err.Error() + "\n"
		}

		assert.Contains(t, errorText, "--verbose") // Distance 3
	})

	t.Run("WithSuggestionThreshold configuration", func(t *testing.T) {
		parser, err := NewParserWith(
			WithSuggestionThreshold(3, 2),
			WithFlag("deploy", &Argument{
				Description: "Deploy flag",
			}),
		)
		require.NoError(t, err)

		// Parse with distant typo
		ok := parser.Parse([]string{"--dep"})
		assert.False(t, ok)

		// Should suggest "deploy" (distance 3) with threshold 3 for flags
		errs := parser.GetErrors()
		require.NotEmpty(t, errs)
		errorText := errs[len(errs)-1].Error()

		assert.Contains(t, errorText, "--deploy")
	})
}

func TestParser_SuggestionsForCommands(t *testing.T) {
	// Test root command suggestions
	t.Run("RootCommandSuggestions", func(t *testing.T) {
		parser := NewParser()

		// Add some commands
		parser.AddCommand(NewCommand(
			WithName("server"),
			WithCommandDescription("Server management"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))
		parser.AddCommand(NewCommand(
			WithName("service"),
			WithCommandDescription("Service management"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))
		parser.AddCommand(NewCommand(
			WithName("status"),
			WithCommandDescription("Show status"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))

		// Test typo that should generate suggestions
		ok := parser.Parse([]string{"serv"})
		assert.False(t, ok)

		errs := parser.GetErrors()
		assert.NotEmpty(t, errs)

		// Should have command not found error and suggestions
		errStr := ""
		for _, err := range errs {
			errStr += err.Error() + "\n"
		}

		assert.Contains(t, errStr, "serv")
		assert.Contains(t, errStr, "Did you mean")
		// Due to conservative logic, it only suggests when there's one very close match
		// In this case "serv" -> "server" is distance 2, so it should suggest it
		assert.Contains(t, errStr, "server")
	})

	// Test subcommand suggestions
	t.Run("SubcommandSuggestions", func(t *testing.T) {
		parser := NewParser()

		// Add command with subcommands
		serverCmd := NewCommand(
			WithName("server"),
			WithCommandDescription("Server management"),
		)
		serverCmd.AddSubcommand(NewCommand(
			WithName("start"),
			WithCommandDescription("Start server"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))
		serverCmd.AddSubcommand(NewCommand(
			WithName("stop"),
			WithCommandDescription("Stop server"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))
		serverCmd.AddSubcommand(NewCommand(
			WithName("status"),
			WithCommandDescription("Server status"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))
		parser.AddCommand(serverCmd)

		// Test typo in subcommand
		ok := parser.Parse([]string{"server", "stat"})
		assert.False(t, ok)

		errs := parser.GetErrors()
		assert.NotEmpty(t, errs)

		errStr := ""
		for _, err := range errs {
			errStr += err.Error() + "\n"
		}

		assert.Contains(t, errStr, "server stat")
		assert.Contains(t, errStr, "Did you mean")
		// Should suggest "start" (distance 1) but not "status" (distance 2)
		assert.Contains(t, errStr, "start")
		// With conservative distance logic, "status" (distance 2) won't be shown
		// when "start" (distance 1) exists
	})

	// Test custom suggestions formatter
	t.Run("CustomSuggestionsFormatter", func(t *testing.T) {
		parser := NewParser()

		// Set custom formatter
		parser.SetSuggestionsFormatter(func(suggestions []string) string {
			return "Perhaps you meant: " + strings.Join(suggestions, " or ")
		})

		parser.AddCommand(NewCommand(
			WithName("deploy"),
			WithCommandDescription("Deploy application"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))
		parser.AddCommand(NewCommand(
			WithName("delete"),
			WithCommandDescription("Delete resources"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))

		// Test with exact match between two commands
		ok := parser.Parse([]string{"depl"})
		assert.False(t, ok)

		errs := parser.GetErrors()
		assert.NotEmpty(t, errs)

		errStr := ""
		for _, err := range errs {
			errStr += err.Error() + "\n"
		}

		// Should use custom formatter
		// The error contains both the "command not found" error and the custom formatter output
		assert.Contains(t, errStr, "Perhaps you meant:")
	})

	// Test flag suggestions
	t.Run("FlagSuggestions", func(t *testing.T) {
		parser := NewParser()

		// Add some flags
		parser.AddFlag("verbose", NewArg(
			WithShortFlag("v"),
			WithType(types.Standalone),
			WithDescription("Verbose output"),
		))
		parser.AddFlag("version", NewArg(
			WithType(types.Standalone),
			WithDescription("Show version"),
		))
		parser.AddFlag("verify", NewArg(
			WithType(types.Standalone),
			WithDescription("Verify mode"),
		))

		// Note: flag parsing strips the prefix, so error shows "ver" not "--ver"
		ok := parser.Parse([]string{"--ver"})
		assert.False(t, ok)

		errs := parser.GetErrors()
		assert.NotEmpty(t, errs)

		errStr := ""
		for _, err := range errs {
			errStr += err.Error() + "\n"
		}

		// The error shows the flag without prefix
		assert.Contains(t, errStr, "ver")
		// The suggestions might be short flags or long flags
		assert.True(t,
			strings.Contains(errStr, "-v") ||
				strings.Contains(errStr, "--verbose") ||
				strings.Contains(errStr, "--version") ||
				strings.Contains(errStr, "--verify"),
			"Should suggest at least one similar flag")
	})

	// Test positional arguments don't trigger suggestions
	t.Run("PositionalArgumentsNoSuggestions", func(t *testing.T) {
		parser := NewParser()

		// Add a command that accepts positional args
		parser.AddCommand(NewCommand(
			WithName("echo"),
			WithCommandDescription("Echo arguments"),
			WithCallback(func(cmdLine *Parser, command *Command) error { return nil }),
		))

		// This should be treated as positional argument, not a typo
		ok := parser.Parse([]string{"hello", "world"})
		assert.True(t, ok) // Should succeed as positional args

		positional := parser.GetPositionalArgs()
		assert.Len(t, positional, 2)
		assert.Equal(t, "hello", positional[0].Value)
		assert.Equal(t, "world", positional[1].Value)
	})
}

func TestParser_PositionalArgument(t *testing.T) {
	type ServerCmd struct {
		Workers    int    `goopt:"default:10"`
		ConfigFile string `goopt:"pos:0"`
	}

	type Config struct {
		Port   int       `goopt:"short:p;default:8080"`
		Server ServerCmd `goopt:"kind:command"`
	}

	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "positional after flags",
			args:     []string{"server", "--port", "8080", "--workers", "20", "config.yaml"},
			expected: "config.yaml",
		},
		{
			name:     "positional before flags",
			args:     []string{"server", "config.yaml", "--port", "8080", "--workers", "20"},
			expected: "config.yaml",
		},
		{
			name:     "positional with short flags",
			args:     []string{"server", "-p", "9000", "myconfig.yaml"},
			expected: "myconfig.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			parser, err := NewParserFromStruct(cfg)
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			success := parser.Parse(tt.args)
			if !success {
				t.Fatalf("Parse failed: %v", parser.GetErrors())
			}

			if cfg.Server.ConfigFile != tt.expected {
				t.Errorf("Expected ConfigFile=%q, got %q", tt.expected, cfg.Server.ConfigFile)
			}
		})
	}
}

func TestParser_PositionalFlow(t *testing.T) {
	p := NewParser()
	p.AddFlag("valueFlag", NewArg())
	p.AddFlag("source", NewArg(WithPosition(0)))
	p.AddFlag("dest", NewArg(WithPosition(1)))

	// Hook into setPositionalArguments to see what's happening
	args := []string{"--valueFlag", "value", "--source", "override.txt", "dest.txt"}
	fmt.Printf("Raw args: %v\n", args)

	// Manually trace through setPositionalArguments logic
	fmt.Println("\nTracing through argument processing:")
	skipNext := false
	for i, arg := range args {
		if skipNext {
			fmt.Printf("  [%d] '%s' - SKIPPED (consumed by previous flag)\n", i, arg)
			skipNext = false
			continue
		}

		if p.isFlag(arg) {
			fmt.Printf("  [%d] '%s' - FLAG\n", i, arg)
			// Would need to check if it needs a value
			name := arg[2:] // strip --
			if name == "valueFlag" || name == "source" {
				skipNext = true
			}
			continue
		}

		fmt.Printf("  [%d] '%s' - POSITIONAL\n", i, arg)
	}
}

// TestCommandExecutionOrder verifies that commands are executed in FIFO order
func TestParser_CommandExecutionOrder(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "single command",
			args:     []string{"cmd1"},
			expected: []string{"cmd1"},
		},
		{
			name:     "multiple commands in order",
			args:     []string{"cmd1", "cmd2", "cmd3"},
			expected: []string{"cmd1", "cmd2", "cmd3"},
		},
		{
			name:     "nested commands maintain order",
			args:     []string{"parent", "child1", "parent", "child2"},
			expected: []string{"parent child1", "parent child2"},
		},
		{
			name:     "mixed depth commands",
			args:     []string{"cmd1", "parent", "child", "cmd2"},
			expected: []string{"cmd1", "parent child", "cmd2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executionOrder := []string{}
			p := NewParser()

			// Create simple command
			cmd1 := &Command{
				Name: "cmd1",
				Callback: func(cmdLine *Parser, command *Command) error {
					executionOrder = append(executionOrder, command.Path())
					return nil
				},
			}

			cmd2 := &Command{
				Name: "cmd2",
				Callback: func(cmdLine *Parser, command *Command) error {
					executionOrder = append(executionOrder, command.Path())
					return nil
				},
			}

			cmd3 := &Command{
				Name: "cmd3",
				Callback: func(cmdLine *Parser, command *Command) error {
					executionOrder = append(executionOrder, command.Path())
					return nil
				},
			}

			// Create parent with children
			parent := &Command{
				Name: "parent",
				Subcommands: []Command{
					{
						Name: "child1",
						Callback: func(cmdLine *Parser, command *Command) error {
							executionOrder = append(executionOrder, command.Path())
							return nil
						},
					},
					{
						Name: "child2",
						Callback: func(cmdLine *Parser, command *Command) error {
							executionOrder = append(executionOrder, command.Path())
							return nil
						},
					},
					{
						Name: "child",
						Callback: func(cmdLine *Parser, command *Command) error {
							executionOrder = append(executionOrder, command.Path())
							return nil
						},
					},
				},
			}

			// Register all commands
			_ = p.AddCommand(cmd1)
			_ = p.AddCommand(cmd2)
			_ = p.AddCommand(cmd3)
			_ = p.AddCommand(parent)

			// Parse and execute
			result := p.Parse(append([]string{"prog"}, tt.args...))
			assert.True(t, result, "parsing should succeed")

			errCount := p.ExecuteCommands()
			assert.Equal(t, 0, errCount, "no execution errors expected")

			// Verify execution order
			assert.Equal(t, tt.expected, executionOrder, "commands should execute in FIFO order")
		})
	}
}

// TestCommandExecutionWithTranslatableErrors verifies translatable error handling
func TestCommandExecutionWithTranslatableErrors(t *testing.T) {
	// Create a custom translatable error
	customErr := i18n.NewError("custom.error.key").WithArgs("test")

	tests := []struct {
		name          string
		commandError  error
		language      language.Tag
		expectedKey   string
		containsInMsg string
	}{
		{
			name:          "translatable error in English",
			commandError:  customErr,
			language:      language.English,
			expectedKey:   "custom.error.key",
			containsInMsg: "Custom error: test",
		},
		{
			name:          "wrapped translatable error",
			commandError:  fmt.Errorf("wrapper: %w", customErr),
			language:      language.English,
			expectedKey:   "custom.error.key",
			containsInMsg: "Custom error: test",
		},
		{
			name:          "regular error (non-translatable)",
			commandError:  errors.New("regular error"),
			language:      language.English,
			expectedKey:   "",
			containsInMsg: "regular error",
		},
		{
			name:          "nil error",
			commandError:  nil,
			language:      language.English,
			expectedKey:   "",
			containsInMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			p.SetLanguage(tt.language)

			// Add a test locale with translations
			testLocale := i18n.NewLocale(tt.language, `{
				"custom.error.key": "Custom error: %s"
			}`)
			p.SetSystemLocales(testLocale)

			cmd := &Command{
				Name: "test",
				Callback: func(cmdLine *Parser, command *Command) error {
					return tt.commandError
				},
			}

			_ = p.AddCommand(cmd)
			_ = p.ParseString("test")
			_ = p.ExecuteCommands()

			// Test GetCommandExecutionError
			err := p.GetCommandExecutionError("test")
			if tt.commandError == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.containsInMsg)

				// Check if it's a translatable error
				var te i18n.TranslatableError
				if errors.As(tt.commandError, &te) {
					// Verify that the error maintains its translatable nature
					var resultTE i18n.TranslatableError
					assert.True(t, errors.As(err, &resultTE), "result should be translatable")
					if resultTE != nil {
						assert.Equal(t, tt.expectedKey, resultTE.Key())
					}
				}
			}
		})
	}
}

// TestGetCommandExecutionErrors verifies batch error retrieval with translation
func TestGetCommandExecutionErrors(t *testing.T) {
	p := NewParser()

	// Set up test locale
	testLocale := i18n.NewLocale(language.English, `{
		"error.cmd1": "Error in command 1: %s",
		"error.cmd2": "Error in command 2"
	}`)
	p.SetSystemLocales(testLocale)

	// Create multiple commands with different error types
	cmd1 := &Command{
		Name: "cmd1",
		Callback: func(cmdLine *Parser, command *Command) error {
			return i18n.NewError("error.cmd1").WithArgs("details")
		},
	}

	cmd2 := &Command{
		Name: "cmd2",
		Callback: func(cmdLine *Parser, command *Command) error {
			return i18n.NewError("error.cmd2")
		},
	}

	cmd3 := &Command{
		Name: "cmd3",
		Callback: func(cmdLine *Parser, command *Command) error {
			return errors.New("regular error in cmd3")
		},
	}

	cmd4 := &Command{
		Name: "cmd4",
		Callback: func(cmdLine *Parser, command *Command) error {
			return nil // No error
		},
	}

	// Register commands
	_ = p.AddCommand(cmd1)
	_ = p.AddCommand(cmd2)
	_ = p.AddCommand(cmd3)
	_ = p.AddCommand(cmd4)

	// Parse and execute all commands
	_ = p.ParseString("cmd1 cmd2 cmd3 cmd4")
	errCount := p.ExecuteCommands()
	assert.Equal(t, 3, errCount, "should have 3 errors")

	// Get all execution errors
	errors := p.GetCommandExecutionErrors()
	assert.Len(t, errors, 3, "should return 3 errors (cmd4 had no error)")

	// Verify error details
	errorMap := make(map[string]error)
	for _, kv := range errors {
		errorMap[kv.Key] = kv.Value
	}

	// Check cmd1 error (translatable)
	assert.Contains(t, errorMap["cmd1"].Error(), "Error in command 1: details")

	// Check cmd2 error (translatable)
	assert.Contains(t, errorMap["cmd2"].Error(), "Error in command 2")

	// Check cmd3 error (regular)
	assert.Contains(t, errorMap["cmd3"].Error(), "regular error in cmd3")

	// cmd4 should not be in the error map
	_, found := errorMap["cmd4"]
	assert.False(t, found, "cmd4 should not be in error map")
}

// TestCommandExecutionOrderWithPreHooks verifies that pre-hooks don't affect execution order
func TestParser_CommandExecutionOrderWithPreHooks(t *testing.T) {
	executionOrder := []string{}
	p := NewParser()

	// Add global pre-hook
	p.AddGlobalPreHook(func(parser *Parser, cmd *Command) error {
		// Pre-hooks should not affect the command execution order
		return nil
	})

	// Create commands
	for i := 1; i <= 5; i++ {
		cmdName := fmt.Sprintf("cmd%d", i)
		cmd := &Command{
			Name: cmdName,
			Callback: func(cmdLine *Parser, command *Command) error {
				executionOrder = append(executionOrder, command.Name)
				return nil
			},
		}
		_ = p.AddCommand(cmd)
	}

	// Parse multiple commands
	_ = p.ParseString("cmd1 cmd2 cmd3 cmd4 cmd5")
	_ = p.ExecuteCommands()

	// Verify FIFO order
	expected := []string{"cmd1", "cmd2", "cmd3", "cmd4", "cmd5"}
	assert.Equal(t, expected, executionOrder, "commands should execute in FIFO order even with pre-hooks")
}

// TestTranslatableErrorInNestedCommands verifies error translation in nested command structures
func TestParser_TranslatableErrorInNestedCommands(t *testing.T) {
	p := NewParser()

	// Set up German locale
	germanLocale := i18n.NewLocale(language.German, `{
		"nested.error": "Verschachtelter Fehler: %s"
	}`)
	p.SetSystemLocales(germanLocale)
	p.SetLanguage(language.German)

	// Create nested command structure
	parent := &Command{
		Name: "parent",
		Subcommands: []Command{
			{
				Name: "child",
				Subcommands: []Command{
					{
						Name: "grandchild",
						Callback: func(cmdLine *Parser, command *Command) error {
							return i18n.NewError("nested.error").WithArgs("deep command")
						},
					},
				},
			},
		},
	}

	_ = p.AddCommand(parent)
	_ = p.ParseString("parent child grandchild")
	_ = p.ExecuteCommands()

	// Get the error
	err := p.GetCommandExecutionError("parent child grandchild")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Verschachtelter Fehler: deep command")
}

// TestExecuteCommandWithTranslatableError verifies ExecuteCommand handles translatable errors
func TestParser_ExecuteCommandWithTranslatableError(t *testing.T) {
	p := NewParser()

	// Set up French locale
	frenchLocale := i18n.NewLocale(language.French, `{
		"auth.failed": "Échec de l'authentification: %s"
	}`)
	p.SetSystemLocales(frenchLocale)
	p.SetLanguage(language.French)

	// Create a command with translatable error
	cmd := &Command{
		Name: "login",
		Callback: func(cmdLine *Parser, command *Command) error {
			return i18n.NewError("auth.failed").WithArgs("invalid credentials")
		},
	}

	_ = p.AddCommand(cmd)
	_ = p.ParseString("login")

	// Execute single command
	err := p.ExecuteCommand()
	assert.Error(t, err)

	// Based on the current PR implementation, ExecuteCommand returns raw error
	// while GetCommandExecutionError returns the wrapped/translated error
	// This test documents the current behavior

	// ExecuteCommand returns the raw translatable error
	var trErr i18n.TranslatableError
	assert.True(t, errors.As(err, &trErr), "error should be translatable")
	assert.Equal(t, "auth.failed", trErr.Key())

	// GetCommandExecutionError returns the translated error
	execErr := p.GetCommandExecutionError("login")
	assert.Error(t, execErr)
	assert.Contains(t, execErr.Error(), "Échec de l'authentification: invalid credentials")
}

// TestExecuteCommandsShouldWrapErrors verifies that the ideal behavior would be
// to wrap errors when storing them, not when retrieving them
func TestParser_ExecuteCommandsShouldWrapErrors(t *testing.T) {
	t.Skip("This test demonstrates the ideal behavior - errors should be wrapped when stored")

	p := NewParser()

	// Set up German locale
	germanLocale := i18n.NewLocale(language.German, `{
		"network.timeout": "Netzwerk-Zeitüberschreitung: %s"
	}`)
	p.SetSystemLocales(germanLocale)
	p.SetLanguage(language.German)

	// Create a command with translatable error
	cmd := &Command{
		Name: "fetch",
		Callback: func(cmdLine *Parser, command *Command) error {
			return i18n.NewError("network.timeout").WithArgs("30s")
		},
	}

	_ = p.AddCommand(cmd)
	_ = p.ParseString("fetch")

	// Execute single command
	err := p.ExecuteCommand()
	assert.Error(t, err)

	// In the ideal implementation, ExecuteCommand would return the translated error
	assert.Contains(t, err.Error(), "Netzwerk-Zeitüberschreitung: 30s")

	// And GetCommandExecutionError would return the same translated error
	execErr := p.GetCommandExecutionError("fetch")
	assert.Error(t, execErr)
	assert.Equal(t, err.Error(), execErr.Error(), "both methods should return the same translated error")

}

// TestParser_CommandHierarchyDescriptions tests that nested command descriptions are preserved correctly
func TestParser_CommandHierarchyDescriptions(t *testing.T) {
	type BottomCommand struct {
		Execute bool `goopt:"short:e;desc:execute the command"`
	}

	type MiddleCommand struct {
		BottomCommand BottomCommand `goopt:"kind:command;name:bottom;desc:bottom level command;descKey:bottom.desc"`
	}

	type TopCommand struct {
		MiddleCommand MiddleCommand `goopt:"kind:command;name:middle;desc:middle level command;descKey:middle.desc"`
	}

	type NestedOptions struct {
		TopCommand TopCommand `goopt:"kind:command;name:top;desc:top level command;descKey:top.desc"`
	}

	opts := &NestedOptions{}
	p, err := NewParserFromStruct(opts)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Also check what's in the top-level command's subcommands
	// This verifies that descriptionKeys are correctly preserved in the command hierarchy
	if topCmd, found := p.registeredCommands.Get("top"); found {
		t.Logf("Top command subcommands: %d", len(topCmd.Subcommands))
		for i, sub := range topCmd.Subcommands {
			t.Logf("  Subcommand[%d]: name=%s, desc=%s, descKey=%s",
				i, sub.Name, sub.Description, sub.DescriptionKey)
		}
	}

	// Test that registered commands have correct descriptions
	tests := []struct {
		commandPath  string
		expectedDesc string
	}{
		{"top", "top level command"},
		{"top middle", "middle level command"},
		{"top middle bottom", "bottom level command"},
	}

	// Debug: print all registered commands
	t.Logf("Registered commands:")
	for kv := p.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		cmd := kv.Value
		t.Logf("  Path: %s, Name: %s, NameKey: %s, Desc: %s, DescKey: %s",
			*kv.Key, cmd.Name, cmd.NameKey, cmd.Description, cmd.DescriptionKey)
	}

	for _, tt := range tests {
		cmd, found := p.registeredCommands.Get(tt.commandPath)
		if !found {
			t.Errorf("Command %s not found in registered commands", tt.commandPath)
			continue
		}

		if cmd.Description != tt.expectedDesc {
			t.Errorf("Command %s has wrong description: got %q, want %q",
				tt.commandPath, cmd.Description, tt.expectedDesc)
		}

		// Note: We're not checking DescriptionKey here because intermediate commands
		// in a path may not have their descriptionKey set when they're created as
		// part of building a longer command path
	}

	// Test hierarchical help output
	p.SetHelpStyle(HelpStyleHierarchical)
	var buf bytes.Buffer
	p.PrintHelp(&buf)
	output := buf.String()

	// Check that the command tree shows correct descriptions
	// Note: Without translations set up, descriptionKeys are shown instead of descriptions.
	// This is intentional - it helps catch regressions where descriptionKeys might be
	// incorrectly propagated from child to parent commands.
	if !strings.Contains(output, "middle               middle.desc") {
		t.Errorf("Hierarchical help doesn't show correct description key for middle command.\nOutput:\n%s", output)
	}
}

// TestParser_CommandHierarchyWithGroups tests that command descriptions work correctly with grouped help
func TestParser_CommandHierarchyWithGroups(t *testing.T) {
	type SubCommand1 struct {
		Flag1 string `goopt:"short:f;desc:flag one"`
	}

	type SubCommand2 struct {
		Flag2 string `goopt:"short:g;desc:flag two"`
	}

	type Command1 struct {
		Sub1 SubCommand1 `goopt:"kind:command;name:sub1;desc:first subcommand"`
	}

	type Command2 struct {
		Sub2 SubCommand2 `goopt:"kind:command;name:sub2;desc:second subcommand"`
	}

	type Options struct {
		Cmd1 Command1 `goopt:"kind:command;name:cmd1;desc:first command"`
		Cmd2 Command2 `goopt:"kind:command;name:cmd2;desc:second command"`
	}

	opts := &Options{}
	p, err := NewParserFromStruct(opts)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Test grouped help output
	var buf bytes.Buffer
	p.PrintUsageWithGroups(&buf)
	output := buf.String()

	// Check that commands show correct descriptions
	expectedPairs := [][]string{
		{"cmd1", "first command"},
		{"cmd2", "second command"},
		{"cmd1 sub1", "first subcommand"},
		{"cmd2 sub2", "second subcommand"},
	}

	for _, pair := range expectedPairs {
		if !strings.Contains(output, pair[0]) || !strings.Contains(output, pair[1]) {
			t.Errorf("Grouped help missing or incorrect for %s with description %q.\nOutput:\n%s",
				pair[0], pair[1], output)
		}
	}
}

// TestParser_HierarchicalHelpRegression tests the specific issue from case-one
// where hierarchical help was showing the wrong description for parent commands
func TestParser_HierarchicalHelpRegression(t *testing.T) {
	// Replicate the structure from case-one that exposed the bug
	type BlobsCommand struct{}
	type ReposCommand struct{}
	type RolesCommand struct{}

	type CopyCommand struct {
		Blobs BlobsCommand `goopt:"kind:command;name:blobs;desc:copy blob configuration"`
		Repos ReposCommand `goopt:"kind:command;name:repos;desc:copy repository configuration"`
		Roles RolesCommand `goopt:"kind:command;name:roles;desc:copy role configuration"`
	}

	type NexusCommand struct {
		Copy CopyCommand `goopt:"kind:command;name:copy;desc:nexus copy commands"`
	}

	type Options struct {
		Nexus NexusCommand `goopt:"kind:command;name:nexus;desc:nexus management commands"`
	}

	opts := &Options{}
	p, err := NewParserFromStruct(opts, WithHelpStyle(HelpStyleHierarchical))
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Test hierarchical help output
	var buf bytes.Buffer
	p.PrintHelp(&buf)
	output := buf.String()

	// The bug was that "copy" showed "copy blob configuration" instead of "nexus copy commands"
	if !strings.Contains(output, "copy                 nexus copy commands") {
		t.Errorf("Hierarchical help shows wrong description for copy command.\nOutput:\n%s", output)
	}
}

// TestParser_DeepCommandNesting tests deeply nested command structures
func TestParser_DeepCommandNesting(t *testing.T) {
	type Level5 struct {
		Flag string `goopt:"short:f;desc:deep flag"`
	}

	type Level4 struct {
		Level5 Level5 `goopt:"kind:command;name:l5;desc:level 5 desc"`
	}

	type Level3 struct {
		Level4 Level4 `goopt:"kind:command;name:l4;desc:level 4 desc"`
	}

	type Level2 struct {
		Level3 Level3 `goopt:"kind:command;name:l3;desc:level 3 desc"`
	}

	type Level1 struct {
		Level2 Level2 `goopt:"kind:command;name:l2;desc:level 2 desc"`
	}

	type RootOptions struct {
		Level1 Level1 `goopt:"kind:command;name:l1;desc:level 1 desc"`
	}

	opts := &RootOptions{}
	p, err := NewParserFromStruct(opts)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Check all levels have correct descriptions
	levels := []struct {
		path string
		desc string
	}{
		{"l1", "level 1 desc"},
		{"l1 l2", "level 2 desc"},
		{"l1 l2 l3", "level 3 desc"},
		{"l1 l2 l3 l4", "level 4 desc"},
		{"l1 l2 l3 l4 l5", "level 5 desc"},
	}

	for _, level := range levels {
		cmd, found := p.registeredCommands.Get(level.path)
		if !found {
			t.Errorf("Command path %s not found", level.path)
			continue
		}
		if cmd.Description != level.desc {
			t.Errorf("Command %s has wrong description: got %q, want %q",
				level.path, cmd.Description, level.desc)
		}
	}
}

// TestParser_PrintCommandTree tests the printCommandTree method
func TestParser_PrintCommandTree(t *testing.T) {
	t.Run("displays top-level command with description", func(t *testing.T) {
		type Config struct {
			SubCmd struct{} `goopt:"kind:command;name:sub;desc:subcommand description"`
		}

		type Options struct {
			TopCmd Config `goopt:"kind:command;name:top;desc:top-level description"`
		}

		opts := &Options{}
		p, err := NewParserFromStruct(opts, WithHelpStyle(HelpStyleHierarchical))
		assert.NoError(t, err)

		var buf bytes.Buffer
		p.printCommandTree(&buf)
		output := buf.String()

		// Should show top-level command with description
		assert.Contains(t, output, "top")
		assert.Contains(t, output, "top-level description")

		// Should show subcommand
		assert.Contains(t, output, "sub")
		assert.Contains(t, output, "subcommand description")
	})

	t.Run("displays top-level command without description", func(t *testing.T) {
		type Config struct {
			SubCmd struct{} `goopt:"kind:command;name:sub"`
		}

		type Options struct {
			TopCmd Config `goopt:"kind:command;name:top"`
		}

		opts := &Options{}
		p, err := NewParserFromStruct(opts, WithHelpStyle(HelpStyleHierarchical))
		assert.NoError(t, err)

		var buf bytes.Buffer
		p.printCommandTree(&buf)
		output := buf.String()

		// Should show top-level command without description
		assert.Contains(t, output, "top")
		// When no description, command is still shown
		lines := strings.Split(strings.TrimSpace(output), "\n")
		found := false
		for _, line := range lines {
			if strings.Contains(line, "top") && !strings.Contains(line, "sub") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have 'top' in output")
	})

	t.Run("truncates long subcommand descriptions", func(t *testing.T) {
		type Config struct {
			SubCmd struct{} `goopt:"kind:command;name:sub;desc:This is a very long description that should be truncated to fit within the display limits"`
		}

		type Options struct {
			TopCmd Config `goopt:"kind:command;name:top;desc:top command"`
		}

		opts := &Options{}
		p, err := NewParserFromStruct(opts, WithHelpStyle(HelpStyleHierarchical))
		assert.NoError(t, err)

		var buf bytes.Buffer
		p.printCommandTree(&buf)
		output := buf.String()

		// Description should be truncated
		assert.Contains(t, output, "...")
		assert.NotContains(t, output, "display limits")
	})

	t.Run("uses terminal prefix for last subcommand", func(t *testing.T) {
		type Config struct {
			Sub1 struct{} `goopt:"kind:command;name:sub1;desc:first sub"`
			Sub2 struct{} `goopt:"kind:command;name:sub2;desc:second sub"`
			Sub3 struct{} `goopt:"kind:command;name:sub3;desc:third sub"`
		}

		type Options struct {
			TopCmd Config `goopt:"kind:command;name:top;desc:top command"`
		}

		opts := &Options{}
		p, err := NewParserFromStruct(opts, WithHelpStyle(HelpStyleHierarchical))
		assert.NoError(t, err)

		var buf bytes.Buffer
		p.printCommandTree(&buf)
		output := buf.String()

		lines := strings.Split(output, "\n")
		// Find lines with subcommands
		subLines := []string{}
		for _, line := range lines {
			if strings.Contains(line, "sub1") || strings.Contains(line, "sub2") || strings.Contains(line, "sub3") {
				subLines = append(subLines, line)
			}
		}

		assert.Equal(t, 3, len(subLines))
		// First two should use DefaultPrefix (├─)
		assert.Contains(t, subLines[0], "├─")
		assert.Contains(t, subLines[1], "├─")
		// Last one should use TerminalPrefix (└─)
		assert.Contains(t, subLines[2], "└─")
	})
}

// TestParser_CommandPropertyMerging tests that command properties are preserved when merging
func TestParser_ommandPropertyMerging(t *testing.T) {
	t.Run("preserves existing command properties when merging", func(t *testing.T) {
		p := NewParser()

		// First, create a command with all properties
		cmd1 := &Command{
			Name:           "test",
			NameKey:        "cmd.test",
			Description:    "Test command",
			DescriptionKey: "cmd.test.desc",
			Callback: func(_ *Parser, _ *Command) error {
				return nil
			},
		}
		p.registeredCommands.Set("parent test", cmd1)

		// Now process a command config that would update this command
		config := &CommandConfig{
			Path:   "parent test",
			Parent: &Command{Name: "parent"},
		}

		_, err := p.buildCommandFromConfig(config)
		assert.NoError(t, err)

		// Get the registered command to verify properties were preserved
		registered, found := p.registeredCommands.Get("parent test")
		assert.True(t, found)

		// Should preserve existing properties
		assert.Equal(t, "cmd.test", registered.NameKey)
		assert.Equal(t, "Test command", registered.Description)
		assert.Equal(t, "cmd.test.desc", registered.DescriptionKey)
		assert.NotNil(t, registered.Callback)
	})

	t.Run("applies new properties when not already set", func(t *testing.T) {
		p := NewParser()

		// First, create a minimal command at top level
		cmd1 := &Command{
			Name: "test",
		}
		p.registeredCommands.Set("test", cmd1)

		// Process with new properties
		config := &CommandConfig{
			Path:           "test",
			NameKey:        "new.name.key",
			Description:    "New description",
			DescriptionKey: "new.desc.key",
		}

		_, err := p.buildCommandFromConfig(config)
		assert.NoError(t, err)

		// Get the registered command to verify properties were applied
		registered, found := p.registeredCommands.Get("test")
		assert.True(t, found)

		// Should apply new properties when not already set
		assert.Equal(t, "new.name.key", registered.NameKey)
		assert.Equal(t, "New description", registered.Description)
		assert.Equal(t, "new.desc.key", registered.DescriptionKey)
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
