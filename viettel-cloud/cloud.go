package viettel_cloud

import (
	"context"
	cmp "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	scp "github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"os"
)

type ViettelCloudProvider struct {
	Cloud          *ViettelCloud
	CloudProjectID string
}

type IViettelCloud interface {
	GetServersByID(instanceID string) (*cmp.ServerDetail, error)
}

type ViettelCloud struct {
	username  string
	password  string
	projectID string
	apiUrl    string
	Client    cmp.ClientWithResponses
	context   context.Context
}

func NewViettelCloud(username string, password string, projectUUID string, apiUrl string) (*ViettelCloud, error) {
	basicAuthProvider, err := scp.NewSecurityProviderBasicAuth(username, password)
	if err != nil {
		return nil, err
	}
	client, clientErr := cmp.NewClientWithResponses(apiUrl, cmp.WithRequestEditorFn(basicAuthProvider.Intercept))
	if clientErr != nil {
		return nil, clientErr
	}
	return &ViettelCloud{username: username, password: password, projectID: projectUUID, apiUrl: apiUrl, Client: *client, context: context.TODO()}, nil
}

// CreateViettelCloudProvider creates Viettel Cloud client
func CreateViettelCloudProvider(ProjectID string) (ViettelCloudProvider, error) {
	VCInstance, err := NewViettelCloud(
		os.Getenv("VIETTEL_CLOUD_USER_NAME"),
		os.Getenv("VIETTEL_CLOUD_PASS_WORD"),
		ProjectID,
		os.Getenv("VIETTEL_CLOUD_AUTH_URL"))
	if err != nil {
		return ViettelCloudProvider{}, err
	}
	return ViettelCloudProvider{Cloud: VCInstance, CloudProjectID: ProjectID}, nil
}
