package viettel_cloud

import (
	"fmt"
	cmp "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	"github.com/google/uuid"
)

// GetServersByID returns server with specified instanceID
func (r *ViettelCloud) GetServersByID(serverID string) (*cmp.ServerDetail, error) {
	instanceUUID, err := uuid.Parse(serverID)
	serverRetriveResponse, err := r.Client.InfraServersRetrieveWithResponse(r.context, instanceUUID, &cmp.InfraServersRetrieveParams{ProjectId: r.projectID})
	if err != nil {
		return nil, fmt.Errorf("error retrive server %s: %v", serverID, err)
	}
	return serverRetriveResponse.JSON200, nil
}
