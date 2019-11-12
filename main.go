package main

import (
	"encoding/base64"
	"log"
	"os"
	"strconv"
	"strings"

	sdk "github.com/gaia-pipeline/gosdk"
	vaultapi "github.com/hashicorp/vault/api"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kubeConfVaultPath = "secret/data/kube-conf"
	kubeLocalPath     = "/tmp/kubeconfig"
)

var hostDNSName = "host.docker.internal"

// Variables dynamically set during runtime.
var (
	vaultAddress  string
	vaultToken    string
	imageName     string
	replicas      int32
	appName       string
	namespace     string
	configmapName string
	clientSet     *kubernetes.Clientset
)

// GetSecretsFromVault retrieves all information and credentials
// from vault and stores it in cache.
func GetSecretsFromVault(args sdk.Arguments) error {
	// Get vault credentials
	for _, arg := range args {
		switch arg.Key {
		case "vault-token":
			vaultToken = arg.Value
		case "vault-address":
			vaultAddress = arg.Value
		}
	}

	// Create new Vault client instance
	vaultClient, err := vaultapi.NewClient(vaultapi.DefaultConfig())
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}

	// Set vault address
	err = vaultClient.SetAddress(vaultAddress)
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}

	// Set token
	vaultClient.SetToken(vaultToken)

	// Read kube config from vault and decode base64
	l := vaultClient.Logical()
	s, err := l.Read(kubeConfVaultPath)
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	conf := s.Data["data"].(map[string]interface{})
	kubeConf, err := base64.StdEncoding.DecodeString(conf["conf"].(string))
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}

	// Convert config to string and replace localhost.
	// We use here the magical DNS name "host.docker.internal",
	// which resolves to the internal IP address used by the host.
	// If this should not work for you, replace it with your real IP address.
	confStr := string(kubeConf[:])
	confStr = strings.Replace(confStr, "localhost", hostDNSName, 1)
	kubeConf = []byte(confStr)

	// Create file
	f, err := os.Create(kubeLocalPath)
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	defer f.Close()

	// Write to file
	_, err = f.Write(kubeConf)

	log.Println("All data has been retrieved from vault!")
	return nil
}

// PrepareDeployment prepares the deployment by setting up
// the kubernetes client and caching all manual input from user.
func PrepareDeployment(args sdk.Arguments) error {
	// Setup kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", kubeLocalPath)
	if err != nil {
		return err
	}

	clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Cache given arguments for other jobs
	for _, arg := range args {
		switch arg.Key {
		case "vault-address":
			vaultAddress = arg.Value
		case "image-name":
			imageName = arg.Value
		case "replicas":
			rep, err := strconv.ParseInt(arg.Value, 10, 64)
			if err != nil {
				log.Printf("Error: %s\n", err)
				return err
			}
			replicas = int32(rep)
		case "app-name":
			appName = arg.Value
		case "namespace":
			namespace = arg.Value
		case "configmap":
			configmapName = arg.Value
		}
	}

	return nil
}

// CreateNamespace creates the namespace for our app.
// If the namespace already exists nothing will happen.
func CreateNamespace(args sdk.Arguments) error {
	// Create namespace object
	ns := &apiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"name":       "nginx",
				"env":        "production",
				"conference": "vday-2019",
			},
		},
	}

	// Lookup if namespace already exists
	nsClient := clientSet.CoreV1().Namespaces()
	_, err := nsClient.Get(namespace, metav1.GetOptions{})

	// namespace exists
	if err == nil {
		log.Printf("Namespace '%s' already exists. Update!", namespace)
		_, err = clientSet.CoreV1().Namespaces().Update(ns)
		if err != nil {
			return err
		}
		return nil
	}

	// Create namespace
	_, err = clientSet.CoreV1().Namespaces().Create(ns)
	if err != nil {
		return err
	}

	log.Printf("Service '%s' has been created!\n", namespace)
	return err
}

// CreateDeployment creates the kubernetes deployment.
// If it already exists, it will be updated.
func CreateDeployment(args sdk.Arguments) error {
	// Create deployment object
	d := &appsv1.Deployment{}
	d.ObjectMeta = metav1.ObjectMeta{
		Name: appName,
		Labels: map[string]string{
			"app": appName,
		},
	}
	d.Spec = appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": appName,
			},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: d.ObjectMeta,
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					apiv1.Container{
						Name:            appName,
						Image:           imageName,
						ImagePullPolicy: apiv1.PullAlways,
						Ports: []apiv1.ContainerPort{
							apiv1.ContainerPort{
								ContainerPort: int32(80),
							},
						},
					},
				},
			},
		},
	}

	// Lookup existing deployments
	deployClient := clientSet.AppsV1().Deployments(namespace)
	_, err := deployClient.Get(appName, metav1.GetOptions{})

	// Deployment already exists
	if err == nil {
		_, err = deployClient.Update(d)
		if err != nil {
			log.Printf("Error: %s\n", err.Error())
			return err
		}
		log.Printf("Deployment '%s' has been updated!\n", appName)
		return nil
	}

	// Create deployment object in kubernetes
	_, err = deployClient.Create(d)
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	log.Printf("Deployment '%s' has been created!\n", appName)
	return nil
}

// CreateService creates the service for our application.
// If the service already exists, it will be updated.
func CreateService(args sdk.Arguments) error {
	// Create service obj
	s := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				"app": appName,
			},
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Protocol:   apiv1.ProtocolTCP,
					Port:       int32(8090),
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}

	// Lookup for existing service
	serviceClient := clientSet.CoreV1().Services(namespace)
	currService, err := serviceClient.Get(appName, metav1.GetOptions{})

	// Service already exists
	if err == nil {
		s.ObjectMeta = currService.ObjectMeta
		s.Spec.ClusterIP = currService.Spec.ClusterIP
		_, err = serviceClient.Update(s)
		if err != nil {
			log.Printf("Error: %s\n", err.Error())
			return err
		}
		log.Printf("Service '%s' has been updated!\n", appName)
		return nil
	}

	// Create service
	_, err = serviceClient.Create(s)
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	log.Printf("Service '%s' has been created!\n", appName)
	return nil
}

func CreateConfigMap(args sdk.Arguments) error {
	_, err := clientSet.CoreV1().ConfigMaps(namespace).Get(configmapName, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		configMap := &apiv1.ConfigMap{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configmapName,
				Namespace: namespace,
			},
			Data:       map[string]string{"hello": "world"},
			BinaryData: nil,
		}

		_, err := clientSet.CoreV1().ConfigMaps(namespace).Create(configMap)
		if err != nil {
			log.Printf("Error: %s\n", err.Error())
			return err
		}
	} else if err != nil {

	}
	return nil
}

func main() {
	jobs := sdk.Jobs{
		sdk.Job{
			Handler:     GetSecretsFromVault,
			Title:       "Get secrets",
			Description: "Get secrets from vault",
			Args: sdk.Arguments{
				sdk.Argument{
					Type: sdk.VaultInp,
					Key:  "vault-token",
				},
				sdk.Argument{
					Type: sdk.VaultInp,
					Key:  "vault-address",
				},
			},
		},
		sdk.Job{
			Handler:     PrepareDeployment,
			Title:       "Prepare Deployment",
			Description: "Prepares the deployment (caches manual input and prepares kubernetes connection)",
			DependsOn:   []string{"Get secrets"},
			Args: sdk.Arguments{
				sdk.Argument{
					Type:        sdk.TextFieldInp,
					Description: "Application name:",
					Key:         "app-name",
					Value:       "vday-app",
				},
				sdk.Argument{
					Type:        sdk.TextFieldInp,
					Description: "Full image name including tag:",
					Key:         "image-name",
					Value:       "nginx:latest",
				},
				sdk.Argument{
					Type:        sdk.TextFieldInp,
					Description: "Number of replicas:",
					Key:         "replicas",
					Value:       "3",
				},
				sdk.Argument{
					Type:        sdk.TextFieldInp,
					Description: "Namespace name:",
					Key:         "namespace",
					Value:       "vday-2019",
				},
				sdk.Argument{
					Type:        sdk.TextFieldInp,
					Description: "Configmap name:",
					Key:         "configmapName",
					Value:       "vday-2019",
				},
			},
		},
		sdk.Job{
			Handler:     CreateNamespace,
			Title:       "Create namespace",
			Description: "Create kubernetes namespace",
			DependsOn:   []string{"Prepare Deployment"},
		},
		sdk.Job{
			Handler:     CreateConfigMap,
			Title:       "Create configmap",
			Description: "Create kubernetes configmap",
			DependsOn:   []string{"Create namespace"},
		},
		sdk.Job{
			Handler:     CreateDeployment,
			Title:       "Create deployment",
			Description: "Create kubernetes app deployment",
			DependsOn:   []string{"Create namespace", "Create configmap"},
		},
		sdk.Job{
			Handler:     CreateService,
			Title:       "Create Service",
			Description: "Create kubernetes service which exposes the service",
			DependsOn:   []string{"Create deployment"},
		},
	}

	// Serve
	if err := sdk.Serve(jobs); err != nil {
		panic(err)
	}
}
