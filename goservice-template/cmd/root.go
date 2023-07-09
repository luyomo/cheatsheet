package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/pingcap/tiup/pkg/tui"
    "github.com/fatih/color"

    "github.com/luyomo/cheatsheet/goservice-template/internal/app"
    "github.com/luyomo/cheatsheet/goservice-template/internal/app/configs"
)

// type Options struct {
//     opt01 string
//     opt02 int32
//     opt03 bool
// }

var (
    rootCmd       *cobra.Command
    gOpt          configs.Options
)

func init() {
    rootCmd = &cobra.Command{
        Use:   "mycli",
        Short: "My CLI is an example command-line interface",
        Long:  "My CLI is a demonstration of how to build a CLI using Cobra",
        PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
            return nil
        },
        Run: func(cmd *cobra.Command, args []string) {
            if err := app.Run(gOpt); err != nil {
                fmt.Println(color.RedString("Error: %v", err)) 
            }
        },
        PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
            return nil
        },
    }

    tui.BeautifyCobraUsageAndHelp(rootCmd)

    rootCmd.PersistentFlags().StringVar(&gOpt.Opt01, "string-opt", "", "The string option")
    rootCmd.PersistentFlags().Int32Var(&gOpt.Opt02, "int32-opt", 0, "The int32 option")
    rootCmd.PersistentFlags().BoolVar(&gOpt.Opt03, "bool-opt", true, "The bool option")
    rootCmd.PersistentFlags().StringVar(&gOpt.ConfigFile, "config-file","configs/config.toml", "The config file for the app")

}

func Execute() {

    if err := rootCmd.Execute(); err != nil {
        fmt.Println(color.RedString("Error: %v", err)) 
		fmt.Println(err)
		os.Exit(1)
	}

    os.Exit(0)
}
