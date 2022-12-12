package viettel_cloud

import (
	"context"
	"fmt"
	infrav1 "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/api/v1"
	cloudapi "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	_ "github.com/deepmap/oapi-codegen/pkg/types"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"sigs.k8s.io/cluster-api-provider-openstack/pkg/metrics"
)

var (
	provider ViettelCloudProvider
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

func (r *ViettelCloud) ListSubnet(ctx context.Context) (*[]cloudapi.Subnet, error) {
	mc := metrics.NewMetricPrometheusContext("Subnet", "list")
	subnets, err := r.Client.InfraSubnetsListWithResponse(ctx, &cloudapi.InfraSubnetsListParams{
		ProjectId: r.projectID,
	})
	if mc.ObserveRequest(err) != nil {
		return subnets.JSON200.Results, fmt.Errorf("can't list Subnet in Project : %s", r.projectID)
	}
	return subnets.JSON200.Results, fmt.Errorf("can't list Subnet in Project : %s", r.projectID)
}

func (r *ViettelCloud) GetSubnet(SubnetID openapi_types.UUID, SubnetList *[]cloudapi.Subnet) (cloudapi.Subnet, error) {
	Subnets := *SubnetList
	for i := range Subnets {
		subnet := Subnets[i]
		if *subnet.Id == SubnetID {
			return subnet, nil
		}
	}
	return cloudapi.Subnet{}, fmt.Errorf("can't found any VPC with ID : %s", SubnetID)
}

func (r *ViettelCloud) ReconcileVpc(ctx context.Context, ViettelCluster *infrav1.ViettelCluster, vcs infrav1.ViettelClusterSpec, ProjectID openapi_types.UUID) error {

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

func (r *ViettelCloud) ReconcileSubnet(log logr.Logger, ctx context.Context, ViettelCluster *infrav1.ViettelCluster, vcs infrav1.ViettelClusterSpec) error {

	//TODO check how many IP remain

	//TODO check network exist or not
	if ViettelCluster.Status.Vpc == nil || ViettelCluster.Status.Vpc.Id.String() == "" {
		log.Info("No need to reconcile network components since no network exists.")
		return nil
	}

	subnet, err := r.GetSubnet(vcs.VpcID, subnets)
	if err != nil {
		return fmt.Errorf("can't found any VPC with the ID %s", vcs.VpcID)
	}

	// Update ViettelCluster Subnet Spec
	ViettelCluster.Status.Subnet = &cloudapi.Subnet{
		Cidr:    subnet.Cidr,
		Id:      subnet.Id,
		Name:    subnet.Name,
		Owner:   subnet.Owner,
		Project: subnet.Project,
		Region:  subnet.Region,
		Vpc:     subnet.Vpc,
	}
	return nil
}
