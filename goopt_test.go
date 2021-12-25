package goopt_test

import (
	"fmt"
	. "github.com/napalu/goopt"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
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

func TestCmdLineOption_AcceptValue(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	_ = opts.AddFlag("test2", NewArgument("t2", "", Single, false, Secure{}, ""))

	err := opts.AcceptValue("test", `^[0-9]+$`, "whole integers only")
	assert.NotNil(t, err, "constraint violation - 'Standalone' flags don't take values and therefore should not AcceptValue")

	err = opts.AcceptValue("test2", `^[0-9]+$`, "whole integers only")
	assert.Nil(t, err, "constraint violation - 'Single' flags take values and therefore should AcceptValue")
	assert.True(t, opts.Parse([]string{"--test2", "12344"}), "test2 should accept values which match whole integer patterns")
}

func TestCmdLineOption_AcceptValues(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("test", NewArgument("t", "", Single, false, Secure{}, ""))

	err := opts.AcceptValues("test", []string{`^[0-9]+$`, `^[0-9]+\.[0-9]+`}, []string{"whole integers", "float numbers"})
	assert.Nil(t, err, "should accept multiple AcceptValues")
	assert.True(t, opts.Parse([]string{"--test", "12344"}), "test should accept values which match whole integer patterns")
	assert.True(t, opts.Parse([]string{"--test", "12344.123"}), "test should accept values which match float patterns")
	assert.False(t, opts.Parse([]string{"--test", "alphabet"}), "test should not accept alphabetical values")

	for _, err := range opts.GetErrors() {
		assert.Contains(t, err, "whole integers, float numbers", "the errors should include the accepted value pattern descriptions")
	}
}

func TestCmdLineOption_AddPreValidationFilter(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("upper", NewArgument("t", "", Single, false, Secure{}, ""))
	err := opts.AddFlagPreValidationFilter("upper", strings.ToUpper)
	assert.Nil(t, err, "should be able to add a filter to a valid flag")

	_ = opts.AcceptValue("upper", "^[A-Z]+$", "upper case only")
	assert.True(t, opts.HasPreValidationFilter("upper"), "flag should have a filter defined")
	assert.True(t, opts.Parse([]string{"--upper", "lowercase"}), "parse should not fail and pass AcceptValue properly")

	value, _ := opts.Get("upper")
	assert.Equal(t, "LOWERCASE", value, "the value of flag upper should be transformed to uppercase")
}

func TestCmdLineOption_AddPostValidationFilter(t *testing.T) {
	opts := NewCmdLineOption()

	_ = opts.AddFlag("upper", NewArgument("t", "", Single, false, Secure{}, ""))
	err := opts.AddFlagPostValidationFilter("upper", strings.ToUpper)
	assert.Nil(t, err, "should be able to add a filter to a valid flag")

	_ = opts.AcceptValue("upper", "^[A-Z]+$", "upper case only")
	assert.True(t, opts.HasPreValidationFilter("upper"), "flag should have a filter defined")
	assert.True(t, opts.Parse([]string{"--upper", "lowercase"}), "parse should not fail and pass AcceptValue properly")

	value, _ := opts.Get("upper")
	assert.Equal(t, "LOWERCASE", value, "the value of flag upper should be transformed to uppercase")
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
		Name:         "",
		Subcommands:  nil,
		Callback:     nil,
		Description:  "",
		DefaultValue: "",
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

func TestCmdLineOption_GetCommandValue(t *testing.T) {
	opts := NewCmdLineOption()

	cmd := &Command{
		Name: "create",
		Subcommands: []Command{{
			Name: "user",
			Subcommands: []Command{{
				Name: "type",
			}},
		}},
	}

	err := opts.AddCommand(cmd)
	assert.Nil(t, err, "should properly add named command chain")

	assert.True(t, opts.ParseString("create user type author"), "should parse well-formed command")
	assert.True(t, opts.HasCommand("create"), "should properly register root command")
	assert.True(t, opts.HasCommand("create user"), "should properly register sub-command")
	assert.True(t, opts.HasCommand("create user type"), "should properly register nested sub-command")
	value, err := opts.GetCommandValue("create user type")
	assert.Nil(t, err, "should find value of sub-command")
	assert.Equal(t, "author", value, "value of nested sub-command should be that supplied via command line")
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
				Callback: func(cmdLine *CmdLineOption, command *Command, value string) error {
					var err error
					if value != "author" {
						err = fmt.Errorf("should receive nested sub-command value on parse")
					}

					shouldBeEqualToOneAfterExecute = 1
					return err
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

	_, err := NewCmdLine(
		WithBindFlag("test", s,
			NewArg(
				WithShortFlag("t"),
				WithType(Single))))
	assert.NotNil(t, err, "should fail to bind non-pointer variable to flag when using fluent interface")

	_, err = NewCmdLine(
		WithBindFlag("test", &s,
			NewArg(
				WithShortFlag("t"),
				WithType(Single))))
	assert.Nil(t, err, "should not fail to bind pointer variable to flag when using fluent interface")

	cmdLine, err := NewCmdLine(
		WithBindFlag("test", &s,
			NewArg(WithShortFlag("t"),
				WithType(Single))),
		WithBindFlag("test1", &i,
			NewArg(WithShortFlag("i"),
				WithType(Single))))

	assert.Nil(t, err, "should not fail to bind multiple pointer variables to flag when using fluent interface")
	assert.True(t, cmdLine.ParseString("--test value --test1 12334"), "should be able to parse a fluent argument")
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

	assert.True(t, opts.ParseString("--test \"hello world\" --test1 12334"), "should parse a command line argument when given a bound variable")
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

	var boolBind bool
	err = opts.BindFlag(&boolBind, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.Nil(t, err, "should accept Standalone flags in BindFlag if the data type is boolean")

	err = opts.BindFlag(&i, "test", NewArgument("t", "", Standalone, false, Secure{}, ""))
	assert.NotNil(t, err, "should not accept Standalone flags in BindFlag if the data type is not boolean")
}

func TestNewCmdLineOption_BindNil(t *testing.T) {
	opts := NewCmdLineOption()

	type tester struct {
		TestStr string
		testInt int
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
	assert.Nil(t, err, "fluent flag composition should work")

	assert.True(t, cmdLine.ParseString(`--flagWithValue 
		"test value" --fa --flagB 
--flagC "1|2|3" create user type
 author create group member admin`), "command line options should be passed correctly")

	value, err := cmdLine.GetCommandValue("create user type")
	assert.Nil(t, err, "should find value of sub-command")
	assert.Equal(t, "author", value, "value of nested sub-command should be that supplied via command line")

	value, err = cmdLine.GetCommandValue("create nil type")
	assert.NotNil(t, err, "should find not value of a sub-command when part of the path is incorrect")
	assert.Equal(t, "", value, "value of nested sub-command should be empty for incorrect path")

	value, err = cmdLine.GetCommandValue("create group member")
	assert.Nil(t, err, "should find value of sub-command")
	assert.Equal(t, "admin", value, "value of nested sub-command should be that supplied via command line")

	list, err := cmdLine.GetList("flagC")
	assert.Nil(t, err, "chained flag should return a list")
	assert.Len(t, list, 3, "list should contain the three values passed on command line")

	val, found := cmdLine.Get("flagB")
	assert.True(t, found, "flagB was supplied on command line we expect it to be err")
	assert.Equal(t, "db", val, "flagB was specified on command line but no value was given,"+
		" we expect it to have the default value")

	warnings := cmdLine.GetWarnings()
	assert.Len(t, warnings, 0, "no warnings were expected all options were supplied")

	allCommands := cmdLine.GetCommandValues()
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

func TestCmdLineOption_FluentCommands(t *testing.T) {
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
						func(cmdLine *CmdLineOption, command *Command, value string) error {
							valueReceived = value

							return nil
						}),
				),
				NewCommand(
					WithName("group"),
					WithCommandDescription("create group")),
			)))
	assert.Nil(t, err, "should be able to fluently add commands")

	assert.True(t, opts.ParseString("create user test create group test2"), "should be able to parse commands")

	assert.Equal(t, "", valueReceived, "command callback should not be called before execute")
	assert.Equal(t, 0, opts.ExecuteCommands(), "command callback error should be nil if no error occurred")
	assert.Equal(t, "test", valueReceived, "command callback should be called on execute")

	val, err := opts.GetCommandValue("create user")
	assert.Nil(t, err, "error should be nil when retrieving existing command value")
	assert.Equal(t, "test", val, "value of terminating command should be correct")
	val, err = opts.GetCommandValue("create group")
	assert.Nil(t, err, "error should be nil when retrieving existing command value")
	assert.Equal(t, "test2", val, "value of terminating command should be correct")
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

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
