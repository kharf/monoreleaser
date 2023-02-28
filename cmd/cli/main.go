package main

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	monoreleaser "github.com/kharf/monoreleaser/internal"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RootCommandBuilder struct {
	releaseCmdBuilder ReleaseCommandBuilder
}

func (builder RootCommandBuilder) Build() *cobra.Command {
	rootCmd := cobra.Command{
		Use:   "monoreleaser",
		Short: "A Monorepo-aware release CLI with Git inside.",
		Long: `Monoreleaser is a CLI to create and view Releases for any Git Repository.
It aims to support a variety of Languages, Repository structures and Git hosting services.`,
	}

	releaseCmd := builder.releaseCmdBuilder.Build()
	rootCmd.AddCommand(releaseCmd)

	return &rootCmd
}

type ReleaseCommandBuilder struct {
	releaser monoreleaser.Releaser
	fs       afero.Fs
}

func (builder ReleaseCommandBuilder) Build() *cobra.Command {
	var artifacts *[]string
	cmd := &cobra.Command{
		Use:   "release [MODULE] [VERSION]",
		Short: "Release a piece of Software (Module)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			var module string
			var moduleDir string
			if args[0] == "." {
				module = ""
				moduleDir = ""
			} else {
				module = args[0]
				moduleDir = module + "/"
			}

			var mrArtifacts []monoreleaser.Artifact
			for _, artifact := range *artifacts {
				artifactNames := strings.SplitAfter(artifact, "/")
				artifactName := artifactNames[len(artifactNames)-1]
				file, err := builder.fs.Open(moduleDir + artifact)
				if err != nil {
					return err
				}
				defer file.Close()
				fileStat, err := file.Stat()
				if err != nil {
					return err
				}
				mrArtifacts = append(mrArtifacts, monoreleaser.Artifact{Reader: bufio.NewReader(file), Name: artifactName, Size: fileStat.Size()})
			}

			return builder.releaser.Release(args[1], monoreleaser.ReleaseOptions{
				Module:    module,
				Artifacts: mrArtifacts,
			})
		},
	}

	artifacts = cmd.Flags().StringSlice("artifacts", []string{}, "artifacts to upload alongside the changelog (if supported by the provider)")
	return cmd
}

func main() {
	repository, err := git.PlainOpen(".")
	if err != nil {
		fmt.Println(err)
		return
	}

	config, err := initConfig(".monoreleaser.yaml")
	if err != nil {
		fmt.Println(err)
		return
	}
	rootCmdBuilder, err := initCli(repository, config, afero.NewOsFs())
	if err != nil {
		fmt.Println(err)
		return
	}

	err = rootCmdBuilder.Build().Execute()
	if err != nil {
		fmt.Println(err)
		return
	}
}

type PlaceholderReleaser struct{}

var _ monoreleaser.Releaser = PlaceholderReleaser{}
var ErrUnimplemented = errors.New("implement me daddy")

func (rel PlaceholderReleaser) Release(version string, opts monoreleaser.ReleaseOptions) error {
	return ErrUnimplemented
}

func initConfig(configFile string) (*viper.Viper, error) {
	config := viper.New()
	config.SetConfigFile(configFile)
	config.SetEnvPrefix("mr")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := config.BindEnv("github.token"); err != nil {
		return nil, err
	}

	if err := config.ReadInConfig(); err != nil {
		return nil, err
	}

	return config, nil
}

func initCli(repository *git.Repository, config *viper.Viper, fs afero.Fs) (*RootCommandBuilder, error) {
	owner := config.GetString("owner")
	name := config.GetString("name")
	provider := config.GetString("provider")
	token := config.GetString("github.token")

	gitRepository := monoreleaser.NewGoGitRepository(name, repository)

	var releaser monoreleaser.Releaser
	if provider == "github" {
		var err error
		releaser, err = monoreleaser.NewGithubReleaser(
			owner,
			gitRepository,
			monoreleaser.UserSettings{Token: token},
		)
		if err != nil {
			return nil, err
		}
	} else {
		releaser = PlaceholderReleaser{}
	}

	releaseCmd := ReleaseCommandBuilder{releaser: releaser, fs: fs}
	rootCmd := RootCommandBuilder{releaseCmd}

	return &rootCmd, nil
}
