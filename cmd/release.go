package cmd

import (
	"time"

	"github.com/apex/log"
	"github.com/caarlos0/ctrlc"
	"github.com/fatih/color"
	"github.com/goreleaser/goreleaser/internal/middleware"
	"github.com/goreleaser/goreleaser/internal/pipeline"
	"github.com/goreleaser/goreleaser/pkg/context"
	"github.com/spf13/cobra"
)

type releaseCmd struct {
	cmd  *cobra.Command
	opts releaseOpts
}

type releaseOpts struct {
	config        string
	releaseNotes  string
	releaseHeader string
	releaseFooter string
	snapshot      bool
	skipPublish   bool
	skipSign      bool
	skipValidate  bool
	rmDist        bool
	deprecated    bool
	parallelism   int
	timeout       time.Duration
}

func newReleaseCmd() *releaseCmd {
	var root = &releaseCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "release",
		Aliases:       []string{"r"},
		Short:         "Releases the current project",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()

			log.Infof(color.New(color.Bold).Sprint("releasing..."))

			ctx, err := releaseProject(root.opts)
			if err != nil {
				return wrapError(err, color.New(color.Bold).Sprintf("release failed after %0.2fs", time.Since(start).Seconds()))
			}

			if ctx.Deprecated {
				log.Warn(color.New(color.Bold).Sprintf("your config is using deprecated properties, check logs above for details"))
			}

			log.Infof(color.New(color.Bold).Sprintf("release succeeded after %0.2fs", time.Since(start).Seconds()))
			return nil
		},
	}

	cmd.Flags().StringVarP(&root.opts.config, "config", "f", "", "Load configuration from file")
	cmd.Flags().StringVar(&root.opts.releaseNotes, "release-notes", "", "Load custom release notes from a markdown file")
	cmd.Flags().StringVar(&root.opts.releaseHeader, "release-header", "", "Load custom release notes header from a markdown file")
	cmd.Flags().StringVar(&root.opts.releaseFooter, "release-footer", "", "Load custom release notes footer from a markdown file")
	cmd.Flags().BoolVar(&root.opts.snapshot, "snapshot", false, "Generate an unversioned snapshot release, skipping all validations and without publishing any artifacts")
	cmd.Flags().BoolVar(&root.opts.skipPublish, "skip-publish", false, "Skips publishing artifacts")
	cmd.Flags().BoolVar(&root.opts.skipSign, "skip-sign", false, "Skips signing the artifacts")
	cmd.Flags().BoolVar(&root.opts.skipValidate, "skip-validate", false, "Skips several sanity checks")
	cmd.Flags().BoolVar(&root.opts.rmDist, "rm-dist", false, "Remove the dist folder before building")
	cmd.Flags().IntVarP(&root.opts.parallelism, "parallelism", "p", 4, "Amount tasks to run concurrently")
	cmd.Flags().DurationVar(&root.opts.timeout, "timeout", 30*time.Minute, "Timeout to the entire release process")
	cmd.Flags().BoolVar(&root.opts.deprecated, "deprecated", false, "Force print the deprecation message - tests only")
	_ = cmd.Flags().MarkHidden("deprecated")

	root.cmd = cmd
	return root
}

func releaseProject(options releaseOpts) (*context.Context, error) {
	cfg, err := loadConfig(options.config)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.NewWithTimeout(cfg, options.timeout)
	defer cancel()
	setupReleaseContext(ctx, options)
	return ctx, ctrlc.Default.Run(ctx, func() error {
		for _, pipe := range pipeline.Pipeline {
			if err := middleware.Logging(
				pipe.String(),
				middleware.ErrHandler(pipe.Run),
				middleware.DefaultInitialPadding,
			)(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func setupReleaseContext(ctx *context.Context, options releaseOpts) *context.Context {
	ctx.Parallelism = options.parallelism
	log.Debugf("parallelism: %v", ctx.Parallelism)
	ctx.ReleaseNotes = options.releaseNotes
	ctx.ReleaseHeader = options.releaseHeader
	ctx.ReleaseFooter = options.releaseFooter
	ctx.Snapshot = options.snapshot
	ctx.SkipPublish = ctx.Snapshot || options.skipPublish
	ctx.SkipValidate = ctx.Snapshot || options.skipValidate
	ctx.SkipSign = options.skipSign
	ctx.RmDist = options.rmDist

	// test only
	ctx.Deprecated = options.deprecated
	return ctx
}
