package util

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/common"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
)

func ToTransformConfig(cmdOpt options.ExtractCmd) *common.TransformationConfig {
	return &common.TransformationConfig{
		TrPattern:           cmdOpt.TrPattern,
		KeepComments:        cmdOpt.KeepComments,
		CleanComments:       cmdOpt.CleanComments,
		IsUpdateMode:        cmdOpt.AutoUpdate,
		TransformMode:       cmdOpt.TransformMode,
		BackupDir:           cmdOpt.BackupDir,
		PackagePath:         cmdOpt.Package,
		UserFacingRegex:     cmdOpt.UserFacingRegex,
		FormatFunctionRegex: cmdOpt.FormatFunctionRegex,
	}
}
