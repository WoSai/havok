package apollo

import (
	"errors"
	"github.com/shima-park/agollo"
	"os"
)

var (
	//apollo ENV
	apolloURL       = "APOLLO_URL"
	apolloAppID     = "APOLLO_APP_ID"
	apolloNameSpace = "APOLLO_NAMESPACE"
	apolloCluster   = "APOLLO_CLUSTER"
	apolloKey       = "APOLLO_CONFIG_KEY"
	// apollo default value
	defaultNamespace = "application"
	defaultCluster   = "default"
)

func LoadConfigurationFromApollo() (string, error) {
	if os.Getenv(apolloURL) == "" || os.Getenv(apolloAppID) == "" || os.Getenv(apolloKey) == "" {
		return "", errors.New("please check apollo required parameter")
	}
	if os.Getenv(apolloNameSpace) != "" {
		defaultNamespace = os.Getenv(apolloNameSpace)
	}
	if os.Getenv(apolloCluster) != "" {
		defaultCluster = os.Getenv(apolloCluster)
	}
	err := agollo.Init(os.Getenv(apolloURL), os.Getenv(apolloAppID),
		agollo.DefaultNamespace(defaultNamespace),
		agollo.Cluster(defaultCluster),
		agollo.AutoFetchOnCacheMiss(),
	)
	if err != nil {
		return "", err
	}
	return agollo.Get(os.Getenv(apolloKey)), nil
}
