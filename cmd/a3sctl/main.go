package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.aporeto.io/a3s/cmd/a3sctl/internal/authcmd"
	"go.aporeto.io/a3s/cmd/a3sctl/internal/compcmd"
	"go.aporeto.io/a3s/cmd/a3sctl/internal/help"
	"go.aporeto.io/a3s/internal/conf"
	"go.aporeto.io/a3s/pkgs/api"
	"go.aporeto.io/a3s/pkgs/bootstrap"
	"go.aporeto.io/manipulate/manipcli"
	"go.uber.org/zap"
)

var (
	cfgFile  string
	cfgName  string
	logLevel string
)

func main() {

	cobra.OnInitialize(initCobra)

	rootCmd := &cobra.Command{
		Use:              "a3sctl",
		Short:            "Controls a3s APIs",
		Long:             help.Load("a3sctl"),
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
				return err
			}
			return viper.BindPFlags(cmd.Flags())
		},
	}
	mflags := manipcli.ManipulatorFlagSet()
	mmaker := manipcli.ManipulatorMakerFromFlags()

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.config/a3sctl/default.yaml)")
	rootCmd.PersistentFlags().StringVar(&cfgName, "config-name", "", "default config name (default: default)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "Log level. Can be debug, info, warn or error")
	rootCmd.PersistentFlags().Bool("refresh-cached-token", false, "If set, the cached token will be refreshed")
	rootCmd.PersistentFlags().String("auto-auth-method", "", "If set, override config's file autoauth.enable")

	apiCmd := manipcli.New(api.Manager(), mmaker)
	apiCmd.PersistentFlags().AddFlagSet(mflags)
	apiCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
			return err
		}
		if err := authcmd.HandleAutoAuth(
			mmaker,
			viper.GetString("auto-auth-method"),
			viper.GetBool("refresh-cached-token"),
		); err != nil {
			return fmt.Errorf("auto auth error: %w", err)
		}
		return nil
	}

	authCmd := authcmd.New(mmaker)
	authCmd.PersistentFlags().AddFlagSet(mflags)

	compCmd := compcmd.New()

	rootCmd.AddCommand(
		apiCmd,
		authCmd,
		compCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
	}
}

func initCobra() {

	viper.SetEnvPrefix("a3sctl")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	bootstrap.ConfigureLogger("a3sctl", conf.LoggingConf{
		LogLevel:  logLevel,
		LogFormat: "console",
	})

	home, err := homedir.Dir()
	if err != nil {
		zap.L().Fatal("unable to find home dir", zap.Error(err))
	}

	hpath := path.Join(home, ".config", "a3sctl")
	if _, err := os.Stat(hpath); os.IsNotExist(err) {
		if err := os.Mkdir(hpath, os.ModePerm); err != nil {
			fmt.Printf("error: failed to create %s: %s\n", hpath, err)
			return
		}
	}

	if cfgFile == "" {
		cfgFile = os.Getenv("A3SCTL_CONFIG")
	}

	if cfgFile != "" {

		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			zap.L().Fatal("config file does not exist", zap.Error(err))
		}

		viper.SetConfigType("yaml")
		viper.SetConfigFile(cfgFile)
		zap.L().Debug("using config file", zap.String("path", cfgFile))
		err = viper.ReadInConfig()
		if err != nil {
			zap.L().Fatal("unable to read config", zap.Error(err))
		}

		return
	}

	viper.AddConfigPath(hpath)
	viper.AddConfigPath("/usr/local/etc/a3sctl")
	viper.AddConfigPath("/etc/a3sctl")

	if cfgName == "" {
		cfgName = os.Getenv("A3SCTL_CONFIG_NAME")
	}

	if cfgName != "" {
		zap.L().Debug("using config name", zap.String("name", cfgName))
		viper.SetConfigName(cfgName)
	} else {
		zap.L().Debug("using default config name")
		viper.SetConfigName("default")
	}

	err = viper.ReadInConfig()
	if err != nil {
		zap.L().Fatal("unable to read config", zap.Error(err))
	}

}
