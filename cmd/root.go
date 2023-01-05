package cmd

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/j178/leetgo/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "0.0.1"
	commit  = "HEAD"
	date    = "unknown"
)

const website = "https://github.com/j178/leetgo"

func buildVersion(version, commit, date string) string {
	result := version
	if commit != "" {
		result = fmt.Sprintf("%s\ncommit: %s", result, commit)
	}
	if date != "" {
		result = fmt.Sprintf("%s\nbuilt at: %s", result, date)
	}
	result = fmt.Sprintf("%s\ngoos: %s\ngoarch: %s", result, runtime.GOOS, runtime.GOARCH)
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
		result = fmt.Sprintf("%s\nmodule version: %s, checksum: %s", result, info.Main.Version, info.Main.Sum)
	}
	return result + "\n\n" + website
}

func loadConfig(cmd *cobra.Command, args []string) error {
	// load global configuration
	cfg := config.Default()
	viper.SetConfigFile(cfg.GlobalConfigFile())
	err := viper.ReadInConfig()
	if err != nil {
		if os.IsNotExist(err) {
			if cmd != initCmd {
				hclog.L().Warn(
					"global config file not found, have you ran `leetgo init`?",
					"file",
					cfg.GlobalConfigFile(),
				)
			}
			return nil
		}
		return err
	}
	// load project configuration
	viper.SetConfigFile(cfg.ProjectConfigFile())
	err = viper.MergeInConfig()
	if err != nil {
		if os.IsNotExist(err) {
			hclog.L().Warn("project config file not found, use global config only", "file", cfg.GlobalConfigFile())
		} else {
			return err
		}
	}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return err
	}
	if err = config.Verify(cfg); err != nil {
		return fmt.Errorf("config file is invalid: %w", err)
	}

	config.Set(*cfg)
	return err
}

var rootCmd = &cobra.Command{
	Use:           config.CmdName,
	Short:         "Leetcode",
	Long:          "Leetcode friend for geek.",
	Version:       buildVersion(version, commit, date),
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		initLogger()
		return loadConfig(cmd, args)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, color.New(color.FgHiRed).Sprint("Error:"), err)
		os.Exit(1)
	}
}

func UsageString() string {
	return rootCmd.UsageString()
}

func initLogger() {
	opts := &hclog.LoggerOptions{
		Level:           hclog.Info,
		DisableTime:     true,
		Color:           hclog.AutoColor,
		ColorHeaderOnly: true,
	}
	if config.Debug {
		opts.Level = hclog.Trace
		opts.DisableTime = false
		opts.Color = hclog.ColorOff
	}
	hclog.SetDefault(hclog.New(opts))
}

func initCommands() {
	cobra.EnableCommandSorting = false

	rootCmd.InitDefaultVersionFlag()
	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().StringP("lang", "l", "", "language of code to generate: cpp, go, python ...")
	rootCmd.PersistentFlags().BoolP("yes", "y", false, "answer yes to all prompts")
	rootCmd.InitDefaultHelpFlag()
	_ = viper.BindPFlag("code.lang", rootCmd.PersistentFlags().Lookup("lang"))
	_ = viper.BindPFlag("yes", rootCmd.PersistentFlags().Lookup("yes"))

	commands := []*cobra.Command{
		initCmd,
		pickCmd,
		infoCmd,
		testCmd,
		submitCmd,
		contestCmd,
		cacheCmd,
		configCmd,
	}
	for _, cmd := range commands {
		cmd.Flags().SortFlags = false
		rootCmd.AddCommand(cmd)
	}
	rootCmd.InitDefaultHelpCmd()

	cc.Init(
		&cc.Config{
			RootCmd:         rootCmd,
			Headings:        cc.HiCyan + cc.Bold + cc.Underline,
			Commands:        cc.HiYellow + cc.Bold,
			Example:         cc.Italic,
			ExecName:        cc.Bold,
			Flags:           cc.Bold,
			NoExtraNewlines: true,
			NoBottomNewline: true,
		},
	)
}

func init() {
	initCommands()
}
