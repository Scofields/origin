package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	kcmdconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type LogoutOptions struct {
	StartingKubeConfig *kclientcmdapi.Config
	Config             *kclient.Config
	Out                io.Writer

	PathOptions *kcmdconfig.PathOptions
}

const logoutLongDescription = `Logs out the current user by deleting the token and removing the token from the kubeconfig file.

Examples:

	# Logout:
	$ %[1]s

If you want to log back into the OpenShift server, try '%[2]s'.
`

// NewCmdLogout implements the OpenShift cli logout command
func NewCmdLogout(name, fullName, oscLoginFullCommand string, f *osclientcmd.Factory, reader io.Reader, out io.Writer) *cobra.Command {
	options := &LogoutOptions{
		Out: out,
	}

	cmds := &cobra.Command{
		Use:   name,
		Short: "Logs out the current user.",
		Long:  fmt.Sprintf(logoutLongDescription, fullName, oscLoginFullCommand),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.RunLogout(); err != nil {
				kcmdutil.CheckErr(err)
			}

		},
	}

	return cmds
}

func (o *LogoutOptions) Complete(f *osclientcmd.Factory, cmd *cobra.Command, args []string) error {
	kubeconfig, err := f.OpenShiftClientConfig.RawConfig()
	o.StartingKubeConfig = &kubeconfig
	if err != nil {
		return err
	}

	o.Config, err = f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	o.PathOptions = config.NewPathOptions(cmd)

	return nil
}

func (o LogoutOptions) Validate(args []string) error {
	if len(args) > 0 {
		return errors.New("No arguments are allowed")
	}

	if o.StartingKubeConfig == nil {
		return errors.New("Must have a config file already created")
	}

	if len(o.Config.BearerToken) == 0 {
		return errors.New("You must have a token in order to logout.")
	}

	return nil
}

func (o LogoutOptions) RunLogout() error {
	token := o.Config.BearerToken

	client, err := client.New(o.Config)
	if err != nil {
		return err
	}

	userInfo, err := whoAmI(client)
	if err != nil {
		return err
	}

	if err := client.OAuthAccessTokens().Delete(token); err != nil {
		return err
	}

	newConfig := *o.StartingKubeConfig

	for key, value := range newConfig.AuthInfos {
		if value.Token == token {
			value.Token = ""
			newConfig.AuthInfos[key] = value
			// don't break, its possible that more than one user stanza has the same token.
		}
	}

	if err := kcmdconfig.ModifyConfig(o.PathOptions, newConfig); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "User, %v, logged out of %v\n", userInfo.Name, o.Config.Host)

	return nil
}
