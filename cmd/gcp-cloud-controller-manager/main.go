/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog"
	_ "k8s.io/kubernetes/pkg/features" // add the kubernetes feature gates
	"k8s.io/legacy-cloud-providers/gce"
)

var version string

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	logs.InitLogs()
	defer logs.FlushLogs()

	controllerList := []string{"cloud-node", "cloud-node-lifecycle", "service", "route"}

	s, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}
	s.KubeCloudShared.CloudProvider.Name = "gce"

	command := &cobra.Command{
		Use:  "gcp-cloud-controller-manager",
		Long: `gcp-cloud-controller-manager manages GCP cloud resources for a Kubernetes cluster.`,
		Run: func(cmd *cobra.Command, args []string) {

			// Use our version instead of the Kubernetes formatted version
			versionFlag := cmd.Flags().Lookup("version")
			if versionFlag.Value.String() == "true" {
				fmt.Printf("%s version: %s\n", cmd.Name(), version)
				os.Exit(0)
			}

			// Hard code gce cloud provider
			cloudProviderFlag := cmd.Flags().Lookup("cloud-provider")
			cloudProviderFlag.Value.Set(gce.ProviderName)

			cliflag.PrintFlags(cmd.Flags())

			c, err := s.Config(controllerList, app.ControllersDisabledByDefault.List(), nil, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			cloudconfig := c.Complete().ComponentConfig.KubeCloudShared.CloudProvider
			cloud, err := cloudprovider.InitCloudProvider(cloudconfig.Name, cloudconfig.CloudConfigFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			if cloud == nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			if !cloud.HasClusterID() {
				if c.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
					klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
				} else {
					klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
				}
			}
			// Initialize the cloud provider with a reference to the clientBuilder
			cloud.Initialize(c.ClientBuilder, make(chan struct{}))
			// Set the informer on the user cloud object
			if informerUserCloud, ok := cloud.(cloudprovider.InformerUser); ok {
				informerUserCloud.SetInformers(c.SharedInformers)
			}
			controllerInitializers := app.ConstructControllerInitializers(app.DefaultInitFuncConstructors, c.Complete(), cloud)

			if err := app.Run(c.Complete(), cloud, controllerInitializers, nil, wait.NeverStop); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	fs := command.Flags()
	namedFlagSets := s.Flags(controllerList, app.ControllersDisabledByDefault.List(), nil, nil)
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), command.Name())

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
