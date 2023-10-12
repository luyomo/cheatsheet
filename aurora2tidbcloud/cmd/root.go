package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/luyomo/cheatsheet/aurora2tidbcloud/internal/app"
	"github.com/luyomo/cheatsheet/aurora2tidbcloud/internal/app/configs"
)

var (
	rootCmd *cobra.Command
	gOpt    configs.Options
)

func init() {
	rootCmd = &cobra.Command{
		Use:   os.Args[0],
		Short: "Data migration from Aurora to TiDB Cloud through Aurora snapshot ",
		Long: `Data migration from Aurora to TiDB Cloud, the whole process is as below:
    1. Make lambda function to get binlog info(binlog file/position)
    2. Make lambda function to dumpling ddl to S3 bucket
    3. Call API to take Aurora snapshot
    4. Call API to export parquet format data from snapshot to S3
    5. Call TiDB Cloud openapi to create the objects from dumpling ddl
    6. Call TiDB Cloud openapi to import parquet data from S3 to database
`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Giving help text")
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	makeCmdMain()
	makeCmdList()
	makeCmdDelete()
}

func makeCmdMain() {
	cmdMain := &cobra.Command{
		Use:   "run",
		Short: "Run the script to migrate the data from Aurora to TiDB Cloud",
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Run(gOpt); err != nil {
				fmt.Println(color.RedString("Error: %v", err))
			}
		},
	}
	cmdMain.PersistentFlags().StringVar(&gOpt.Opt01, "string-opt", "", "The string option")
	cmdMain.PersistentFlags().Int32Var(&gOpt.Opt02, "int32-opt", 0, "The int32 option")
	cmdMain.PersistentFlags().BoolVar(&gOpt.Opt03, "bool-opt", true, "The bool option")
	cmdMain.PersistentFlags().StringVar(&gOpt.ConfigFile, "config-file", "configs/config.toml", "The config file for the app")

	rootCmd.AddCommand(cmdMain)

}

// lambda function: binlog
// lambda function: dumpling
// S3 bucket: from dumpling
// Export role:
// Import role:
func makeCmdList() {
	cmdList := &cobra.Command{
		Use:   "list",
		Short: "List all the aws created resources",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Listing all the aws resources")
			// if err := app.Run(gOpt); err != nil {
			// 	fmt.Println(color.RedString("Error: %v", err))
			// }
		},
	}
	// cmdMain.PersistentFlags().StringVar(&gOpt.Opt01, "string-opt", "", "The string option")
	// cmdMain.PersistentFlags().Int32Var(&gOpt.Opt02, "int32-opt", 0, "The int32 option")
	// cmdMain.PersistentFlags().BoolVar(&gOpt.Opt03, "bool-opt", true, "The bool option")
	// cmdMain.PersistentFlags().StringVar(&gOpt.ConfigFile, "config-file", "configs/config.toml", "The config file for the app")

	rootCmd.AddCommand(cmdList)
}

func makeCmdDelete() {
	cmdDelete := &cobra.Command{
		Use:   "clean",
		Short: "Clean all the aws resources",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Cleaning all the aws resources")
			if err := app.Clean(gOpt); err != nil {
				fmt.Println(color.RedString("Error: %v", err))
			}
		},
	}
	// cmdMain.PersistentFlags().StringVar(&gOpt.Opt01, "string-opt", "", "The string option")
	// cmdMain.PersistentFlags().Int32Var(&gOpt.Opt02, "int32-opt", 0, "The int32 option")
	// cmdMain.PersistentFlags().BoolVar(&gOpt.Opt03, "bool-opt", true, "The bool option")
	// cmdMain.PersistentFlags().StringVar(&gOpt.ConfigFile, "config-file", "configs/config.toml", "The config file for the app")

	rootCmd.AddCommand(cmdDelete)
}

func Execute() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}
