package viettel_cloud

import (
	"context"
	"fmt"
	infrav1 "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/api/v1"
	cloudapi "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"strings"
	"time"
)

var backoff = wait.Backoff{
	Steps:    30,
	Duration: time.Second,
	Factor:   1.25,
	Jitter:   0.1,
}

func (r *ViettelCloud) GetOrCreateLB(log logr.Logger, ctx context.Context, LBName string, ProjectID openapi_types.UUID, vcluster *infrav1.ViettelCluster, vpc cloudapi.VPC, vcs infrav1.ViettelClusterSpec) (cloudapi.LoadBalancerDetail, error) {
	// get and check server group exist or not
	LbList, err := r.Client.InfraLoadBalancingLoadBalancerListWithResponse(ctx, &cloudapi.InfraLoadBalancingLoadBalancerListParams{ProjectId: ProjectID, Name: &LBName})
	if err != nil {
		return cloudapi.LoadBalancerDetail{}, fmt.Errorf("can't list LoadBalancer in project %s", ProjectID)
	}
	LBs := *LbList.JSON200.Results
	if len(LBs) > 1 {
		return cloudapi.LoadBalancerDetail{}, fmt.Errorf("error : found duplicate LoadBalancer with name %s in project %s", LBName, ProjectID)
	} else if len(LBs) < 1 {
		// Cus LB empty then create a new one
		regionID, _ := uuid.Parse(vcs.RegionID)
		var LBTopology cloudapi.TopologyEnum = ""
		var LBPackage cloudapi.PackageEnum = ""

		if strings.ToUpper(vcs.LoadBalancerTopology) == "" || strings.ToUpper(vcs.LoadBalancerPackage) == "" {
			HandleUpdateVCError(vcluster, fmt.Errorf("have to set both value of LoadBalancerTopology and LoadBalancerPackage"))
			return cloudapi.LoadBalancerDetail{}, fmt.Errorf("have to set both value of LoadBalancerTopology and LoadBalancerPackage")
		} else if strings.ToUpper(vcs.LoadBalancerTopology) == "" && strings.ToUpper(vcs.LoadBalancerPackage) == "" {
			LBTopology = cloudapi.SINGLE
			LBPackage = cloudapi.PackageEnumMEDIUM
		} else {
			if strings.ToUpper(vcs.LoadBalancerPackage) == string(cloudapi.PackageEnumLARGE) {
				LBPackage = cloudapi.PackageEnumLARGE
			} else if strings.ToUpper(vcs.LoadBalancerPackage) == string(cloudapi.PackageEnumSMALL) {
				LBPackage = cloudapi.PackageEnumSMALL
			} else {
				HandleUpdateVCError(vcluster, fmt.Errorf("invalid Value of LoadBalancer Package"))
				return cloudapi.LoadBalancerDetail{}, fmt.Errorf("invalid Value of LoadBalancer Package")
			}

			if strings.ToUpper(vcs.LoadBalancerTopology) == string(cloudapi.ACTIVESTANDBY) {
				LBTopology = cloudapi.ACTIVESTANDBY
			} else {
				HandleUpdateVCError(vcluster, fmt.Errorf("invalid Value of LoadBalancer Topology"))
				return cloudapi.LoadBalancerDetail{}, fmt.Errorf("invalid Value of LoadBalancer Topology")
			}
		}

		lb, err := r.Client.InfraLoadBalancingLoadBalancerCreateWithResponse(ctx, &cloudapi.InfraLoadBalancingLoadBalancerCreateParams{ProjectId: ProjectID},
			cloudapi.LoadBalancer{
				Name:     LBName,
				Package:  LBPackage,
				Region:   regionID,
				Topology: LBTopology,
				Vpc:      *vpc.Id,
			})
		if err != nil {
			return cloudapi.LoadBalancerDetail{}, fmt.Errorf("fail when create LB with ID %s : %s", lb.JSON201.Id, err)
		}
		getLb, err := r.Client.InfraLoadBalancingLoadBalancerRetrieveWithResponse(ctx, *lb.JSON201.Id, &cloudapi.InfraLoadBalancingLoadBalancerRetrieveParams{ProjectId: ProjectID})
		if err != nil {
			return cloudapi.LoadBalancerDetail{}, fmt.Errorf("can't get LoadBalancer with ID %s in project %s", *lb.JSON201.Id, ProjectID)
		}
		if err := r.waitForLoadBalancerActive(log, getLb.JSON200.Name, ctx, *getLb.JSON200.Id, vcluster, ProjectID); err != nil {
			return cloudapi.LoadBalancerDetail{}, fmt.Errorf("LoadBalancer %q with id %s is not active after timeout: %v", getLb.JSON200.Name, getLb.JSON200.Id.String(), err)
		}
		return *getLb.JSON200, nil
	} else {
		// found one LB exist so retrieve it
		LB, err := r.Client.InfraLoadBalancingLoadBalancerRetrieveWithResponse(ctx, *LBs[0].Id, &cloudapi.InfraLoadBalancingLoadBalancerRetrieveParams{ProjectId: ProjectID})
		if err != nil {
			return cloudapi.LoadBalancerDetail{}, fmt.Errorf("can't get LoadBalancer with ID %s in project %s", *LBs[0].Id, ProjectID)
		}
		if err := r.waitForLoadBalancerActive(log, LB.JSON200.Name, ctx, *LB.JSON200.Id, vcluster, ProjectID); err != nil {
			return cloudapi.LoadBalancerDetail{}, fmt.Errorf("LoadBalancer %q with id %s is not active after timeout: %v", LB.JSON200.Name, LB.JSON200.Id.String(), err)
		}
		return *LB.JSON200, nil
	}
}

func (r *ViettelCloud) GetOrCreateServerGroup(log logr.Logger, ctx context.Context, SGName string, ProjectID openapi_types.UUID, vcluster *infrav1.ViettelCluster, lb cloudapi.LoadBalancerDetail) (cloudapi.ServerGroupDetail, error) {

	SgList, err := r.Client.InfraLoadBalancingServerGroupListWithResponse(ctx, &cloudapi.InfraLoadBalancingServerGroupListParams{ProjectId: ProjectID, Name: &SGName})
	if err != nil {
		return cloudapi.ServerGroupDetail{}, fmt.Errorf("can't list LoadBalancer in project %s", ProjectID)
	}
	SGs := *SgList.JSON200.Results

	if len(SGs) > 1 {
		return cloudapi.ServerGroupDetail{}, fmt.Errorf("error : found duplicate LoadBalancer with name %s in project %s", SGName, ProjectID)
	} else if len(SGs) < 1 {
		// Cus SG empty then create a new one
		sg, err := r.Client.InfraLoadBalancingServerGroupCreateWithResponse(ctx, &cloudapi.InfraLoadBalancingServerGroupCreateParams{ProjectId: ProjectID},
			cloudapi.ServerGroup{
				Algorithm:             cloudapi.AlgorithmEnumROUNDROBIN,
				EnableHealthCheck:     true,
				LoadBalancer:          *lb.Id,
				MonitorDelay:          nil,
				MonitorExpectedCodes:  nil,
				MonitorHttpMethod:     nil,
				MonitorMaxRetries:     nil,
				MonitorMaxRetriesDown: nil,
				MonitorPath:           nil,
				MonitorTimeout:        nil,
				MonitorType:           nil,
				Name:                  SGName,
				Protocol:              cloudapi.LoadBalancingProtocolTypeEnumTCP,
			})
		if err != nil {
			return cloudapi.ServerGroupDetail{}, fmt.Errorf("fail when create LB with ID %s : %s", sg.JSON201.Id, err)
		}
		getSg, err := r.Client.InfraLoadBalancingServerGroupRetrieveWithResponse(ctx, *sg.JSON201.Id, &cloudapi.InfraLoadBalancingServerGroupRetrieveParams{ProjectId: ProjectID})
		if err != nil {
			return cloudapi.ServerGroupDetail{}, fmt.Errorf("can't get LoadBalancer with ID %s in project %s", *sg.JSON201.Id, ProjectID)
		}
		// check LB active or not to make sure Server Group are created
		if err := r.waitForLoadBalancerActive(log, lb.Name, ctx, *lb.Id, vcluster, ProjectID); err != nil {
			return cloudapi.ServerGroupDetail{}, fmt.Errorf("LoadBalancer %q with id %s is not active after timeout: %v", lb.Name, lb.Id.String(), err)
		}
		if err := r.waitForServerGroupActive(log, getSg.JSON200.Name, ctx, *getSg.JSON200.Id, vcluster, ProjectID); err != nil {
			return cloudapi.ServerGroupDetail{}, fmt.Errorf("ServerGroup %q with id %s is not active after timeout: %v", getSg.JSON200.Name, getSg.JSON200.Id.String(), err)
		}
		return *getSg.JSON200, nil
	} else {
		// found one SG exist so retrieve it
		SG, err := r.Client.InfraLoadBalancingServerGroupRetrieveWithResponse(ctx, *SGs[0].Id, &cloudapi.InfraLoadBalancingServerGroupRetrieveParams{ProjectId: ProjectID})
		if err != nil {
			return cloudapi.ServerGroupDetail{}, fmt.Errorf("can't get LoadBalancer with ID %s in project %s", *SGs[0].Id, ProjectID)
		}
		// check LB active or not to make sure Server Group are created
		if err := r.waitForLoadBalancerActive(log, lb.Name, ctx, *lb.Id, vcluster, ProjectID); err != nil {
			return cloudapi.ServerGroupDetail{}, fmt.Errorf("LoadBalancer %q with id %s is not active after timeout: %v", lb.Name, lb.Id.String(), err)
		}
		if err := r.waitForServerGroupActive(log, SG.JSON200.Name, ctx, *SG.JSON200.Id, vcluster, ProjectID); err != nil {
			return cloudapi.ServerGroupDetail{}, fmt.Errorf("ServerGroup %q with id %s is not active after timeout: %v", SG.JSON200.Name, SG.JSON200.Id.String(), err)
		}
		return *SG.JSON200, nil
	}
}

func (r *ViettelCloud) GetOrCreateListener(log logr.Logger, ctx context.Context, LSName string, ProjectID openapi_types.UUID, vcluster *infrav1.ViettelCluster, lb cloudapi.LoadBalancerDetail, sg cloudapi.ServerGroupDetail) (cloudapi.ListenerDetail, error) {
	// get and check Listener exist or not
	LsList, err := r.Client.InfraLoadBalancingListenerListWithResponse(ctx, &cloudapi.InfraLoadBalancingListenerListParams{ProjectId: ProjectID, Name: &LSName})
	if err != nil {
		return cloudapi.ListenerDetail{}, fmt.Errorf("can't list LoadBalancer in project %s", ProjectID)
	}
	Ls := *LsList.JSON200.Results

	if len(Ls) > 1 {
		return cloudapi.ListenerDetail{}, fmt.Errorf("error : found duplicate LoadBalancer with name %s in project %s", LSName, ProjectID)
	} else if len(Ls) < 1 {
		// Cus LS empty then create a new one
		ls, err := r.Client.InfraLoadBalancingListenerCreateWithResponse(ctx, &cloudapi.InfraLoadBalancingListenerCreateParams{ProjectId: ProjectID},
			cloudapi.Listener{
				AllowedCidrs:   &[]string{""},
				DefaultTlsCert: nil,
				LoadBalancer:   *lb.Id,
				Name:           LSName,
				Protocol:       cloudapi.LoadBalancingProtocolTypeEnumTCP,
				ProtocolPort:   0,
				ServerGroup:    sg.Id,
			})
		if err != nil {
			return cloudapi.ListenerDetail{}, fmt.Errorf("fail when create Listener with ID %s : %s", ls.JSON201.Id, err)
		}
		getLs, err := r.Client.InfraLoadBalancingListenerRetrieveWithResponse(ctx, *ls.JSON201.Id, &cloudapi.InfraLoadBalancingListenerRetrieveParams{ProjectId: ProjectID})
		if err != nil {
			return cloudapi.ListenerDetail{}, fmt.Errorf("can't get Listener with ID %s in project %s", *ls.JSON201.Id, ProjectID)
		}
		// check LB active or not to make sure Listener are created
		if err := r.waitForLoadBalancerActive(log, lb.Name, ctx, *lb.Id, vcluster, ProjectID); err != nil {
			return cloudapi.ListenerDetail{}, fmt.Errorf("LoadBalancer %q with id %s is not active after timeout: %v", lb.Name, lb.Id.String(), err)
		}
		if err := r.waitForListenerActive(log, getLs.JSON200.Name, ctx, *getLs.JSON200.Id, vcluster, ProjectID); err != nil {

			return cloudapi.ListenerDetail{}, fmt.Errorf("ServerGroup %q with id %s is not active after timeout: %v", getLs.JSON200.Name, getLs.JSON200.Id.String(), err)
		}
		return *getLs.JSON200, nil
	} else {
		// found one SG exist so retrieve it
		LS, err := r.Client.InfraLoadBalancingListenerRetrieveWithResponse(ctx, *Ls[0].Id, &cloudapi.InfraLoadBalancingListenerRetrieveParams{ProjectId: ProjectID})
		if err != nil {
			return cloudapi.ListenerDetail{}, fmt.Errorf("can't get LoadBalancer with ID %s in project %s", *Ls[0].Id, ProjectID)
		}
		// check LB active or not to make sure Server Group are created
		if err := r.waitForLoadBalancerActive(log, lb.Name, ctx, *lb.Id, vcluster, ProjectID); err != nil {
			HandleUpdateVCError(vcluster, errors.Errorf("LoadBalancer %q with id %s is not active", lb.Name, lb.Id.String()))
			return cloudapi.ListenerDetail{}, fmt.Errorf("LoadBalancer %q with id %s is not active after timeout: %v", lb.Name, lb.Id.String(), err)
		}
		if err := r.waitForListenerActive(log, LS.JSON200.Name, ctx, *LS.JSON200.Id, vcluster, ProjectID); err != nil {
			HandleUpdateVCError(vcluster, errors.Errorf("Listener %q with id %s is not active", LS.JSON200.Name, LS.JSON200.Id.String()))
			return cloudapi.ListenerDetail{}, fmt.Errorf("listener %q with id %s is not active after timeout: %v", LS.JSON200.Name, LS.JSON200.Id.String(), err)
		}
		return *LS.JSON200, nil
	}
}

func (r *ViettelCloud) ReconcileLB(log logr.Logger, ctx context.Context, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID, vpc cloudapi.VPC, vcs infrav1.ViettelClusterSpec) error {
	log.Info("Start Reconcile LoadBalancer ", "in ProjectID", ProjectID, "with VpcID", vpc.Id.String())

	//TODO hardening when reconcile LB, --> add tag Viettel Cloud for LB
	// Generate LoadBalancer Name
	LBName := "k8s-api-LB-" + vcluster.Name + vcluster.Namespace
	SGName := "k8s-api-SG-" + vcluster.Name + vcluster.Namespace
	LSName := "k8s-api-LS-" + vcluster.Name + vcluster.Namespace
	// Check LB exist or not
	LoadBalancer, err := r.GetOrCreateLB(log, ctx, LBName, ProjectID, vcluster, vpc, vcs)
	if err != nil {
		HandleUpdateVCError(vcluster, errors.Errorf("load balancer %q with id %s is not active or create LoadBalancer fail", LoadBalancer.Name, LoadBalancer.Id.String()))
		return fmt.Errorf("have some trouble when creating LoadBalancer : %s", err)
	}

	vcluster.Status.LoadBalancer = &LoadBalancer

	ServerGroup, err := r.GetOrCreateServerGroup(log, ctx, SGName, ProjectID, vcluster, LoadBalancer)
	if err != nil {
		HandleUpdateVCError(vcluster, errors.Errorf("Server Group %q with id %s is not active or create Server Group fail", ServerGroup.Name, ServerGroup.Id.String()))
		return fmt.Errorf("have some trouble when creating Server Group : %s", err)
	}

	vcluster.Status.ServerGroup = &ServerGroup

	Listener, err := r.GetOrCreateListener(log, ctx, LSName, ProjectID, vcluster, LoadBalancer, ServerGroup)
	if err != nil {
		HandleUpdateVCError(vcluster, errors.Errorf("Listener %q with id %s is not active or create Listener fail", Listener.Name, Listener.Id.String()))
		return fmt.Errorf("have some trouble when creating Listener : %s", err)
	}

	vcluster.Status.Listener = &Listener

	return nil
}

func (r *ViettelCloud) ReconcileDeleteLB(log logr.Logger, ctx context.Context, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID) error {
	log.Info("Start Reconcile delete LoadBalancer")

	// Generate LoadBalancer Name
	LBName := "k8s-api-LB-" + vcluster.Name + vcluster.Namespace
	getLb, err := r.Client.InfraLoadBalancingLoadBalancerListWithResponse(ctx, &cloudapi.InfraLoadBalancingLoadBalancerListParams{ProjectId: ProjectID, Name: &LBName})
	if err != nil {
		return fmt.Errorf("can't list LoadBalancer in project %s", ProjectID)
	}
	LBs := *getLb.JSON200.Results
	if len(LBs) > 1 {
		return fmt.Errorf("reconcile delete fail: found duplicate LoadBalancer with name %s in project %s", LBName, ProjectID)
	} else if len(LBs) < 1 {
		return fmt.Errorf("reconcile delete fail: can't found LoadBalancer with name %s in project %s", LBName, ProjectID)
	} else {
		_, err := r.Client.InfraLoadBalancingLoadBalancerDestroyWithResponse(ctx, *LBs[0].Id, &cloudapi.InfraLoadBalancingLoadBalancerDestroyParams{ProjectId: ProjectID})
		if err != nil {
			return fmt.Errorf("fail to delete Loadbalancer with ID %s : %s", LBs[0].Id, err)
		}
		return nil
	}
}

func (r *ViettelCloud) waitForLoadBalancerActive(log logr.Logger, LBName string, ctx context.Context, LBUUID openapi_types.UUID, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID) error {
	log.Info("Waiting for load balancer", LBName, "id", LBUUID.String(), "targetStatus", "ACTIVE")
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		res, err := r.Client.InfraLoadBalancingLoadBalancerRetrieveWithResponse(ctx, LBUUID,
			&cloudapi.InfraLoadBalancingLoadBalancerRetrieveParams{
				ProjectId: ProjectID,
			})
		if err != nil {
			return false, err
		}
		status := res.JSON200.Status
		provisioning := res.JSON200.ProvisioningStatus
		if status != nil && provisioning != nil {
			return false, err
		}
		sts, _ := status.AsLoadBalancingStatusEnum()
		stspro, _ := provisioning.AsLoadBalancingProvisioningStatusEnum()
		log.Info("waiting until... ", "LB Status : ", sts, "Provisioning Status : ", stspro)
		if sts == "ONLINE" && stspro == "ACTIVE" {
			return true, err
		}
		return false, err
	},
	)
	if err != nil {
		HandleUpdateVCError(vcluster, errors.Errorf("LoadBalancer %q with id %s is not active", LBName, LBUUID.String()))
		return err
	}
	return nil
}

func (r *ViettelCloud) waitForListenerActive(log logr.Logger, LSName string, ctx context.Context, LSUUID openapi_types.UUID, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID) error {
	log.Info("Waiting for Listener", LSName, "id", LSUUID.String(), "targetStatus", "ACTIVE")
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		res, err := r.Client.InfraLoadBalancingListenerRetrieveWithResponse(ctx, LSUUID,
			&cloudapi.InfraLoadBalancingListenerRetrieveParams{
				ProjectId: ProjectID,
			})
		if err != nil {
			return false, err
		}
		status := res.JSON200.Status
		provisioning := res.JSON200.ProvisioningStatus
		if status != nil && provisioning != nil {
			return false, err
		}
		sts, _ := status.AsLoadBalancingStatusEnum()
		stspro, _ := provisioning.AsLoadBalancingProvisioningStatusEnum()
		log.Info("waiting until... ", "LB Status : ", sts, "Provisioning Status : ", stspro)
		if sts == "ONLINE" && stspro == "ACTIVE" {
			return true, err
		}
		return false, err
	},
	)
	if err != nil {
		HandleUpdateVCError(vcluster, errors.Errorf("Server Group %q with id %s is not active", LSName, LSUUID.String()))
		return err
	}
	return nil
}

func (r *ViettelCloud) waitForServerGroupActive(log logr.Logger, SGName string, ctx context.Context, SGUUID openapi_types.UUID, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID) error {
	log.Info("Waiting for Server Group", SGName, "id", SGUUID.String(), "targetStatus", "ACTIVE")
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		res, err := r.Client.InfraLoadBalancingServerGroupRetrieveWithResponse(ctx, SGUUID,
			&cloudapi.InfraLoadBalancingServerGroupRetrieveParams{
				ProjectId: ProjectID,
			})
		if err != nil {
			return false, err
		}
		status := res.JSON200.Status
		provisioning := res.JSON200.ProvisioningStatus
		if status != nil && provisioning != nil {
			return false, err
		}
		sts, _ := status.AsLoadBalancingStatusEnum()
		stspro, _ := provisioning.AsLoadBalancingProvisioningStatusEnum()
		log.Info("waiting until... ", "LB Status : ", sts, "Provisioning Status : ", stspro)
		if stspro == "ACTIVE" {
			return true, err
		}

		return false, err
	},
	)
	if err != nil {
		HandleUpdateVCError(vcluster, errors.Errorf("Server Group %q with id %s is not active", SGName, SGUUID.String()))
		return err
	}
	return nil
}

func (r *ViettelCloud) waitForServerGroupMemberActive(log logr.Logger, SGMemberName string, ctx context.Context, SGMemberUUID openapi_types.UUID, vcluster *infrav1.ViettelCluster, ProjectID openapi_types.UUID) error {
	log.Info("Waiting for load balancer", SGMemberName, "id", SGMemberUUID.String(), "targetStatus", "ACTIVE")
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		res, err := r.Client.InfraLoadBalancingServerGroupMemberRetrieveWithResponse(ctx, SGMemberUUID,
			&cloudapi.InfraLoadBalancingServerGroupMemberRetrieveParams{
				ProjectId: ProjectID,
			})
		if err != nil {
			return false, err
		}
		status := res.JSON200.Status
		provisioning := res.JSON200.ProvisioningStatus
		if status != nil && provisioning != nil {
			return false, err
		}
		sts, _ := status.AsLoadBalancingStatusEnum()
		stspro, _ := provisioning.AsLoadBalancingProvisioningStatusEnum()
		log.Info("waiting until... ", "LB Status : ", sts, "Provisioning Status : ", stspro)
		if stspro == "ACTIVE" {
			return true, err
		}
		return false, err
	},
	)
	if err != nil {
		HandleUpdateVCError(vcluster, errors.Errorf("Server Group Member %q with id %s is not active", SGMemberName, SGMemberUUID.String()))
	}
	return nil
}
