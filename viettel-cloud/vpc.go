package viettel_cloud

import (
	"context"
	"fmt"
	infrav1 "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/api/v1"
	cloudapi "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
)

func (r *ViettelCloud) GetVpc(ctx context.Context, VpcID string, ProjectID openapi_types.UUID) (cloudapi.VPC, error) {
	vpcID, _ := uuid.Parse(VpcID)
	vpc, err := r.Client.InfraVpcsRetrieveWithResponse(ctx, vpcID, &cloudapi.InfraVpcsRetrieveParams{ProjectId: ProjectID})
	if err != nil {
		return cloudapi.VPC{}, fmt.Errorf("can't found any VPC with ID : %s", VpcID)
	}
	return *vpc.JSON200, nil
}

func (r *ViettelCloud) CheckandEnableInternet(ctx context.Context, vpc cloudapi.VPC, ProjectID openapi_types.UUID) error {
	// check if VPC have internet access
	if *vpc.InternetAccess {
		return nil
	}
	routeList := *vpc.RouteTables
	for i := range routeList {
		_, err := r.Client.InfraRouteTablesEnableInternetAccessUpdate(ctx, routeList[i], &cloudapi.InfraRouteTablesEnableInternetAccessUpdateParams{ProjectId: ProjectID})
		if err != nil {
			return fmt.Errorf("have some trouble when enable internet access to VPC with ID : %s", vpc.Id)
		}
	}
	return nil
}

func (r *ViettelCloud) GetSubnet(ctx context.Context, SubnetID string, ProjectID openapi_types.UUID) (cloudapi.Subnet, error) {
	subnetID, _ := uuid.Parse(SubnetID)
	subnet, err := r.Client.InfraSubnetsRetrieveWithResponse(ctx, subnetID, &cloudapi.InfraSubnetsRetrieveParams{ProjectId: ProjectID})
	if err != nil {
		return cloudapi.Subnet{}, fmt.Errorf("can't found any Subnet with ID : %s", SubnetID)
	}
	return *subnet.JSON200, nil
}

func (r *ViettelCloud) ReconcileVpc(log logr.Logger, ctx context.Context, ViettelCluster *infrav1.ViettelCluster, vcs infrav1.ViettelClusterSpec, ProjectID openapi_types.UUID) error {
	log.Info("Start ReconcileVpc")
	// check VPC exist or not
	vpc, err := r.GetVpc(ctx, vcs.VpcID, ProjectID)
	if err != nil {
		return fmt.Errorf("can't found any VPC with the ID %s", vcs.VpcID)
	}
	err = r.CheckandEnableInternet(ctx, vpc, ProjectID)
	if err != nil {
		return fmt.Errorf("can't enable internet access to VPC with id  %s", vpc.Id)
	}
	// Update ViettelCluster VPC Spec
	ViettelCluster.Status.Vpc = &vpc
	return nil
}

func (r *ViettelCloud) ReconcileSubnet(log logr.Logger, ctx context.Context, ViettelCluster *infrav1.ViettelCluster, vcs infrav1.ViettelClusterSpec, ProjectID openapi_types.UUID) error {

	//TODO check how many IP remain

	//TODO check network exist or not
	if ViettelCluster.Status.Vpc == nil || ViettelCluster.Status.Vpc.Id.String() == "" {
		log.Info("No need to reconcile network components since no network exists.")
		return nil
	}

	subnet, err := r.GetSubnet(ctx, vcs.SubnetID, ProjectID)
	if err != nil {
		return fmt.Errorf("can't found any VPC with the ID %s", vcs.VpcID)
	}

	// Update ViettelCluster Subnet Spec
	ViettelCluster.Status.Subnet = &subnet
	return nil
}
