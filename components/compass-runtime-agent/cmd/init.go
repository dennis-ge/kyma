package main

import (
	"time"

	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/k8sconsts"
	appsecrets "github.com/kyma-project/kyma/components/compass-runtime-agent/internal/kyma/secrets"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/kyma/secrets/strategy"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/secrets"

	appclient "github.com/kyma-project/kyma/components/application-operator/pkg/client/clientset/versioned"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/apperrors"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/kyma"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/kyma/apiresources/rafter"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/kyma/apiresources/rafter/upload"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/kyma/applications"
	"github.com/kyma-project/kyma/components/compass-runtime-agent/internal/metrics"
	"github.com/kyma-project/rafter/pkg/apis/rafter/v1beta1"
	"github.com/pkg/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	clientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

type k8sResourceClientSets struct {
	core        *kubernetes.Clientset
	application *appclient.Clientset
	dynamic     dynamic.Interface
}

func k8sResourceClients(k8sConfig *restclient.Config) (*k8sResourceClientSets, error) {
	coreClientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create k8s core client")
	}

	applicationClientset, err := appclient.NewForConfig(k8sConfig)
	if err != nil {
		return nil, apperrors.Internal("Failed to create k8s application client, %s", err)
	}

	dynamicClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		return nil, apperrors.Internal("Failed to create dynamic client, %s", err)
	}

	return &k8sResourceClientSets{
		core:        coreClientset,
		application: applicationClientset,
		dynamic:     dynamicClient,
	}, nil
}

func createKymaService(k8sResourceClients *k8sResourceClientSets, uploadServiceUrl string, integrationNamespace string, centralGatewayServiceUrl string) (kyma.Service, error) {
	nameResolver := k8sconsts.NewNameResolver()
	secretsManagerConstructor := func(namespace string) secrets.Manager {
		return k8sResourceClients.core.CoreV1().Secrets(namespace)
	}
	repository := appsecrets.NewRepository(secretsManagerConstructor(integrationNamespace))

	applicationManager := newApplicationManager(k8sResourceClients.application)
	converter := applications.NewConverter(nameResolver, centralGatewayServiceUrl)
	rafterService := newRafter(k8sResourceClients.dynamic, uploadServiceUrl)
	credentialsService := appsecrets.NewCredentialsService(repository, strategy.NewSecretsStrategyFactory(), nameResolver)
	requestParametersService := appsecrets.NewRequestParametersService(repository, nameResolver)

	return kyma.NewService(applicationManager, converter, rafterService, credentialsService, requestParametersService), nil
}

func newRafter(dynamicClient dynamic.Interface, uploadServiceURL string) rafter.Service {
	groupVersionResource := v1beta1.GroupVersion.WithResource("clusterassetgroups")
	resourceInterface := dynamicClient.Resource(groupVersionResource)

	clusterAssetGroupRepository := rafter.NewAssetGroupRepository(resourceInterface)
	uploadClient := upload.NewClient(uploadServiceURL)
	return rafter.NewService(clusterAssetGroupRepository, uploadClient)
}

func newApplicationManager(appClientset *appclient.Clientset) applications.Repository {
	appInterface := appClientset.ApplicationconnectorV1alpha1().Applications()
	return applications.NewRepository(appInterface)
}

func newMetricsLogger(loggingTimeInterval time.Duration) (metrics.Logger, error) {
	config, err := restclient.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cluster config")
	}

	resourcesClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create resources clientset for config")
	}

	metricsClientset, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create metrics clientset for config")
	}

	return metrics.NewMetricsLogger(resourcesClientset, metricsClientset, loggingTimeInterval), nil
}
