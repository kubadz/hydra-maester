package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"

	hydrav1alpha1 "github.com/ory/hydra-maester/api/v1alpha1"
	"github.com/ory/hydra-maester/controllers"
	"github.com/ory/hydra-maester/hydra"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = ctrl.Log.WithName("cron")

func main() {
	var (
		hydraURL, endpoint, forwardedProto string
		hydraPort                          int
	)

	ctrl.SetLogger(zap.Logger(true))

	flag.StringVar(&hydraURL, "hydra-url", "", "The address of ORY Hydra")
	flag.IntVar(&hydraPort, "hydra-port", 4445, "Port ORY Hydra is listening on")
	flag.StringVar(&endpoint, "endpoint", "/clients", "ORY Hydra's client endpoint")
	flag.StringVar(&forwardedProto, "forwarded-proto", "", "If set, this adds the value as the X-Forwarded-Proto header in requests to the ORY Hydra admin server")

	if hydraURL == "" {
		log.Error(fmt.Errorf("hydra URL can't be empty"), "Failed to load configuration")
		os.Exit(1)
	}

	scheme := runtime.NewScheme()
	hydrav1alpha1.AddToScheme(scheme)
	config := ctrl.GetConfigOrDie()

	client, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "Failed to create a client for Oauth2Client resources")
		os.Exit(1)
	}

	hydraClient, err := getHydraClient(hydrav1alpha1.OAuth2ClientSpec{
		HydraAdmin: hydrav1alpha1.HydraAdmin{
			URL:            hydraURL,
			Port:           hydraPort,
			Endpoint:       endpoint,
			ForwardedProto: forwardedProto,
		},
	})

	synchronizeClientsWithHydra(hydraClient, client)
}

func synchronizeClientsWithHydra(hydraClient controllers.HydraClientInterface, k8sClient client.Client) {
	ctx := context.Background()
	var oauth2clients hydrav1alpha1.OAuth2ClientList
	err := k8sClient.List(ctx, &oauth2clients)
	if err != nil {
		log.Error(err, "Failed to list Oauth2Client resources")
		os.Exit(1)
	}

	oauth2clientsJSON, err := json.Marshal(oauth2clients)
	if err != nil {
		log.Error(err, "Failed to marshal Oauth2Client resources into JSON")
		os.Exit(1)
	}
	fmt.Println(string(oauth2clientsJSON))
}

func getHydraClient(spec hydrav1alpha1.OAuth2ClientSpec) (controllers.HydraClientInterface, error) {

	address := fmt.Sprintf("%s:%d", spec.HydraAdmin.URL, spec.HydraAdmin.Port)
	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("unable to parse ORY Hydra's URL: %w", err)
	}

	client := &hydra.Client{
		HydraURL:   *u.ResolveReference(&url.URL{Path: spec.HydraAdmin.Endpoint}),
		HTTPClient: &http.Client{},
	}

	if spec.HydraAdmin.ForwardedProto != "" && spec.HydraAdmin.ForwardedProto != "off" {
		client.ForwardedProto = spec.HydraAdmin.ForwardedProto
	}

	return client, nil
}
