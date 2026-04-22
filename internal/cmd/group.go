package cmd

import (
	"fmt"
	"strings"

	"github.com/pinealctx/mrepo/internal/config"

	"github.com/spf13/cobra"
)

type jsonGroup struct {
	Name  string   `json:"name"`
	Repos []string `json:"repos"`
}

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Manage repo groups",
}

var groupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all groups and their repos",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		if jsonOutput {
			groups := make([]jsonGroup, 0, len(cfg.Groups))
			for _, name := range cfg.SortedGroupNames() {
				groups = append(groups, jsonGroup{
					Name:  name,
					Repos: cfg.Groups[name].Repos,
				})
			}
			return printJSON(groups)
		}

		if len(cfg.Groups) == 0 {
			fmt.Println(infoStyle.Render("  No groups defined."))
			return nil
		}

		t := newHeaderTable("GROUP", "REPOS")

		for _, name := range cfg.SortedGroupNames() {
			group := cfg.Groups[name]
			repoList := dimStyle.Render(strings.Join(group.Repos, ", "))
			if len(group.Repos) == 0 {
				repoList = dimStyle.Render("(empty)")
			}
			t.Row(accentStyle.Render(name), repoList)
		}

		fmt.Println(t.Render())
		return nil
	},
}

var groupCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfgPath, err := config.EnsureConfig(rootDir, format)
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		if err := cfg.AddGroup(name); err != nil {
			return err
		}
		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		fmt.Printf("  %s %s %s\n", successIcon(), boldStyle.Render("Created group"), accentStyle.Render(name))
		return nil
	},
}

var groupDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfgPath, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		if err := cfg.DeleteGroup(name); err != nil {
			return err
		}
		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		fmt.Printf("  %s %s %s\n", errorIcon(), boldStyle.Render("Deleted group"), accentStyle.Render(name))
		return nil
	},
}

var groupAddCmd = &cobra.Command{
	Use:   "add [name] [repo...]",
	Short: "Add repos to a group",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		repoNames := args[1:]

		cfgPath, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		var added []string
		for _, r := range repoNames {
			if err := cfg.AddRepoToGroup(name, r); err != nil {
				fmt.Printf("  %s %s\n", warnIcon(), dimStyle.Render(err.Error()))
				continue
			}
			added = append(added, r)
		}

		if len(added) == 0 {
			return fmt.Errorf("no repos added to group %q", name)
		}

		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		fmt.Printf("  %s %s %s %s %s\n",
			successIcon(),
			boldStyle.Render("Added"),
			dimStyle.Render(strings.Join(added, ", ")),
			dimStyle.Render("→"),
			accentStyle.Render(name))
		return nil
	},
}

var groupRemoveCmd = &cobra.Command{
	Use:   "remove [name] [repo...]",
	Short: "Remove repos from a group",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		repoNames := args[1:]

		cfgPath, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		var removed []string
		for _, r := range repoNames {
			if err := cfg.RemoveRepoFromGroup(name, r); err != nil {
				fmt.Printf("  %s %s\n", warnIcon(), dimStyle.Render(err.Error()))
				continue
			}
			removed = append(removed, r)
		}

		if len(removed) == 0 {
			return fmt.Errorf("no repos removed from group %q", name)
		}

		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		fmt.Printf("  %s %s %s %s %s\n",
			errorIcon(),
			boldStyle.Render("Removed"),
			dimStyle.Render(strings.Join(removed, ", ")),
			dimStyle.Render("from"),
			accentStyle.Render(name))
		return nil
	},
}

func init() {
	groupListCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")

	groupCmd.AddCommand(groupListCmd, groupCreateCmd, groupDeleteCmd, groupAddCmd, groupRemoveCmd)
	rootCmd.AddCommand(groupCmd)
}
