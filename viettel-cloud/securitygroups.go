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

const (
	secGroupPrefix     string = "k8s-api-sec-"
	controlPlaneSuffix string = "controlplane"
	workerSuffix       string = "worker"
	SecGroupExist      string = "exist"
	SecGroupNonExist   string = "non-exist"
	SecGroupDuplicate  string = "duplicate"
)

func (r *ViettelCloud) ReconcileSecurityGroups(log logr.Logger, ctx context.Context, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID, clusterName string, vcs infrav1.ViettelClusterSpec) error {
	log.Info("Reconciling security groups", "cluster", clusterName)
	if !vcluster.Spec.ManagedSecurityGroups {
		log.V(4).Info("No need to reconcile security groups", "cluster", clusterName)
		return nil
	}
	secControlPlaneGroupName := secGroupPrefix + vcluster.Name + vcluster.Namespace + controlPlaneSuffix
	secWorkerGroupName := secGroupPrefix + vcluster.Name + vcluster.Namespace + workerSuffix
	// get or create SecControlPlane
	secControlPlane, err := r.getOrCreateSecurityGroup(log, ctx, vcluster, secControlPlaneGroupName, ProjectID, vcs)
	if err != nil {
		return err
	}
	// get or create SecWorker
	secWorker, err := r.getOrCreateSecurityGroup(log, ctx, vcluster, secWorkerGroupName, ProjectID, vcs)
	if err != nil {
		return err
	}
	// get or create SecControlPlane Rule
	if err := r.CreateSecurityGroupRule(log, ctx, ProjectID, secControlPlane, secWorker); err != nil {
		return err
	}
	return nil
}

func (r *ViettelCloud) DeleteSecurityGroups(log logr.Logger, ctx context.Context, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID, clusterName string) error {
	log.Info("Reconciling delete security groups...", "cluster", clusterName)

	// set name for Security Group
	secControlPlaneGroupName := secGroupPrefix + vcluster.Name + vcluster.Namespace + controlPlaneSuffix
	secWorkerGroupName := secGroupPrefix + vcluster.Name + vcluster.Namespace + workerSuffix

	secGroupNames := map[string]string{
		controlPlaneSuffix: secControlPlaneGroupName,
		workerSuffix:       secWorkerGroupName,
	}

	for _, secName := range secGroupNames {
		sec, status, err := r.getSecurityGroupByName(ctx, secName, ProjectID)
		if err != nil {
			return err
		}
		if status != SecGroupNonExist {
			_, err := r.Client.InfraSecurityGroupsDestroyWithResponse(ctx, *sec.Id, &cloudapi.InfraSecurityGroupsDestroyParams{ProjectId: ProjectID})
			if err != nil {
				HandleUpdateVCError(vcluster, fmt.Errorf("failedDeleteSecurityGroup : Failed to delete security group %s", sec.Name))
				return fmt.Errorf("failedDeleteSecurityGroup : Failed to delete security group %s", sec.Name)
			}
			log.Info("SuccessfulDeleteSecurityGroup", "Deleted security group", sec.Name, "with id", sec.Id.String())
			return nil
		} else {
			HandleUpdateVCError(vcluster, fmt.Errorf("failedDeleteSecurityGroup : Failed to delete security group %s", sec.Name))
			return fmt.Errorf("failedDeleteSecurityGroup : Failed to delete security group %s", sec.Name)
		}
	}
	return nil
}

func (r *ViettelCloud) getOrCreateSecurityGroup(log logr.Logger, ctx context.Context, vcluster *infrav1.ViettelCluster, secName string, ProjectID openapi_types.UUID, vcs infrav1.ViettelClusterSpec) (cloudapi.SecurityGroup, error) {
	secGroup, status, err := r.getSecurityGroupByName(ctx, secName, ProjectID)
	if status == SecGroupDuplicate {
		HandleUpdateVCError(vcluster, fmt.Errorf("more than one security group found named: %s - Error : %s", secName, err))
		return cloudapi.SecurityGroup{}, err
	} else if status == SecGroupExist {
		// if Security Group exist then update it
		log.Info(fmt.Sprintf("Reuse Existing SecurityGroup %s with %s", secName, secGroup.Id.String()))
		// Update Status
		vcluster.Status.SecurityGroup = &secGroup
		return secGroup, nil
	} else if status == SecGroupNonExist {
		// if Security Group non exist then create it
		log.Info("Group doesn't exist, creating it.", "name", secName)
		log.Info("Creating group", "name", secName)
		sec, err := r.CreateSecGroup(ctx, secName, ProjectID, vcs)
		if err != nil {
			HandleUpdateVCError(vcluster, fmt.Errorf("FailedCreateSecurityGroup : Failed to create security group %s: %v", sec.Name, err))
			return cloudapi.SecurityGroup{}, err
		}
		log.Info("SuccessfulCreateSecurityGroup", "Created security group", sec.Name, "with id", sec.Id.String())
		return sec, nil
	}
	return cloudapi.SecurityGroup{}, fmt.Errorf("can't get or create Security Group in ProjectID %s", ProjectID)
}

func (r *ViettelCloud) CreateSecurityGroupRule(log logr.Logger, ctx context.Context, ProjectID openapi_types.UUID, SecCtrl cloudapi.SecurityGroup, SecWrk cloudapi.SecurityGroup) error {
	if SecCtrl.Id.String() != "" || SecWrk.Id.String() != "" {
		// with securityGroup controlplane will apply these rules
		Ipv4 := &cloudapi.SecurityGroupRule_Ethertype{}
		tcpType := &cloudapi.SecurityGroupRule_Protocol{}
		udpType := &cloudapi.SecurityGroupRule_Protocol{}

		_ = Ipv4.FromSGREtherTypeEnum(cloudapi.Ipv4)
		_ = tcpType.FromSecurityGroupRuleProtocolEnum(cloudapi.Tcp)
		_ = udpType.FromSecurityGroupRuleProtocolEnum(cloudapi.Udp)
		// Rule 1
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"Kubernetes API"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{6443}[0],
			PortRangeMin:  &[]int{6443}[0],
			Project:       &ProjectID,
			Protocol:      tcpType,
			SecurityGroup: *SecCtrl.Id,
		}); err != nil {
			return err
		}
		//Rule 2
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"Node Port Services TCP"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{30000}[0],
			PortRangeMin:  &[]int{32767}[0],
			Project:       &ProjectID,
			SecurityGroup: *SecCtrl.Id,
		}); err != nil {
			return err
		}
		// Rule 3
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"Node Port Services UDP"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{30000}[0],
			PortRangeMin:  &[]int{32767}[0],
			Project:       &ProjectID,
			SecurityGroup: *SecCtrl.Id,
		}); err != nil {
			return err
		}
		// Rule 4 - Ingress Internal
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"In-cluster Ingress"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{0}[0],
			PortRangeMin:  &[]int{0}[0],
			Project:       &ProjectID,
			RemoteGroup:   SecCtrl.Id, // self
			SecurityGroup: *SecCtrl.Id,
		}); err != nil {
			return err
		}
		// Rule 5
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"In-cluster Ingress"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{0}[0],
			PortRangeMin:  &[]int{0}[0],
			Project:       &ProjectID,
			RemoteGroup:   SecWrk.Id, // SecWorker ID
			SecurityGroup: *SecCtrl.Id,
		}); err != nil {
			return err
		}

		// with securityGroup worker will apply these rules
		//Rule 1
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"Node Port Services TCP"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{30000}[0],
			PortRangeMin:  &[]int{32767}[0],
			Project:       &ProjectID,
			SecurityGroup: *SecWrk.Id,
		}); err != nil {
			return err
		}
		// Rule 2
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"Node Port Services UDP"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{30000}[0],
			PortRangeMin:  &[]int{32767}[0],
			Project:       &ProjectID,
			SecurityGroup: *SecWrk.Id,
		}); err != nil {
			return err
		}
		// Rule 3 - Ingress Internal
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"In-cluster Ingress"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{0}[0],
			PortRangeMin:  &[]int{0}[0],
			Project:       &ProjectID,
			RemoteGroup:   SecWrk.Id, // self
			SecurityGroup: *SecWrk.Id,
		}); err != nil {
			return err
		}
		// Rule 4
		if _, err := r.Client.InfraSecurityGroupRulesCreate(ctx, &cloudapi.InfraSecurityGroupRulesCreateParams{ProjectId: ProjectID}, cloudapi.InfraSecurityGroupRulesCreateJSONRequestBody{
			Description:   &[]string{"In-cluster Ingress"}[0],
			Direction:     cloudapi.Ingress,
			Ethertype:     Ipv4,
			PortRangeMax:  &[]int{0}[0],
			PortRangeMin:  &[]int{0}[0],
			Project:       &ProjectID,
			RemoteGroup:   SecCtrl.Id, // SecControlPlane ID
			SecurityGroup: *SecWrk.Id,
		}); err != nil {
			return err
		}
		log.Info("create successful SecurityGroupRule for SecurityGroup Controlplane")
		return nil
	} else {
		return fmt.Errorf("error when creating SecurityGroup Rule because SecurityGroup not exist")
	}
}

func (r *ViettelCloud) getSecurityGroupByName(ctx context.Context, secName string, ProjectID openapi_types.UUID) (cloudapi.SecurityGroup, string, error) {
	// check if SecurityGroup exist or not
	secList, err := r.Client.InfraSecurityGroupsListWithResponse(ctx, &cloudapi.InfraSecurityGroupsListParams{ProjectId: ProjectID, Name: &secName})
	if err != nil {
		return cloudapi.SecurityGroup{}, "", fmt.Errorf("error when list Security Group in ProjectID %s", ProjectID.String())
	}
	Secs := *secList.JSON200.Results
	if len(Secs) > 1 {
		status := SecGroupDuplicate
		return cloudapi.SecurityGroup{}, status, fmt.Errorf("more than one security group found named: %s", secName)
	} else if len(Secs) < 1 {
		status := SecGroupNonExist
		return cloudapi.SecurityGroup{}, status, nil
	} else {
		status := SecGroupExist
		return Secs[0], status, nil
	}
}

func (r *ViettelCloud) CreateSecGroup(ctx context.Context, secName string, ProjectID openapi_types.UUID, vcs infrav1.ViettelClusterSpec) (cloudapi.SecurityGroup, error) {
	regionID, _ := uuid.Parse(vcs.RegionID)
	sec, err := r.Client.InfraSecurityGroupsCreateWithResponse(ctx, &cloudapi.InfraSecurityGroupsCreateParams{ProjectId: ProjectID}, cloudapi.SecurityGroup{
		Name:    secName,
		Project: &ProjectID,
		Region:  regionID,
	})
	if err != nil {
		return cloudapi.SecurityGroup{}, fmt.Errorf("error when create securityGroup named: %s - Error : %s", secName, err)
	}
	return *sec.JSON201, nil
}
