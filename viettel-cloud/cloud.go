package viettel_cloud

import (
	"context"
	cmp "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	scp "github.com/deepmap/oapi-codegen/pkg/securityprovider"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"k8s.io/klog"
	"os"
)

type Cloud struct {
	Client         cmp.ClientWithResponses
	CloudProjectID string
}

type IViettelCloud interface {
	GetInstanceByID(instanceID string) (*cmp.ServerDetail, error)
}

type ViettelCloud struct {
	username  string
	password  string
	projectID openapi_types.UUID
	apiUrl    string
	Client    cmp.ClientWithResponses
	context   context.Context
}

func NewViettelCloud(username string, password string, projectID string, apiUrl string) (*ViettelCloud, error) {
	projectUUID, err := uuid.Parse(projectID)
	if err != nil {
		klog.Errorf("project is not in UUID format")
		return nil, err
	}
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

// CreateViettelCloudProvider creates Viettel Cloud Instance
func CreateViettelCloudProvider() (IViettelCloud, error) {
	VCInstance, err := NewViettelCloud(
		os.Getenv("VIETTEL_CLOUD_USER_NAME"),
		os.Getenv("VIETTEL_CLOUD_PASS_WORD"),
		os.Getenv("VIETTEL_CLOUD_PROJECT_ID"),
		os.Getenv("VIETTEL_CLOUD_AUTH_URL"))
	if err != nil {
		return &ViettelCloud{}, err
	}
	return VCInstance, nil
}
