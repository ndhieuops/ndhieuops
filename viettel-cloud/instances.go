package viettel_cloud

import (
	"fmt"
	cmp "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	"github.com/google/uuid"
)

// GetInstanceByID returns server with specified instanceID
func (vc *ViettelCloud) GetInstanceByID(instanceID string) (*cmp.ServerDetail, error) {
	instanceUUID, err := uuid.Parse(instanceID)
	serverRetriveResponse, err := vc.Client.InfraServersRetrieveWithResponse(vc.context, instanceUUID, &cmp.InfraServersRetrieveParams{ProjectId: vc.projectID})
	if err != nil {
		return nil, fmt.Errorf("error retrive server %s: %v", instanceID, err)
	}
	return serverRetriveResponse.JSON200, nil
}
