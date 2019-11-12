package main

import (
	"fmt"
	"os"
	"testing"

	sdk "github.com/gaia-pipeline/gosdk"
)

func TestMain(m *testing.M) {
	hostDNSName = "localhost"
	err := GetSecretsFromVault(sdk.Arguments{
		sdk.Argument{
			Type:  sdk.VaultInp,
			Key:   "vault-token",
			Value: "root-token",
		},
		sdk.Argument{
			Type:  sdk.VaultInp,
			Key:   "vault-address",
			Value: "http://localhost:8200",
		}})
	if err != nil {
		fmt.Printf("Cannot retrieve data from vault: %s\n", err.Error())
		os.Exit(1)
	}

	err = PrepareDeployment(sdk.Arguments{
		sdk.Argument{
			Type:  sdk.TextFieldInp,
			Key:   "image-name",
			Value: "nginx:latest",
		},
		sdk.Argument{
			Type:  sdk.TextFieldInp,
			Key:   "app-name",
			Value: "vday-test-app",
		},
		sdk.Argument{
			Type:  sdk.TextFieldInp,
			Key:   "replicas",
			Value: "3",
		},
		sdk.Argument{
			Type:  sdk.TextFieldInp,
			Key:   "namespace",
			Value: "vday-2019",
		},
		sdk.Argument{
			Type:  sdk.TextFieldInp,
			Key:   "configmap",
			Value: "vday-configmap",
		},
	})
	if err != nil {
		fmt.Printf("Cannot prepare the deployment: %s\n", err.Error())
		os.Exit(1)
	}

	r := m.Run()
	os.Exit(r)
}

func TestCreateNamespace(t *testing.T) {
	err := CreateNamespace(sdk.Arguments{})
	if err != nil {
		t.Error(err)
	}
}

func TestCreateService(t *testing.T) {
	err := CreateService(sdk.Arguments{})
	if err != nil {
		t.Error(err)
	}
}

func TestCreateDeployment(t *testing.T) {
	err := CreateDeployment(sdk.Arguments{})
	if err != nil {
		t.Error(err)
	}
}

func TestCreateConfigMap(t *testing.T) {
	err := CreateConfigMap(sdk.Arguments{})
	if err != nil {
		t.Error(err)
	}
}
