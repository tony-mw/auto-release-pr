/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"github.com/spf13/cobra"
)

// prodCmd represents the prod command
var prodCmd = &cobra.Command{
	Use:   "prod",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Println("prod called")
		stagingRepoSlug, _ := cmd.Flags().GetString("staging-repo-slug")
		prodRepoSlug, _ := cmd.Flags().GetString("prod-repo-slug")
		bbProject, _ := cmd.Flags().GetString("bitbucket-project")
		sourceBranch, _ := cmd.Flags().GetString("source-branch")
		product, _ := cmd.Flags().GetString("product")
		services, _ := cmd.Flags().GetStringSlice("services")

		myProdConfig := PrConfig{
			StagingRepoSlug: stagingRepoSlug,
			ProdRepoSlug:    prodRepoSlug,
			BBProject:       bbProject,
			SourceBranch:    sourceBranch,
			Product:         product,
			Services:        services,
		}

		PrepRelease(myProdConfig)
	},
}

func init() {
	rootCmd.AddCommand(prodCmd)

	prodCmd.PersistentFlags().String("staging-repo-slug", "", "The repository slug for staging")
	prodCmd.PersistentFlags().String("prod-repo-slug", "", "The repository slug for prod")
	prodCmd.PersistentFlags().String("bitbucket-project", "", "The repository bitbucket project")
	prodCmd.PersistentFlags().String("source-branch", "", "The branch to create")
	prodCmd.PersistentFlags().String("product", "", "The product which will also be the top level directory of the repo")
	prodCmd.PersistentFlags().StringSlice("services", []string{""}, "A list of the services that will be deployed to staging")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// prodCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// prodCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
