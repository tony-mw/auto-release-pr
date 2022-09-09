/*
Copyright © 2022 Tony Prestifilippo
*/

package cmd

import (
	"github.com/spf13/cobra"
)

// stagingCmd represents the staging command
var stagingCmd = &cobra.Command{
	Use:   "staging",
	Short: "Trigger an automatic PR to the staging environment",
	Long:  "Trigger an automatic PR to the staging environment",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Println("staging called")
		repoSlug, _ := cmd.Flags().GetString("repo-slug")
		bbProject, _ := cmd.Flags().GetString("bitbucket-project")
		sourceBranch, _ := cmd.Flags().GetString("source-branch")
		product, _ := cmd.Flags().GetString("product")
		services, _ := cmd.Flags().GetStringSlice("services")

		myStagingConfig := PrConfig{
			StagingRepoSlug: repoSlug,
			BBProject:       bbProject,
			SourceBranch:    sourceBranch,
			Product:         product,
			Services:        services,
		}

		PrepRelease(myStagingConfig)
	},
}

func init() {
	rootCmd.AddCommand(stagingCmd)

	stagingCmd.PersistentFlags().String("repo-slug", "", "The repository slug")
	stagingCmd.PersistentFlags().String("bitbucket-project", "", "The repository bitbucket project")
	stagingCmd.PersistentFlags().String("source-branch", "", "The branch to create")
	stagingCmd.PersistentFlags().String("product", "", "The product which will also be the top level directory of the repo")
	stagingCmd.PersistentFlags().StringSlice("services", []string{""}, "A list of the services that will be deployed to staging")
}
