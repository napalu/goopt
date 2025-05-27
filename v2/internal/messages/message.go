package messages

const (
	prefixKey        = "goopt"
	WarningPrefixKey = prefixKey + ".warning"
	MessagePrefixKey = prefixKey + ".msg"
)

// Warnings contains keys for warning messages
const (
	WarnDependencyNotSpecifiedKey      = WarningPrefixKey + ".dependency_not_specified"
	WarnDependencyValueNotSpecifiedKey = WarningPrefixKey + ".dependency_value_not_specified"
)

// UIMessages contains keys for user interface messages
const (
	MsgOptionalKey            = MessagePrefixKey + ".optional"
	MsgRequiredKey            = MessagePrefixKey + ".required"
	MsgConditionalKey         = MessagePrefixKey + ".conditional"
	MsgDefaultsToKey          = MessagePrefixKey + ".defaults_to"
	MsgPositionalKey          = MessagePrefixKey + ".positional"
	MsgOrKey                  = MessagePrefixKey + ".or"
	MsgUsageKey               = MessagePrefixKey + ".usage"
	MsgCommandsKey            = MessagePrefixKey + ".commands"
	MsgGlobalFlagsKey         = MessagePrefixKey + ".global_flags"
	MsgPositionalArgumentsKey = MessagePrefixKey + ".positional_arguments"
)

// Help message keys - placeholders for future expansion
const (
	HelpUsageKey    = prefixKey + ".help.usage"
	HelpCommandsKey = prefixKey + ".help.commands"
	HelpFlagsKey    = prefixKey + ".help.flags"
	HelpExamplesKey = prefixKey + ".help.examples"
	HelpFooterKey   = prefixKey + ".help.footer"
)
